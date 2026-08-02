package status

import (
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"

	"github.com/iggy/botkube/pkg/config"
)

// This test is based on golden files. To update them, run:
// go test ./internal/status/... -test.update-golden

// fixedTime keeps the rendered output deterministic; a wall clock would make the golden files churn.
var fixedTime = time.Date(2026, time.August, 1, 12, 30, 0, 0, time.UTC)

func fullSnapshot() Snapshot {
	return Snapshot{
		Summary: Summary{
			ClusterName:       "prod-eu",
			KubernetesVer:     "v1.34.2",
			NodesTotal:        3,
			NodesReady:        2,
			NamespacesInScope: 4,
			PodsTotal:         42,
			PodsUnhealthy:     2,
			WorkloadsTotal:    17,
			Health:            HealthFailing,
			SnapshotAt:        fixedTime,
		},
		Nodes: []Node{
			{Name: "node-1", Health: HealthOK, Ready: "True", Schedulable: true, KubeletVer: "v1.34.2"},
			{Name: "node-2", Health: HealthFailing, Ready: "False", Schedulable: true, KubeletVer: "v1.34.2", Reason: "KubeletNotReady"},
			{Name: "node-3", Health: HealthDegraded, Ready: "True", Schedulable: false, KubeletVer: "v1.34.1", Reason: "unschedulable"},
		},
		Workloads: []Workload{
			{Kind: "Pod", Namespace: "shop", Name: "api-7c9d-x2k", Health: HealthFailing, Ready: 0, Desired: 1, Reason: "CrashLoopBackOff", Restarts: 12},
			{Kind: "Deployment", Namespace: "shop", Name: "web", Health: HealthDegraded, Ready: 2, Desired: 3, Reason: "2/3 ready"},
		},
		Catalog: []CatalogEntry{
			{
				Kind: "Deployment", Namespace: "shop", Name: "api",
				Images: []ContainerImage{
					{Container: "api", Image: "ghcr.io/acme/api:1.4.2", Version: "1.4.2"},
					{Container: "sidecar", Image: "ghcr.io/acme/proxy:2.0.0", Version: "2.0.0"},
				},
				Ready: 3, Desired: 3, Health: HealthOK,
			},
			{
				Kind: "Deployment", Namespace: "shop", Name: "web",
				Images: []ContainerImage{{Container: "web", Image: "ghcr.io/acme/web:2.1.0", Version: "2.1.0"}},
				Ready:  2, Desired: 3,
				RollingOut: true, Health: HealthDegraded,
			},
		},
		Warnings: []Warning{
			{Namespace: "shop", Kind: "Pod", Name: "api-7c9d-x2k", Reason: "BackOff", Message: "Back-off restarting failed container", Count: 12, Timestamp: fixedTime},
			{Namespace: "shop", Kind: "Pod", Name: "web-1", Reason: "Unhealthy", Message: "Readiness probe failed", Count: 1, Timestamp: fixedTime.Add(-2 * time.Minute)},
		},
	}
}

func TestRenderFullSnapshot(t *testing.T) {
	// given
	r := NewRenderer(config.StatusCanvas{})

	// when
	out := r.Render(fullSnapshot())

	// then
	golden.Assert(t, out, filepath.Join(t.Name(), "canvas.golden.md"))
}

func TestRenderEmptyCluster(t *testing.T) {
	// given
	cfg := config.StatusCanvas{}
	cfg.Sections.Catalog.LabelSelector = "botkube.io/canvas=true"
	r := NewRenderer(cfg)

	// when
	out := r.Render(Snapshot{Summary: Summary{ClusterName: "empty", Health: HealthUnknown}})

	// then
	golden.Assert(t, out, filepath.Join(t.Name(), "canvas.golden.md"))
}

func TestRenderIsDeterministic(t *testing.T) {
	// The publisher skips a canvas write when the rendered body is unchanged, which only holds if
	// rendering the same snapshot twice yields identical bytes.
	r := NewRenderer(config.StatusCanvas{})

	first := r.Render(fullSnapshot())
	second := r.Render(fullSnapshot())

	assert.Equal(t, first, second)
}

func TestRenderDisabledSections(t *testing.T) {
	// given
	cfg := config.StatusCanvas{}
	cfg.Sections.Nodes.Disabled = true
	cfg.Sections.Catalog.Disabled = true
	cfg.Sections.Warnings.Disabled = true
	r := NewRenderer(cfg)

	// when
	out := r.Render(fullSnapshot())

	// then
	assert.NotContains(t, out, "## Nodes")
	assert.NotContains(t, out, "## Service catalog")
	assert.NotContains(t, out, "## Recent warnings")
	assert.Contains(t, out, "## Unhealthy workloads")
}

func TestRenderReportsOmittedEntries(t *testing.T) {
	// A cap that silently dropped entries would make a partial section look complete.
	cfg := config.StatusCanvas{}
	cfg.Sections.Nodes.Limit = 2
	r := NewRenderer(cfg)

	out := r.Render(fullSnapshot())

	assert.Contains(t, out, "_...and 1 more nodes not shown._")
	assert.Contains(t, out, "node-2")
	assert.NotContains(t, out, "node-3")
}

func TestRenderEscapesTableCells(t *testing.T) {
	// A pipe would open a new column and a newline would end the row, corrupting every later row.
	r := NewRenderer(config.StatusCanvas{})

	out := r.Render(Snapshot{
		Warnings: []Warning{{
			Namespace: "default",
			Kind:      "Pod",
			Name:      "weird",
			Reason:    "Failed",
			Message:   "command failed: sh -c 'a | b'\nsecond line",
			Count:     1,
			Timestamp: fixedTime,
		}},
	})

	require.Contains(t, out, "\\|")
	// The row must remain a single line, otherwise the table breaks.
	for line := range strings.SplitSeq(out, "\n") {
		if strings.Contains(line, "command failed") {
			assert.Contains(t, line, "second line", "message should be flattened onto one row")
		}
	}
}

func TestTruncateText(t *testing.T) {
	tests := []struct {
		name     string
		in       string
		max      int
		expected string
	}{
		{name: "under limit is untouched", in: "short", max: 10, expected: "short"},
		{name: "over limit is truncated", in: "abcdefghij", max: 5, expected: "abcde…"},
		{name: "zero limit disables truncation", in: "abcdefghij", max: 0, expected: "abcdefghij"},
		// A byte-based cut would split the multi-byte rune and emit invalid UTF-8.
		{name: "multi-byte runes are not split", in: "日本語テスト", max: 3, expected: "日本語…"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, truncateText(tc.in, tc.max))
		})
	}
}

func TestFormatTimestampIsStable(t *testing.T) {
	// A relative age such as "5m ago" would change on every render even when the cluster did not,
	// defeating the publisher's no-op detection and rewriting the canvas forever.
	inUTC := formatTimestamp(fixedTime)
	inOtherZone := formatTimestamp(fixedTime.In(time.FixedZone("UTC+9", 9*60*60)))

	assert.Equal(t, inUTC, inOtherZone)
	assert.Equal(t, "unknown", formatTimestamp(time.Time{}))
}
