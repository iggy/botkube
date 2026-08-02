package status

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/loggerx"
)

// sourceEvent mirrors the JSON shape the Kubernetes source plugin sends across the plugin boundary.
// The observer decodes structurally, so the test feeds it the same map a real plugin would.
func sourceEvent(kind, name, namespace, level string) map[string]any {
	return map[string]any{
		"Kind":      kind,
		"Name":      name,
		"Namespace": namespace,
		"Level":     level,
		"Reason":    "BackOff",
		"Messages":  []string{"Back-off restarting failed container"},
		"Count":     3,
		"TimeStamp": "2026-08-01T12:30:00Z",
	}
}

func TestObserverAppliesWarnings(t *testing.T) {
	// given
	state := NewClusterState("test", 10)
	observer := NewObserver(loggerx.NewNoop(), state, config.StatusCanvas{})

	// when
	observer.ObserveEvent("botkube/kubernetes", sourceEvent("Pod", "api-1", "shop", string(config.Error)))

	// then
	warnings := state.Snapshot().Warnings
	require.Len(t, warnings, 1)
	assert.Equal(t, "shop", warnings[0].Namespace)
	assert.Equal(t, "api-1", warnings[0].Name)
	assert.Equal(t, "BackOff", warnings[0].Reason)
	assert.Equal(t, "Back-off restarting failed container", warnings[0].Message)
	assert.Equal(t, int32(3), warnings[0].Count)
	assert.Equal(t, time.Date(2026, time.August, 1, 12, 30, 0, 0, time.UTC), warnings[0].Timestamp.UTC())
}

func TestObserverMarksStaleWithoutAddingWarning(t *testing.T) {
	// An info-level event carries no renderable status across the plugin boundary, so all it can do
	// is ask for a re-collect.
	state := NewClusterState("test", 10)
	observer := NewObserver(loggerx.NewNoop(), state, config.StatusCanvas{})

	observer.ObserveEvent("botkube/kubernetes", sourceEvent("Deployment", "web", "shop", string(config.Info)))

	assert.Empty(t, state.Snapshot().Warnings)
	assert.Equal(t, []Section{SectionCatalog, SectionSummary, SectionWorkloads}, state.TakeStale())
}

func TestObserverIgnoresOtherPlugins(t *testing.T) {
	// Only the Kubernetes source describes cluster state; other sources would corrupt the view.
	tests := []struct {
		name       string
		pluginName string
		handled    bool
	}{
		{name: "full plugin key", pluginName: "botkube/kubernetes", handled: true},
		{name: "versioned plugin key", pluginName: "botkube/kubernetes@v1.0.0", handled: true},
		// A custom plugin repository still ships the same plugin.
		{name: "custom repository", pluginName: "acme/kubernetes", handled: true},
		{name: "bare plugin name", pluginName: "kubernetes", handled: true},
		{name: "different plugin", pluginName: "botkube/prometheus", handled: false},
		{name: "bare other name", pluginName: "argocd", handled: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := NewClusterState("test", 10)
			observer := NewObserver(loggerx.NewNoop(), state, config.StatusCanvas{})

			observer.ObserveEvent(tc.pluginName, sourceEvent("Pod", "api-1", "shop", string(config.Error)))

			assert.Len(t, state.Snapshot().Warnings, map[bool]int{true: 1, false: 0}[tc.handled])
		})
	}
}

func TestObserverRespectsNamespaceScope(t *testing.T) {
	// given
	state := NewClusterState("test", 10)
	cfg := config.StatusCanvas{Namespaces: config.RegexConstraints{Include: []string{"shop"}}}
	observer := NewObserver(loggerx.NewNoop(), state, cfg)

	// when
	observer.ObserveEvent("botkube/kubernetes", sourceEvent("Pod", "api-1", "shop", string(config.Error)))
	observer.ObserveEvent("botkube/kubernetes", sourceEvent("Pod", "job-1", "kube-system", string(config.Error)))

	// then
	warnings := state.Snapshot().Warnings
	require.Len(t, warnings, 1)
	assert.Equal(t, "shop", warnings[0].Namespace)
}

func TestObserverAllowsClusterScopedObjects(t *testing.T) {
	// A Node has no namespace; filtering it out would hide node failures whenever a namespace scope
	// is configured.
	state := NewClusterState("test", 10)
	cfg := config.StatusCanvas{Namespaces: config.RegexConstraints{Include: []string{"shop"}}}
	observer := NewObserver(loggerx.NewNoop(), state, cfg)

	observer.ObserveEvent("botkube/kubernetes", sourceEvent("Node", "node-1", "", string(config.Error)))

	assert.Len(t, state.Snapshot().Warnings, 1)
}

func TestObserverSkipsWarningsWhenSectionDisabled(t *testing.T) {
	// given
	state := NewClusterState("test", 10)
	cfg := config.StatusCanvas{}
	cfg.Sections.Warnings.Disabled = true
	observer := NewObserver(loggerx.NewNoop(), state, cfg)

	// when
	observer.ObserveEvent("botkube/kubernetes", sourceEvent("Pod", "api-1", "shop", string(config.Error)))

	// then
	assert.Empty(t, state.Snapshot().Warnings)
	// The other sections still need refreshing; disabling warnings must not disable the whole hook.
	assert.NotEmpty(t, state.TakeStale())
}

func TestObserverIgnoresUnidentifiableEvents(t *testing.T) {
	// given
	state := NewClusterState("test", 10)
	observer := NewObserver(loggerx.NewNoop(), state, config.StatusCanvas{})
	before := state.Revision()

	// when
	observer.ObserveEvent("botkube/kubernetes", nil)
	observer.ObserveEvent("botkube/kubernetes", map[string]any{"Level": "error"})
	// A payload of the wrong shape must be dropped rather than panicking on the delivery hot path.
	observer.ObserveEvent("botkube/kubernetes", "not an event")

	// then
	assert.Equal(t, before, state.Revision())
	assert.Empty(t, state.Snapshot().Warnings)
	assert.Nil(t, state.TakeStale())
}

func TestIsWarning(t *testing.T) {
	tests := []struct {
		name     string
		event    pluginEvent
		expected bool
	}{
		{name: "warn level", event: pluginEvent{Level: string(config.Warn)}, expected: true},
		{name: "error level", event: pluginEvent{Level: string(config.Error)}, expected: true},
		{name: "critical level", event: pluginEvent{Level: string(config.Critical)}, expected: true},
		// Events read from the Kubernetes Event API carry the severity in Type, not Level.
		{name: "raw warning type", event: pluginEvent{Type: "warning"}, expected: true},
		{name: "mixed case type", event: pluginEvent{Type: "Warning"}, expected: true},
		{name: "info level", event: pluginEvent{Level: string(config.Info)}, expected: false},
		{name: "debug level", event: pluginEvent{Level: string(config.Debug)}, expected: false},
		{name: "nothing set", event: pluginEvent{}, expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, isWarning(tc.event))
		})
	}
}

func TestStaleSectionsFor(t *testing.T) {
	tests := []struct {
		kind     string
		expected []Section
	}{
		{kind: "Node", expected: []Section{SectionNodes, SectionSummary}},
		{kind: "Pod", expected: []Section{SectionWorkloads, SectionCatalog, SectionSummary}},
		{kind: "Deployment", expected: []Section{SectionWorkloads, SectionCatalog, SectionSummary}},
		{kind: "StatefulSet", expected: []Section{SectionWorkloads, SectionCatalog, SectionSummary}},
		{kind: "DaemonSet", expected: []Section{SectionWorkloads, SectionCatalog, SectionSummary}},
		// An unrecognised kind still moves cluster-wide counters, so the summary is refreshed.
		{kind: "ConfigMap", expected: []Section{SectionSummary}},
		{kind: "", expected: nil},
	}

	for _, tc := range tests {
		t.Run(tc.kind, func(t *testing.T) {
			assert.Equal(t, tc.expected, staleSectionsFor(tc.kind))
		})
	}
}

func TestParseEventTime(t *testing.T) {
	expected := time.Date(2026, time.August, 1, 12, 30, 0, 0, time.UTC)

	assert.Equal(t, expected, parseEventTime("2026-08-01T12:30:00Z").UTC())
	assert.Equal(t, expected, parseEventTime("2026-08-01T12:30:00.000000000Z").UTC())

	// An absent or unparseable timestamp still describes something that just happened, so it must not
	// fall back to the zero time and sink to the bottom of the warnings list.
	for _, in := range []string{"", "yesterday"} {
		assert.WithinDuration(t, time.Now(), parseEventTime(in), time.Minute)
	}
}
