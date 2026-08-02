package status

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/iggy/botkube/pkg/config"
)

// kubernetesPluginName is the plugin whose events describe cluster state.
const kubernetesPluginName = "kubernetes"

// Observer applies Kubernetes source plugin events to the cluster state, so the canvas reacts to
// changes without waiting for the next full snapshot.
//
// What an event can contribute is limited by the plugin boundary. Source events are serialized to
// JSON before reaching the agent, and the Kubernetes source event type excludes the underlying
// object from that JSON. Events therefore carry identity, reason and message, but no object status.
// The consequence is a split:
//
//   - Warnings need only what the event already carries, so they are applied directly.
//   - Nodes, workloads and the catalog need replica counts, conditions and images, so the event can
//     only flag those sections stale. The publisher then asks the collector for fresh data.
//
// The alternative, inferring status from an event's reason string, would put guessed data on the
// canvas; marking stale keeps the collector the single source of truth for anything status-shaped.
type Observer struct {
	log   logrus.FieldLogger
	state *ClusterState
	cfg   config.StatusCanvas
}

// NewObserver returns an Observer feeding the given cluster state.
func NewObserver(log logrus.FieldLogger, state *ClusterState, cfg config.StatusCanvas) *Observer {
	return &Observer{log: log, state: state, cfg: cfg}
}

// pluginEvent mirrors the subset of the Kubernetes source event that survives JSON serialization.
// It is decoded structurally rather than by importing the source plugin's event type, which would
// couple the agent to a plugin's internals.
type pluginEvent struct {
	Kind      string   `json:"Kind"`
	Name      string   `json:"Name"`
	Namespace string   `json:"Namespace"`
	Type      string   `json:"Type"`
	Reason    string   `json:"Reason"`
	Level     string   `json:"Level"`
	Messages  []string `json:"Messages"`
	Count     int32    `json:"Count"`
	TimeStamp string   `json:"TimeStamp"`
	Resource  string   `json:"Resource"`
}

// ObserveEvent applies a single source plugin event to the cluster state.
//
// It never blocks and never fails the caller: the dispatcher calls this on the hot path of event
// delivery, and a canvas update must not interfere with sending notifications.
func (o *Observer) ObserveEvent(pluginName string, rawObject any) {
	if !o.handles(pluginName) {
		return
	}

	event, ok := o.decode(rawObject)
	if !ok {
		return
	}

	// Warning-level events are fully described by the event itself.
	if !o.cfg.Sections.Warnings.Disabled && isWarning(event) {
		if allowed := o.namespaceAllowed(event.Namespace); allowed {
			o.state.AddWarning(Warning{
				Namespace: event.Namespace,
				Kind:      event.Kind,
				Name:      event.Name,
				Reason:    event.Reason,
				Message:   strings.Join(event.Messages, " "),
				Count:     event.Count,
				Timestamp: parseEventTime(event.TimeStamp),
			})
		}
	}

	// Anything else means some object's status may have changed. Flag the sections that render
	// object status so the publisher re-lists them.
	o.state.MarkStale(staleSectionsFor(event.Kind)...)
}

// handles reports whether events from the given plugin describe cluster state.
func (o *Observer) handles(pluginName string) bool {
	// pluginName is a full plugin key such as "botkube/kubernetes". Compare the plugin part so a
	// custom repository name still matches.
	_, name, _, err := config.DecomposePluginKey(pluginName)
	if err != nil {
		// Not a plugin key; fall back to a direct comparison rather than dropping the event.
		return pluginName == kubernetesPluginName
	}
	return name == kubernetesPluginName
}

// decode converts the dispatcher's raw event into the fields the canvas can use.
func (o *Observer) decode(rawObject any) (pluginEvent, bool) {
	// The dispatcher hands over whatever the plugin sent. Round-tripping through JSON handles both
	// the decoded map from a real plugin and a typed struct from an in-process caller.
	raw, err := json.Marshal(rawObject)
	if err != nil {
		o.log.WithError(err).Debug("Skipping status canvas event that could not be marshalled")
		return pluginEvent{}, false
	}

	var event pluginEvent
	if err := json.Unmarshal(raw, &event); err != nil {
		o.log.WithError(err).Debug("Skipping status canvas event that could not be decoded")
		return pluginEvent{}, false
	}

	// An event with no object identity cannot be attributed to anything renderable.
	if event.Kind == "" && event.Name == "" {
		return pluginEvent{}, false
	}

	return event, true
}

// namespaceAllowed reports whether the namespace is in the configured scope.
func (o *Observer) namespaceAllowed(namespace string) bool {
	// Cluster-scoped objects have no namespace and are always in scope.
	if namespace == "" {
		return true
	}

	constraints := o.cfg.Namespaces
	if !constraints.AreConstraintsDefined() {
		return true
	}

	allowed, err := constraints.IsAllowed(namespace)
	if err != nil {
		o.log.WithError(err).Debugf("While matching namespace %q for the status canvas", namespace)
		return false
	}
	return allowed
}

// isWarning reports whether the event should appear in the warnings section.
func isWarning(event pluginEvent) bool {
	// The source plugin maps its event types onto these levels; anything at warn or above is worth
	// surfacing. The raw Kubernetes event type is also checked, because events read from the Event
	// API carry "Warning" there rather than in Level.
	switch {
	case strings.EqualFold(event.Level, string(config.Warn)),
		strings.EqualFold(event.Level, string(config.Error)),
		strings.EqualFold(event.Level, string(config.Critical)),
		strings.EqualFold(event.Type, "warning"):
		return true
	default:
		return false
	}
}

// staleSectionsFor maps an object kind to the canvas sections whose rendering depends on it.
func staleSectionsFor(kind string) []Section {
	switch kind {
	case "Node":
		return []Section{SectionNodes, SectionSummary}
	case "Pod":
		// A pod change can move a workload's ready count and reveal a rollout mid-flight.
		return []Section{SectionWorkloads, SectionCatalog, SectionSummary}
	case "Deployment", "StatefulSet", "DaemonSet", "ReplicaSet":
		return []Section{SectionWorkloads, SectionCatalog, SectionSummary}
	case "":
		return nil
	default:
		// An unrecognised kind still changes cluster-wide counters.
		return []Section{SectionSummary}
	}
}

// parseEventTime parses the timestamp a source event carries, falling back to now.
func parseEventTime(in string) time.Time {
	if in == "" {
		return time.Now()
	}

	if ts, err := time.Parse(time.RFC3339Nano, in); err == nil {
		return ts
	}
	if ts, err := time.Parse(time.RFC3339, in); err == nil {
		return ts
	}

	// An unparseable timestamp still describes something that just happened, so treating it as now
	// keeps the warning near the top of the list instead of pinning it to the zero time.
	return time.Now()
}
