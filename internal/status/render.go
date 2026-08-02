package status

import (
	"fmt"
	"strings"
	"time"

	"github.com/iggy/botkube/pkg/config"
)

const (
	defaultNodesLimit     = 25
	defaultWorkloadsLimit = 25
	defaultCatalogLimit   = 50
	defaultWarningsLimit  = 15

	// maxMessageLen bounds how much of a warning message is shown, so one verbose event cannot push
	// the rest of the table off screen.
	maxMessageLen = 160
)

var healthIcon = map[Health]string{
	HealthOK:       ":large_green_circle:",
	HealthDegraded: ":large_yellow_circle:",
	HealthFailing:  ":red_circle:",
	HealthUnknown:  ":white_circle:",
}

var healthLabel = map[Health]string{
	HealthOK:       "Healthy",
	HealthDegraded: "Degraded",
	HealthFailing:  "Failing",
	HealthUnknown:  "Unknown",
}

// Renderer turns a Snapshot into the canvas markdown body.
//
// Render is a pure function of its input: determinism is load-bearing, because the publisher skips
// writing the canvas when the rendered body is unchanged from the last write.
type Renderer struct {
	cfg config.StatusCanvas
}

// NewRenderer returns a Renderer for the given configuration.
func NewRenderer(cfg config.StatusCanvas) *Renderer {
	return &Renderer{cfg: cfg}
}

// Render returns the full canvas markdown body for the given snapshot.
func (r *Renderer) Render(snap Snapshot) string {
	var b strings.Builder

	fmt.Fprintf(&b, "# %s %s\n\n", healthIcon[snap.Summary.Health], clusterTitle(snap.Summary.ClusterName))
	fmt.Fprintf(&b, "**Status:** %s\n", healthLabel[snap.Summary.Health])
	if snap.Summary.KubernetesVer != "" {
		fmt.Fprintf(&b, "**Kubernetes:** `%s`\n", snap.Summary.KubernetesVer)
	}
	fmt.Fprintf(&b, "**Nodes:** %d/%d ready\n", snap.Summary.NodesReady, snap.Summary.NodesTotal)
	fmt.Fprintf(&b, "**Pods:** %d total, %d unhealthy\n", snap.Summary.PodsTotal, snap.Summary.PodsUnhealthy)
	fmt.Fprintf(&b, "**Workloads:** %d in %d namespaces\n", snap.Summary.WorkloadsTotal, snap.Summary.NamespacesInScope)
	fmt.Fprintf(&b, "**Updated:** %s\n", formatTimestamp(snap.Summary.SnapshotAt))

	if !r.cfg.Sections.Nodes.Disabled {
		r.renderNodes(&b, snap.Nodes)
	}
	if !r.cfg.Sections.Workloads.Disabled {
		r.renderWorkloads(&b, snap.Workloads)
	}
	if !r.cfg.Sections.Catalog.Disabled {
		r.renderCatalog(&b, snap.Catalog)
	}
	if !r.cfg.Sections.Warnings.Disabled {
		r.renderWarnings(&b, snap.Warnings)
	}

	b.WriteString("\n---\n\n")
	b.WriteString("_Maintained by Botkube. Manual edits are overwritten on the next update._\n")

	return b.String()
}

func clusterTitle(name string) string {
	if name == "" {
		return "Kubernetes cluster"
	}
	return name
}

func (r *Renderer) renderNodes(b *strings.Builder, nodes []Node) {
	b.WriteString("\n## Nodes\n\n")
	if len(nodes) == 0 {
		b.WriteString("_No nodes found._\n")
		return
	}

	limit := limitFor(r.cfg.Sections.Nodes.Limit, defaultNodesLimit)
	shown, omitted := capEntries(nodes, limit)

	b.WriteString("| | Node | Ready | Schedulable | Kubelet | Reason |\n")
	b.WriteString("|---|---|---|---|---|---|\n")
	for _, n := range shown {
		schedulable := "yes"
		if !n.Schedulable {
			schedulable = "no"
		}
		fmt.Fprintf(b, "| %s | %s | %s | %s | `%s` | %s |\n",
			healthIcon[n.Health], escapeCell(n.Name), escapeCell(n.Ready), schedulable, escapeCell(n.KubeletVer), escapeCell(n.Reason))
	}
	writeOmitted(b, omitted, "nodes")
}

func (r *Renderer) renderWorkloads(b *strings.Builder, workloads []Workload) {
	b.WriteString("\n## Unhealthy workloads\n\n")
	if len(workloads) == 0 {
		b.WriteString("_All workloads are healthy._\n")
		return
	}

	limit := limitFor(r.cfg.Sections.Workloads.Limit, defaultWorkloadsLimit)
	shown, omitted := capEntries(workloads, limit)

	b.WriteString("| | Kind | Namespace | Name | Ready | Restarts | Reason |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, w := range shown {
		restarts := ""
		if w.Restarts > 0 {
			restarts = fmt.Sprintf("%d", w.Restarts)
		}
		fmt.Fprintf(b, "| %s | %s | `%s` | %s | %d/%d | %s | %s |\n",
			healthIcon[w.Health], escapeCell(w.Kind), escapeCell(w.Namespace), escapeCell(w.Name), w.Ready, w.Desired, restarts, escapeCell(w.Reason))
	}
	writeOmitted(b, omitted, "workloads")
}

func (r *Renderer) renderCatalog(b *strings.Builder, catalog []CatalogEntry) {
	b.WriteString("\n## Service catalog\n\n")
	if len(catalog) == 0 {
		b.WriteString("_No workloads opted in. Add the configured label to include a workload._\n")
		return
	}

	limit := limitFor(r.cfg.Sections.Catalog.Limit, defaultCatalogLimit)
	shown, omitted := capEntries(catalog, limit)

	b.WriteString("| | Workload | Namespace | Container | Version | Ready | Rollout |\n")
	b.WriteString("|---|---|---|---|---|---|---|\n")
	for _, entry := range shown {
		rollout := "settled"
		if entry.RollingOut {
			rollout = ":arrows_counterclockwise: in progress"
		}

		if len(entry.Images) == 0 {
			fmt.Fprintf(b, "| %s | %s | `%s` | | | %d/%d | %s |\n",
				healthIcon[entry.Health], escapeCell(entry.Name), escapeCell(entry.Namespace), entry.Ready, entry.Desired, rollout)
			continue
		}

		for i, img := range entry.Images {
			if i == 0 {
				fmt.Fprintf(b, "| %s | %s | `%s` | `%s` | `%s` | %d/%d | %s |\n",
					healthIcon[entry.Health], escapeCell(entry.Name), escapeCell(entry.Namespace), escapeCell(img.Container), escapeCell(img.Version), entry.Ready, entry.Desired, rollout)
				continue
			}
			fmt.Fprintf(b, "| | | | `%s` | `%s` | | |\n", escapeCell(img.Container), escapeCell(img.Version))
		}
	}
	writeOmitted(b, omitted, "catalog entries")
}

func (r *Renderer) renderWarnings(b *strings.Builder, warnings []Warning) {
	b.WriteString("\n## Recent warnings\n\n")
	if len(warnings) == 0 {
		b.WriteString("_No recent warnings._\n")
		return
	}

	limit := limitFor(r.cfg.Sections.Warnings.Limit, defaultWarningsLimit)
	shown, omitted := capEntries(warnings, limit)

	b.WriteString("| Age | Object | Reason | Count | Message |\n")
	b.WriteString("|---|---|---|---|---|\n")
	for _, w := range shown {
		count := ""
		if w.Count > 0 {
			count = fmt.Sprintf("x%d", w.Count)
		}
		fmt.Fprintf(b, "| %s | %s/%s/%s | %s | %s | %s |\n",
			formatTimestamp(w.Timestamp), escapeCell(w.Namespace), escapeCell(w.Kind), escapeCell(w.Name),
			escapeCell(w.Reason), count, escapeCell(truncateText(w.Message, maxMessageLen)))
	}
	writeOmitted(b, omitted, "warnings")
}

func limitFor(configured, def int) int {
	if configured > 0 {
		return configured
	}
	if configured < 0 {
		return 0
	}
	return def
}

// capEntries returns up to limit entries and the number omitted. A limit of 0 means unlimited.
func capEntries[T any](in []T, limit int) ([]T, int) {
	if limit <= 0 || len(in) <= limit {
		return in, 0
	}
	return in[:limit], len(in) - limit
}

// writeOmitted reports how many entries were capped, so a partial section never looks complete.
func writeOmitted(b *strings.Builder, omitted int, noun string) {
	if omitted <= 0 {
		return
	}
	fmt.Fprintf(b, "\n_...and %d more %s not shown._\n", omitted, noun)
}

// formatTimestamp renders an absolute UTC timestamp.
//
// A relative age such as "5m ago" would change on every render even when the cluster did not,
// defeating the publisher's no-op detection and rewriting the canvas forever.
func formatTimestamp(t time.Time) string {
	if t.IsZero() {
		return "unknown"
	}
	return t.UTC().Format("Mon, 02 Jan 2006 15:04:05 MST")
}

// escapeCell escapes characters that would corrupt a markdown table cell: a pipe would open a new
// column, and a newline would end the row.
func escapeCell(s string) string {
	s = strings.ReplaceAll(s, "|", "\\|")
	s = strings.ReplaceAll(s, "\r\n", " ")
	s = strings.ReplaceAll(s, "\n", " ")
	return s
}

// truncateText cuts on rune boundaries so a byte-based cut cannot split a multi-byte rune.
func truncateText(s string, max int) string {
	if max <= 0 {
		return s
	}
	runes := []rune(s)
	if len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "…"
}
