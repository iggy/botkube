package status

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApplySnapshotDerivesOverallHealth(t *testing.T) {
	tests := []struct {
		name      string
		nodes     []Node
		workloads []Workload
		expected  Health
	}{
		{
			name:     "no nodes is unknown",
			expected: HealthUnknown,
		},
		{
			name:     "all healthy",
			nodes:    []Node{{Name: "a", Health: HealthOK}},
			expected: HealthOK,
		},
		{
			name:      "a degraded workload degrades the cluster",
			nodes:     []Node{{Name: "a", Health: HealthOK}},
			workloads: []Workload{{Name: "w", Health: HealthDegraded}},
			expected:  HealthDegraded,
		},
		{
			// The verdict takes the worst entry, so one failure is not masked by many healthy ones.
			name:      "a failing workload outweighs healthy nodes",
			nodes:     []Node{{Name: "a", Health: HealthOK}, {Name: "b", Health: HealthOK}},
			workloads: []Workload{{Name: "w", Health: HealthFailing}, {Name: "x", Health: HealthOK}},
			expected:  HealthFailing,
		},
		{
			name:     "a failing node outweighs a degraded one",
			nodes:    []Node{{Name: "a", Health: HealthDegraded}, {Name: "b", Health: HealthFailing}},
			expected: HealthFailing,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			state := NewClusterState("test", 10)

			state.ApplySnapshot(SnapshotData{Nodes: tc.nodes, Workloads: tc.workloads})

			assert.Equal(t, tc.expected, state.Snapshot().Summary.Health)
		})
	}
}

func TestApplySnapshotPreservesClusterName(t *testing.T) {
	// The collector does not know the configured cluster name, so the state must keep its own.
	state := NewClusterState("prod-eu", 10)

	state.ApplySnapshot(SnapshotData{Summary: Summary{KubernetesVer: "v1.34.2"}})

	snap := state.Snapshot()
	assert.Equal(t, "prod-eu", snap.Summary.ClusterName)
	assert.Equal(t, "v1.34.2", snap.Summary.KubernetesVer)
}

func TestRevisionChangesOnMutation(t *testing.T) {
	state := NewClusterState("test", 10)
	initial := state.Revision()

	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "a", Health: HealthOK}}})
	afterSnapshot := state.Revision()
	assert.Greater(t, afterSnapshot, initial, "a snapshot should bump the revision")

	state.AddWarning(Warning{Namespace: "default", Kind: "Pod", Name: "p", Reason: "Failed"})
	assert.Greater(t, state.Revision(), afterSnapshot, "a warning should bump the revision")
}

func TestMarkStaleDoesNotBumpRevision(t *testing.T) {
	// Marking stale means "re-collect this", not "the canvas changed". Bumping here would publish an
	// unchanged canvas on every event that carries no renderable data.
	state := NewClusterState("test", 10)
	before := state.Revision()

	state.MarkStale(SectionNodes, SectionSummary)

	assert.Equal(t, before, state.Revision())
	assert.Equal(t, []Section{SectionNodes, SectionSummary}, state.TakeStale())
}

func TestTakeStaleClearsAndDeduplicates(t *testing.T) {
	state := NewClusterState("test", 10)

	state.MarkStale(SectionWorkloads)
	state.MarkStale(SectionWorkloads, SectionCatalog)

	// Repeated marks collapse, so one re-collect covers a burst of events.
	assert.Equal(t, []Section{SectionCatalog, SectionWorkloads}, state.TakeStale())
	assert.Nil(t, state.TakeStale(), "stale sections should be cleared once taken")
}

func TestApplySnapshotClearsStale(t *testing.T) {
	// A full snapshot refreshes everything, so any pending re-collect request is already satisfied.
	state := NewClusterState("test", 10)
	state.MarkStale(SectionNodes)

	state.ApplySnapshot(SnapshotData{})

	assert.Nil(t, state.TakeStale())
}

func TestAddWarningUpdatesInPlace(t *testing.T) {
	// A hot-looping pod emits the same warning repeatedly. Appending each occurrence would crowd out
	// every other warning, so the same object/reason updates its existing entry.
	state := NewClusterState("test", 10)
	now := time.Now()

	state.AddWarning(Warning{Namespace: "shop", Kind: "Pod", Name: "api", Reason: "BackOff", Count: 1, Timestamp: now})
	state.AddWarning(Warning{Namespace: "shop", Kind: "Pod", Name: "api", Reason: "BackOff", Count: 7, Timestamp: now.Add(time.Second)})

	warnings := state.Snapshot().Warnings
	require.Len(t, warnings, 1)
	assert.Equal(t, int32(7), warnings[0].Count)
}

func TestAddWarningKeepsHighestCount(t *testing.T) {
	// Events can arrive out of order; the running count must not walk backwards.
	state := NewClusterState("test", 10)
	now := time.Now()

	state.AddWarning(Warning{Namespace: "shop", Kind: "Pod", Name: "api", Reason: "BackOff", Count: 9, Timestamp: now})
	state.AddWarning(Warning{Namespace: "shop", Kind: "Pod", Name: "api", Reason: "BackOff", Count: 2, Timestamp: now.Add(time.Second)})

	warnings := state.Snapshot().Warnings
	require.Len(t, warnings, 1)
	assert.Equal(t, int32(9), warnings[0].Count)
}

func TestAddWarningDistinguishesReasons(t *testing.T) {
	state := NewClusterState("test", 10)

	state.AddWarning(Warning{Namespace: "shop", Kind: "Pod", Name: "api", Reason: "BackOff"})
	state.AddWarning(Warning{Namespace: "shop", Kind: "Pod", Name: "api", Reason: "Unhealthy"})

	assert.Len(t, state.Snapshot().Warnings, 2)
}

func TestWarningsAreCappedNewestFirst(t *testing.T) {
	// The cap bounds memory on a noisy cluster; it must drop the oldest, not the newest.
	state := NewClusterState("test", 3)
	base := time.Date(2026, time.August, 1, 12, 0, 0, 0, time.UTC)

	for i := range 6 {
		state.AddWarning(Warning{
			Namespace: "default",
			Kind:      "Pod",
			Name:      string(rune('a' + i)),
			Reason:    "Failed",
			Timestamp: base.Add(time.Duration(i) * time.Minute),
		})
	}

	warnings := state.Snapshot().Warnings
	require.Len(t, warnings, 3)
	assert.Equal(t, "f", warnings[0].Name, "newest warning should be first")
	assert.Equal(t, "d", warnings[2].Name, "oldest retained warning")
}

func TestNewClusterStateFallsBackToDefaultCap(t *testing.T) {
	state := NewClusterState("test", 0)

	for i := range defaultMaxWarnings + 5 {
		state.AddWarning(Warning{Namespace: "default", Kind: "Pod", Name: string(rune(i)), Reason: "Failed"})
	}

	assert.Len(t, state.Snapshot().Warnings, defaultMaxWarnings)
}

func TestSnapshotIsACopy(t *testing.T) {
	// The publisher renders outside the lock, so a snapshot must not alias the live slices.
	state := NewClusterState("test", 10)
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	snap := state.Snapshot()
	snap.Nodes[0].Name = "mutated"

	assert.Equal(t, "node-1", state.Snapshot().Nodes[0].Name)
}

func TestConcurrentAccessIsSafe(t *testing.T) {
	// The event observer writes while the publisher reads, so this must be race-free under -race.
	state := NewClusterState("test", 20)

	var wg sync.WaitGroup
	for i := range 50 {
		wg.Add(3)
		go func() {
			defer wg.Done()
			state.AddWarning(Warning{Namespace: "default", Kind: "Pod", Name: string(rune(i)), Reason: "Failed"})
		}()
		go func() {
			defer wg.Done()
			state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "n", Health: HealthOK}}})
		}()
		go func() {
			defer wg.Done()
			_ = state.Snapshot()
			state.MarkStale(SectionNodes)
			_ = state.TakeStale()
		}()
	}
	wg.Wait()

	assert.NotZero(t, state.Revision())
}

func TestWorseOf(t *testing.T) {
	assert.Equal(t, HealthFailing, WorseOf(HealthOK, HealthFailing))
	assert.Equal(t, HealthFailing, WorseOf(HealthFailing, HealthOK))
	assert.Equal(t, HealthDegraded, WorseOf(HealthUnknown, HealthDegraded))
	assert.Equal(t, HealthOK, WorseOf(HealthOK, HealthOK))
	// Unknown is worse than OK: a health that could not be determined should not read as healthy.
	assert.Equal(t, HealthUnknown, WorseOf(HealthOK, HealthUnknown))
}
