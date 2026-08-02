package status

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/sirupsen/logrus"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"

	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/multierror"
)

// Collector builds a full cluster snapshot by listing resources directly.
//
// Listing rather than watching is deliberate: the canvas renders a whole-cluster view that is
// rewritten in full on every update, so a point-in-time list is exactly the shape needed. It also
// makes the snapshot self-correcting, since it never depends on having seen every intermediate event.
type Collector struct {
	k8sCli kubernetes.Interface
	log    logrus.FieldLogger
	cfg    config.StatusCanvas

	// catalogSelector is the pre-validated label selector for the opt-in service catalog.
	catalogSelector string
}

// NewCollector returns a Collector for the given configuration.
func NewCollector(log logrus.FieldLogger, k8sCli kubernetes.Interface, cfg config.StatusCanvas) (*Collector, error) {
	selector := strings.TrimSpace(cfg.Sections.Catalog.LabelSelector)
	if selector != "" {
		// Fail at construction rather than on every collection pass, so a typo surfaces at startup.
		if _, err := metav1.ParseToLabelSelector(selector); err != nil {
			return nil, fmt.Errorf("while parsing status canvas catalog label selector %q: %w", selector, err)
		}
	}

	return &Collector{
		k8sCli:          k8sCli,
		log:             log,
		cfg:             cfg,
		catalogSelector: selector,
	}, nil
}

// Collect gathers a full cluster snapshot.
//
// A failure in one section does not abandon the whole snapshot: a canvas showing four of five
// sections plus a logged error is more useful than no update at all. Errors are aggregated and
// returned so the caller can log them, while the returned data holds whatever did succeed.
func (c *Collector) Collect(ctx context.Context) (SnapshotData, error) {
	var (
		out    SnapshotData
		errs   = multierror.New()
		podsNS = map[string][]corev1.Pod{}
	)

	if version, err := c.k8sCli.Discovery().ServerVersion(); err != nil {
		errs = multierror.Append(errs, fmt.Errorf("while getting server version: %w", err))
	} else {
		out.Summary.KubernetesVer = version.GitVersion
	}

	namespaces, err := c.namespacesInScope(ctx)
	if err != nil {
		// Without the namespace scope every namespaced lookup below would be unscoped, which could
		// leak resources the operator excluded on purpose. Bail out instead.
		return out, fmt.Errorf("while resolving namespaces in scope: %w", err)
	}
	out.Summary.NamespacesInScope = len(namespaces)

	nodes, err := c.collectNodes(ctx)
	if err != nil {
		errs = multierror.Append(errs, err)
	} else {
		out.Nodes = nodes
		out.Summary.NodesTotal = len(nodes)
		for _, n := range nodes {
			if n.Health == HealthOK {
				out.Summary.NodesReady++
			}
		}
	}

	for _, ns := range namespaces {
		pods, err := c.k8sCli.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing pods in namespace %q: %w", ns, err))
			continue
		}
		podsNS[ns] = pods.Items
		out.Summary.PodsTotal += len(pods.Items)
	}

	workloads, total, err := c.collectWorkloads(ctx, namespaces, podsNS)
	if err != nil {
		errs = multierror.Append(errs, err)
	}
	out.Workloads = workloads
	out.Summary.WorkloadsTotal = total
	for _, w := range workloads {
		if w.Kind == "Pod" {
			out.Summary.PodsUnhealthy++
		}
	}

	if !c.cfg.Sections.Catalog.Disabled {
		catalog, err := c.collectCatalog(ctx, namespaces, podsNS)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		out.Catalog = catalog
	}

	if !c.cfg.Sections.Warnings.Disabled {
		warnings, err := c.collectWarnings(ctx, namespaces)
		if err != nil {
			errs = multierror.Append(errs, err)
		}
		out.Warnings = warnings
	}

	return out, errs.ErrorOrNil()
}

// namespacesInScope lists namespaces allowed by the configured include/exclude constraints.
func (c *Collector) namespacesInScope(ctx context.Context) ([]string, error) {
	list, err := c.k8sCli.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("while listing namespaces: %w", err)
	}

	constraints := c.cfg.Namespaces
	// An unconfigured namespace list means the whole cluster. Without this, RegexConstraints with an
	// empty Include would reject everything and the canvas would silently render empty.
	allowAll := !constraints.AreConstraintsDefined()

	var out []string
	for _, ns := range list.Items {
		if allowAll {
			out = append(out, ns.Name)
			continue
		}

		allowed, err := constraints.IsAllowed(ns.Name)
		if err != nil {
			return nil, fmt.Errorf("while matching namespace %q: %w", ns.Name, err)
		}
		if allowed {
			out = append(out, ns.Name)
		}
	}

	sort.Strings(out)
	return out, nil
}

func (c *Collector) collectNodes(ctx context.Context) ([]Node, error) {
	if c.cfg.Sections.Nodes.Disabled && c.cfg.Sections.Summary.Disabled {
		return nil, nil
	}

	list, err := c.k8sCli.CoreV1().Nodes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("while listing nodes: %w", err)
	}

	out := make([]Node, 0, len(list.Items))
	for _, node := range list.Items {
		entry := Node{
			Name:        node.Name,
			Schedulable: !node.Spec.Unschedulable,
			KubeletVer:  node.Status.NodeInfo.KubeletVersion,
			Health:      HealthUnknown,
			Ready:       string(corev1.ConditionUnknown),
		}

		for _, cond := range node.Status.Conditions {
			if cond.Type != corev1.NodeReady {
				continue
			}

			entry.Ready = string(cond.Status)
			switch cond.Status {
			case corev1.ConditionTrue:
				entry.Health = HealthOK
			case corev1.ConditionFalse:
				entry.Health = HealthFailing
				entry.Reason = cond.Reason
			default:
				// A node reporting Unknown has stopped heartbeating; its workloads are at risk even
				// though the node has not been declared NotReady, so treat it as failing.
				entry.Health = HealthFailing
				entry.Reason = cond.Reason
			}
			break
		}

		// A cordoned node is not broken, but it cannot take new work, which is worth surfacing.
		if entry.Health == HealthOK && !entry.Schedulable {
			entry.Health = HealthDegraded
			entry.Reason = "unschedulable"
		}

		out = append(out, entry)
	}

	return out, nil
}

// collectWorkloads returns workloads and pods which are not in their desired state, along with the
// total number of workloads inspected. Healthy entries are intentionally dropped: the section exists
// to show what needs attention, and keeping them would bury the signal.
func (c *Collector) collectWorkloads(ctx context.Context, namespaces []string, podsNS map[string][]corev1.Pod) ([]Workload, int, error) {
	if c.cfg.Sections.Workloads.Disabled && c.cfg.Sections.Summary.Disabled {
		return nil, 0, nil
	}

	var (
		out   []Workload
		total int
		errs  = multierror.New()
	)

	for _, ns := range namespaces {
		deployments, err := c.k8sCli.AppsV1().Deployments(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing deployments in namespace %q: %w", ns, err))
		} else {
			total += len(deployments.Items)
			for _, d := range deployments.Items {
				desired := int32(1)
				if d.Spec.Replicas != nil {
					desired = *d.Spec.Replicas
				}
				if entry, ok := replicaWorkload("Deployment", ns, d.Name, d.Status.ReadyReplicas, desired); ok {
					out = append(out, entry)
				}
			}
		}

		statefulSets, err := c.k8sCli.AppsV1().StatefulSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing statefulsets in namespace %q: %w", ns, err))
		} else {
			total += len(statefulSets.Items)
			for _, s := range statefulSets.Items {
				desired := int32(1)
				if s.Spec.Replicas != nil {
					desired = *s.Spec.Replicas
				}
				if entry, ok := replicaWorkload("StatefulSet", ns, s.Name, s.Status.ReadyReplicas, desired); ok {
					out = append(out, entry)
				}
			}
		}

		daemonSets, err := c.k8sCli.AppsV1().DaemonSets(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing daemonsets in namespace %q: %w", ns, err))
		} else {
			total += len(daemonSets.Items)
			for _, d := range daemonSets.Items {
				// DaemonSets have no replica spec; the scheduler decides how many are wanted.
				desired := d.Status.DesiredNumberScheduled
				if entry, ok := replicaWorkload("DaemonSet", ns, d.Name, d.Status.NumberReady, desired); ok {
					out = append(out, entry)
				}
			}
		}

		for _, pod := range podsNS[ns] {
			if entry, ok := unhealthyPod(pod); ok {
				out = append(out, entry)
			}
		}
	}

	return out, total, errs.ErrorOrNil()
}

// replicaWorkload reports a replica-based workload when it is below its desired count.
func replicaWorkload(kind, namespace, name string, ready, desired int32) (Workload, bool) {
	if ready >= desired {
		return Workload{}, false
	}

	// Zero ready replicas means the workload serves nothing; a partial rollout still serves traffic.
	health := HealthDegraded
	if ready == 0 && desired > 0 {
		health = HealthFailing
	}

	return Workload{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Health:    health,
		Ready:     ready,
		Desired:   desired,
		Reason:    fmt.Sprintf("%d/%d ready", ready, desired),
	}, true
}

// unhealthyPod reports a pod which is not running normally.
func unhealthyPod(pod corev1.Pod) (Workload, bool) {
	// Succeeded pods are completed Jobs, not failures, so they are not reported.
	if pod.Status.Phase == corev1.PodSucceeded {
		return Workload{}, false
	}

	var (
		restarts   int32
		reason     string
		health     = HealthOK
		ready      int32
		containers = int32(len(pod.Status.ContainerStatuses))
	)

	for _, cs := range pod.Status.ContainerStatuses {
		if cs.RestartCount > restarts {
			restarts = cs.RestartCount
		}
		if cs.Ready {
			ready++
		}
		if cs.State.Waiting != nil && reason == "" {
			reason = cs.State.Waiting.Reason
			// CrashLoopBackOff and ImagePullBackOff mean the container cannot start at all.
			health = HealthFailing
		}
	}

	switch pod.Status.Phase {
	case corev1.PodFailed:
		health = HealthFailing
		if reason == "" {
			reason = pod.Status.Reason
		}
	case corev1.PodPending:
		if health == HealthOK {
			health = HealthDegraded
			if reason == "" {
				reason = "Pending"
			}
		}
	case corev1.PodRunning:
		// A Running pod with containers not yet ready is mid-startup or failing readiness.
		if health == HealthOK && containers > 0 && ready < containers {
			health = HealthDegraded
			if reason == "" {
				reason = "containers not ready"
			}
		}
	}

	if health == HealthOK {
		return Workload{}, false
	}
	if reason == "" {
		reason = string(pod.Status.Phase)
	}

	return Workload{
		Kind:      "Pod",
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Health:    health,
		Ready:     ready,
		Desired:   containers,
		Reason:    reason,
		Restarts:  restarts,
	}, true
}

// collectCatalog builds the opt-in service catalog. Only workloads matching the configured label
// selector are included, which keeps the section a curated catalog instead of a cluster dump.
func (c *Collector) collectCatalog(ctx context.Context, namespaces []string, podsNS map[string][]corev1.Pod) ([]CatalogEntry, error) {
	var (
		out  []CatalogEntry
		errs = multierror.New()
		opts = metav1.ListOptions{LabelSelector: c.catalogSelector}
	)

	for _, ns := range namespaces {
		deployments, err := c.k8sCli.AppsV1().Deployments(ns).List(ctx, opts)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing catalog deployments in namespace %q: %w", ns, err))
		} else {
			for _, d := range deployments.Items {
				desired := int32(1)
				if d.Spec.Replicas != nil {
					desired = *d.Spec.Replicas
				}
				out = append(out, c.catalogEntry("Deployment", ns, d.Name, d.Spec.Selector, d.Spec.Template, d.Status.ReadyReplicas, desired, podsNS[ns]))
			}
		}

		statefulSets, err := c.k8sCli.AppsV1().StatefulSets(ns).List(ctx, opts)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing catalog statefulsets in namespace %q: %w", ns, err))
		} else {
			for _, s := range statefulSets.Items {
				desired := int32(1)
				if s.Spec.Replicas != nil {
					desired = *s.Spec.Replicas
				}
				out = append(out, c.catalogEntry("StatefulSet", ns, s.Name, s.Spec.Selector, s.Spec.Template, s.Status.ReadyReplicas, desired, podsNS[ns]))
			}
		}

		daemonSets, err := c.k8sCli.AppsV1().DaemonSets(ns).List(ctx, opts)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing catalog daemonsets in namespace %q: %w", ns, err))
		} else {
			for _, d := range daemonSets.Items {
				out = append(out, c.catalogEntry("DaemonSet", ns, d.Name, d.Spec.Selector, d.Spec.Template, d.Status.NumberReady, d.Status.DesiredNumberScheduled, podsNS[ns]))
			}
		}
	}

	return out, errs.ErrorOrNil()
}

// catalogEntry describes one catalog workload: the image versions it declares plus whether the
// running pods have actually converged on them.
func (c *Collector) catalogEntry(kind, namespace, name string, selector *metav1.LabelSelector, template corev1.PodTemplateSpec, ready, desired int32, pods []corev1.Pod) CatalogEntry {
	entry := CatalogEntry{
		Kind:      kind,
		Namespace: namespace,
		Name:      name,
		Ready:     ready,
		Desired:   desired,
		Health:    HealthOK,
	}

	for _, container := range template.Spec.Containers {
		entry.Images = append(entry.Images, ContainerImage{
			Container: container.Name,
			Image:     container.Image,
			Version:   imageVersion(container.Image),
		})
	}

	// The spec says which image is wanted; the pods say which images are actually live. If the pods
	// disagree with the spec, or with each other, the rollout has not finished.
	live := liveImages(selector, template, pods)
	for _, container := range entry.Images {
		versions := live[container.Container]
		if len(versions) > 1 {
			entry.RollingOut = true
			break
		}
		if len(versions) == 1 && !versions[container.Image] {
			entry.RollingOut = true
			break
		}
	}

	if ready < desired {
		entry.Health = HealthDegraded
		if ready == 0 && desired > 0 {
			entry.Health = HealthFailing
		}
	}

	return entry
}

// liveImages maps container name to the set of images that container is running across the
// workload's pods.
func liveImages(selector *metav1.LabelSelector, template corev1.PodTemplateSpec, pods []corev1.Pod) map[string]map[string]bool {
	out := map[string]map[string]bool{}

	match, err := metav1.LabelSelectorAsSelector(selector)
	if err != nil || match == nil || match.Empty() {
		// Without a usable selector the workload's pods cannot be identified. Reporting no live
		// images simply leaves RollingOut false rather than guessing from unrelated pods.
		return out
	}

	for _, pod := range pods {
		if !match.Matches(labels.Set(pod.Labels)) {
			continue
		}

		for _, container := range pod.Spec.Containers {
			if out[container.Name] == nil {
				out[container.Name] = map[string]bool{}
			}
			out[container.Name][container.Image] = true
		}
	}

	// Containers absent from the template are sidecars injected by a mutating webhook; they would
	// otherwise show up as spurious rollout signal.
	declared := map[string]bool{}
	for _, container := range template.Spec.Containers {
		declared[container.Name] = true
	}
	for name := range out {
		if !declared[name] {
			delete(out, name)
		}
	}

	return out
}

func (c *Collector) collectWarnings(ctx context.Context, namespaces []string) ([]Warning, error) {
	var (
		out  []Warning
		errs = multierror.New()
		// Only Warning-type events are rendered, so filter server-side rather than fetching
		// every Normal event just to discard it.
		opts = metav1.ListOptions{FieldSelector: "type=Warning"}
	)

	for _, ns := range namespaces {
		list, err := c.k8sCli.CoreV1().Events(ns).List(ctx, opts)
		if err != nil {
			errs = multierror.Append(errs, fmt.Errorf("while listing warning events in namespace %q: %w", ns, err))
			continue
		}

		for _, event := range list.Items {
			out = append(out, Warning{
				Namespace: event.Namespace,
				Kind:      event.InvolvedObject.Kind,
				Name:      event.InvolvedObject.Name,
				Reason:    event.Reason,
				Message:   event.Message,
				Count:     event.Count,
				Timestamp: eventTimestamp(event),
			})
		}
	}

	return out, errs.ErrorOrNil()
}

// eventTimestamp picks the most recent timestamp an event carries. Which fields are populated
// depends on how the event was emitted, so all three are checked.
func eventTimestamp(event corev1.Event) time.Time {
	if !event.LastTimestamp.IsZero() {
		return event.LastTimestamp.Time
	}
	if !event.EventTime.IsZero() {
		return event.EventTime.Time
	}
	return event.CreationTimestamp.Time
}

// imageVersion extracts the human-readable version from an image reference.
func imageVersion(image string) string {
	// A digest-pinned reference has no meaningful tag; show a short digest instead.
	if idx := strings.LastIndex(image, "@"); idx != -1 {
		digest := image[idx+1:]
		if short := strings.TrimPrefix(digest, "sha256:"); short != digest && len(short) > 12 {
			return "sha256:" + short[:12]
		}
		return digest
	}

	// The last colon is only a tag separator if it comes after the last slash; otherwise it is the
	// port of a registry host such as registry.local:5000/app.
	idx := strings.LastIndex(image, ":")
	if idx == -1 || idx < strings.LastIndex(image, "/") {
		return "latest"
	}
	return image[idx+1:]
}
