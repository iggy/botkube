package status

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/loggerx"
	"github.com/iggy/botkube/pkg/ptr"
)

func namespace(name string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func readyNode(name string) *corev1.Node {
	return &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Status: corev1.NodeStatus{
			Conditions: []corev1.NodeCondition{{Type: corev1.NodeReady, Status: corev1.ConditionTrue}},
			NodeInfo:   corev1.NodeSystemInfo{KubeletVersion: "v1.34.2"},
		},
	}
}

func TestCollectorNamespaceScope(t *testing.T) {
	tests := []struct {
		name       string
		namespaces config.RegexConstraints
		expected   int
	}{
		{
			// An unconfigured namespace list must mean "everything". RegexConstraints with an empty
			// Include rejects every value, which would silently render an empty canvas.
			name:     "no constraints means all namespaces",
			expected: 3,
		},
		{
			name:       "include regex",
			namespaces: config.RegexConstraints{Include: []string{"app-.*"}},
			expected:   2,
		},
		{
			name:       "exclude wins over include",
			namespaces: config.RegexConstraints{Include: []string{".*"}, Exclude: []string{"kube-system"}},
			expected:   2,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// given
			k8sCli := fake.NewSimpleClientset(namespace("app-one"), namespace("app-two"), namespace("kube-system"))
			cfg := config.StatusCanvas{Namespaces: tc.namespaces}
			collector, err := NewCollector(loggerx.NewNoop(), k8sCli, cfg)
			require.NoError(t, err)

			// when
			out, err := collector.Collect(context.Background())

			// then
			require.NoError(t, err)
			assert.Equal(t, tc.expected, out.Summary.NamespacesInScope)
		})
	}
}

func TestCollectorNodeHealth(t *testing.T) {
	// given
	notReady := readyNode("node-bad")
	notReady.Status.Conditions = []corev1.NodeCondition{{
		Type: corev1.NodeReady, Status: corev1.ConditionFalse, Reason: "KubeletNotReady",
	}}

	cordoned := readyNode("node-cordoned")
	cordoned.Spec.Unschedulable = true

	unknown := readyNode("node-unknown")
	unknown.Status.Conditions = []corev1.NodeCondition{{
		Type: corev1.NodeReady, Status: corev1.ConditionUnknown, Reason: "NodeStatusUnknown",
	}}

	k8sCli := fake.NewSimpleClientset(readyNode("node-ok"), notReady, cordoned, unknown)
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, config.StatusCanvas{})
	require.NoError(t, err)

	// when
	out, err := collector.Collect(context.Background())

	// then
	require.NoError(t, err)
	byName := map[string]Node{}
	for _, n := range out.Nodes {
		byName[n.Name] = n
	}

	assert.Equal(t, HealthOK, byName["node-ok"].Health)
	assert.Equal(t, HealthFailing, byName["node-bad"].Health)
	// A cordoned node still serves traffic, so it is degraded rather than failing.
	assert.Equal(t, HealthDegraded, byName["node-cordoned"].Health)
	assert.Equal(t, "unschedulable", byName["node-cordoned"].Reason)
	// A node that stopped heartbeating puts its workloads at risk, so Unknown counts as failing.
	assert.Equal(t, HealthFailing, byName["node-unknown"].Health)

	assert.Equal(t, 4, out.Summary.NodesTotal)
	assert.Equal(t, 1, out.Summary.NodesReady)
}

func TestCollectorReportsOnlyUnhealthyWorkloads(t *testing.T) {
	// given
	healthy := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "healthy", Namespace: "app"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr.FromType[int32](2)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 2},
	}
	degraded := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "degraded", Namespace: "app"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr.FromType[int32](3)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	down := &appsv1.StatefulSet{
		ObjectMeta: metav1.ObjectMeta{Name: "down", Namespace: "app"},
		Spec:       appsv1.StatefulSetSpec{Replicas: ptr.FromType[int32](2)},
		Status:     appsv1.StatefulSetStatus{ReadyReplicas: 0},
	}

	k8sCli := fake.NewSimpleClientset(namespace("app"), healthy, degraded, down)
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, config.StatusCanvas{})
	require.NoError(t, err)

	// when
	out, err := collector.Collect(context.Background())

	// then
	require.NoError(t, err)

	byName := map[string]Workload{}
	for _, w := range out.Workloads {
		byName[w.Name] = w
	}

	assert.NotContains(t, byName, "healthy", "healthy workloads would bury the ones needing attention")
	assert.Equal(t, HealthDegraded, byName["degraded"].Health)
	// Zero ready replicas means the workload serves nothing, which is worse than a partial rollout.
	assert.Equal(t, HealthFailing, byName["down"].Health)
	assert.Equal(t, 3, out.Summary.WorkloadsTotal, "all workloads are counted, not just unhealthy ones")
}

func TestUnhealthyPod(t *testing.T) {
	tests := []struct {
		name           string
		pod            corev1.Pod
		expectReported bool
		expectedHealth Health
		expectedReason string
	}{
		{
			name: "running and ready is not reported",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Phase:             corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
				},
			},
		},
		{
			// A completed Job is not a failure; reporting it would fill the section with noise.
			name: "succeeded is not reported",
			pod:  corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodSucceeded}},
		},
		{
			name: "crash looping is failing",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Phase: corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{{
						RestartCount: 5,
						State:        corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
					}},
				},
			},
			expectReported: true,
			expectedHealth: HealthFailing,
			expectedReason: "CrashLoopBackOff",
		},
		{
			name:           "failed phase is failing",
			pod:            corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodFailed, Reason: "Evicted"}},
			expectReported: true,
			expectedHealth: HealthFailing,
			expectedReason: "Evicted",
		},
		{
			name:           "pending is degraded",
			pod:            corev1.Pod{Status: corev1.PodStatus{Phase: corev1.PodPending}},
			expectReported: true,
			expectedHealth: HealthDegraded,
			expectedReason: "Pending",
		},
		{
			name: "running but not ready is degraded",
			pod: corev1.Pod{
				Status: corev1.PodStatus{
					Phase:             corev1.PodRunning,
					ContainerStatuses: []corev1.ContainerStatus{{Ready: true}, {Ready: false}},
				},
			},
			expectReported: true,
			expectedHealth: HealthDegraded,
			expectedReason: "containers not ready",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			out, reported := unhealthyPod(tc.pod)

			require.Equal(t, tc.expectReported, reported)
			if !tc.expectReported {
				return
			}
			assert.Equal(t, tc.expectedHealth, out.Health)
			assert.Equal(t, tc.expectedReason, out.Reason)
		})
	}
}

func TestCollectorCatalogIsOptIn(t *testing.T) {
	// given
	labelled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "listed",
			Namespace: "app",
			Labels:    map[string]string{"botkube.io/canvas": "true"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.FromType[int32](1),
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "listed"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "listed"}},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{Name: "api", Image: "ghcr.io/acme/api:1.2.3"}},
				},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}
	unlabelled := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "hidden", Namespace: "app"},
		Spec:       appsv1.DeploymentSpec{Replicas: ptr.FromType[int32](1)},
		Status:     appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	k8sCli := fake.NewSimpleClientset(namespace("app"), labelled, unlabelled)
	cfg := config.StatusCanvas{}
	cfg.Sections.Catalog.LabelSelector = "botkube.io/canvas=true"
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, cfg)
	require.NoError(t, err)

	// when
	out, err := collector.Collect(context.Background())

	// then
	require.NoError(t, err)
	require.Len(t, out.Catalog, 1, "only labelled workloads belong in a curated catalog")
	assert.Equal(t, "listed", out.Catalog[0].Name)
	require.Len(t, out.Catalog[0].Images, 1)
	assert.Equal(t, "1.2.3", out.Catalog[0].Images[0].Version)
}

func TestCollectorDetectsRollout(t *testing.T) {
	// A rollout is in progress when the live pods disagree with each other about the image, which the
	// workload's own status cannot express.
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	podLabels := map[string]string{"app": "web"}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "app",
			Labels: map[string]string{"botkube.io/canvas": "true"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.FromType[int32](2),
			Selector: selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "acme/web:2.0.0"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 2},
	}

	oldPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-old", Namespace: "app", Labels: podLabels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "acme/web:1.0.0"}}},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
		},
	}
	newPod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-new", Namespace: "app", Labels: podLabels},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "acme/web:2.0.0"}}},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}},
		},
	}

	k8sCli := fake.NewSimpleClientset(namespace("app"), deployment, oldPod, newPod)
	cfg := config.StatusCanvas{}
	cfg.Sections.Catalog.LabelSelector = "botkube.io/canvas=true"
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, cfg)
	require.NoError(t, err)

	// when
	out, err := collector.Collect(context.Background())

	// then
	require.NoError(t, err)
	require.Len(t, out.Catalog, 1)
	assert.True(t, out.Catalog[0].RollingOut, "two live image versions mean the rollout has not settled")
}

func TestCollectorIgnoresInjectedSidecarsForRollout(t *testing.T) {
	// A mutating webhook can add containers absent from the template. Counting them would report a
	// permanent rollout that never settles.
	selector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	podLabels := map[string]string{"app": "web"}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name: "web", Namespace: "app",
			Labels: map[string]string{"botkube.io/canvas": "true"},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: ptr.FromType[int32](1),
			Selector: selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: podLabels},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "web", Image: "acme/web:2.0.0"}}},
			},
		},
		Status: appsv1.DeploymentStatus{ReadyReplicas: 1},
	}

	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "app", Labels: podLabels},
		Spec: corev1.PodSpec{Containers: []corev1.Container{
			{Name: "web", Image: "acme/web:2.0.0"},
			{Name: "istio-proxy", Image: "istio/proxyv2:1.20.0"},
		}},
		Status: corev1.PodStatus{
			Phase:             corev1.PodRunning,
			ContainerStatuses: []corev1.ContainerStatus{{Ready: true}, {Ready: true}},
		},
	}

	k8sCli := fake.NewSimpleClientset(namespace("app"), deployment, pod)
	cfg := config.StatusCanvas{}
	cfg.Sections.Catalog.LabelSelector = "botkube.io/canvas=true"
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, cfg)
	require.NoError(t, err)

	// when
	out, err := collector.Collect(context.Background())

	// then
	require.NoError(t, err)
	require.Len(t, out.Catalog, 1)
	assert.False(t, out.Catalog[0].RollingOut)
}

func TestNewCollectorRejectsInvalidLabelSelector(t *testing.T) {
	// Failing at construction surfaces a typo at startup instead of on every collection pass.
	cfg := config.StatusCanvas{}
	cfg.Sections.Catalog.LabelSelector = "!!! not a selector"

	_, err := NewCollector(loggerx.NewNoop(), fake.NewSimpleClientset(), cfg)

	require.Error(t, err)
	assert.Contains(t, err.Error(), "catalog label selector")
}

func TestImageVersion(t *testing.T) {
	tests := []struct {
		name     string
		image    string
		expected string
	}{
		{name: "simple tag", image: "nginx:1.25", expected: "1.25"},
		{name: "registry with tag", image: "ghcr.io/acme/api:1.4.2", expected: "1.4.2"},
		{name: "no tag defaults to latest", image: "nginx", expected: "latest"},
		{name: "registry path without tag", image: "ghcr.io/acme/api", expected: "latest"},
		{
			// The colon here is a registry port, not a tag separator.
			name:     "registry port without tag",
			image:    "registry.local:5000/acme/api",
			expected: "latest",
		},
		{name: "registry port with tag", image: "registry.local:5000/acme/api:3.1", expected: "3.1"},
		{
			name:     "digest is truncated",
			image:    "acme/api@sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef",
			expected: "sha256:0123456789ab",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, imageVersion(tc.image))
		})
	}
}
