package status

import (
	"slices"
	"sort"
	"sync"
	"time"
)

// Section identifies one part of the rendered canvas.
type Section string

const (
	// SectionSummary is the cluster-wide summary section.
	SectionSummary Section = "summary"
	// SectionNodes is the node health section.
	SectionNodes Section = "nodes"
	// SectionWorkloads is the unhealthy workloads section.
	SectionWorkloads Section = "workloads"
	// SectionCatalog is the service catalog section.
	SectionCatalog Section = "catalog"
	// SectionWarnings is the recent warning events section.
	SectionWarnings Section = "warnings"
)

// Health describes the health of a single entry or of the cluster as a whole.
type Health string

const (
	// HealthOK means the entry is in its desired state.
	HealthOK Health = "ok"
	// HealthDegraded means the entry is impaired but still serving.
	HealthDegraded Health = "degraded"
	// HealthFailing means the entry is not serving at all.
	HealthFailing Health = "failing"
	// HealthUnknown means health could not be determined.
	HealthUnknown Health = "unknown"
)

// severity ranks health from best to worst, so the worse of two values can be picked.
var severity = map[Health]int{
	HealthOK:       0,
	HealthUnknown:  1,
	HealthDegraded: 2,
	HealthFailing:  3,
}

// WorseOf returns whichever of a and b is worse.
//
// Unknown ranks worse than OK: a health that could not be determined should not read as healthy.
func WorseOf(a, b Health) Health {
	if severity[b] > severity[a] {
		return b
	}
	return a
}

// Summary holds cluster-wide counters and the overall health verdict.
type Summary struct {
	ClusterName       string
	KubernetesVer     string
	NodesTotal        int
	NodesReady        int
	NamespacesInScope int
	PodsTotal         int
	PodsUnhealthy     int
	WorkloadsTotal    int
	Health            Health
	SnapshotAt        time.Time
}

// Node describes one cluster node.
type Node struct {
	Name        string
	Health      Health
	Ready       string
	Schedulable bool
	KubeletVer  string
	Reason      string
}

// Workload describes one workload or pod that is not in its desired state.
type Workload struct {
	Kind      string
	Namespace string
	Name      string
	Health    Health
	Ready     int32
	Desired   int32
	Reason    string
	Restarts  int32
}

// ContainerImage describes one container's declared image and version.
type ContainerImage struct {
	Container string
	Image     string
	Version   string
}

// CatalogEntry describes one opt-in workload in the service catalog.
type CatalogEntry struct {
	Kind       string
	Namespace  string
	Name       string
	Images     []ContainerImage
	Ready      int32
	Desired    int32
	RollingOut bool
	Health     Health
}

// Warning describes one recent warning event.
type Warning struct {
	Namespace string
	Kind      string
	Name      string
	Reason    string
	Message   string
	Count     int32
	Timestamp time.Time
}

// key identifies the object/reason a warning is about, used to update an existing entry instead of
// appending a duplicate for every occurrence.
func (w Warning) key() string {
	return w.Namespace + "/" + w.Kind + "/" + w.Name + "/" + w.Reason
}

// SnapshotData is the output of a single collection pass.
type SnapshotData struct {
	Summary   Summary
	Nodes     []Node
	Workloads []Workload
	Catalog   []CatalogEntry
	Warnings  []Warning
}

// Snapshot is a point-in-time, read-only view of the cluster state.
type Snapshot struct {
	Summary   Summary
	Nodes     []Node
	Workloads []Workload
	Catalog   []CatalogEntry
	Warnings  []Warning
}

// defaultMaxWarnings bounds the warnings ring buffer when none is configured.
const defaultMaxWarnings = 50

// ClusterState holds the current view of the cluster, built from a mix of periodic snapshots and
// event deltas. It is safe for concurrent use: the event observer writes while the publisher reads.
type ClusterState struct {
	mu sync.Mutex

	clusterName string
	revision    int64
	maxWarnings int

	summary   Summary
	nodes     []Node
	workloads []Workload
	catalog   []CatalogEntry
	warnings  []Warning

	stale map[Section]struct{}
}

// NewClusterState returns an empty ClusterState for the given cluster name.
func NewClusterState(clusterName string, maxWarnings int) *ClusterState {
	if maxWarnings <= 0 {
		maxWarnings = defaultMaxWarnings
	}
	return &ClusterState{
		clusterName: clusterName,
		maxWarnings: maxWarnings,
		summary:     Summary{ClusterName: clusterName, Health: HealthUnknown},
		stale:       map[Section]struct{}{},
	}
}

// Revision returns a counter that increases every time the state changes in a way that would change
// the rendered canvas. The publisher uses this to skip redundant renders.
func (s *ClusterState) Revision() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.revision
}

// Snapshot returns a deep copy of the current state, safe to render outside the lock.
func (s *ClusterState) Snapshot() Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	return Snapshot{
		Summary:   s.summary,
		Nodes:     append([]Node(nil), s.nodes...),
		Workloads: append([]Workload(nil), s.workloads...),
		Catalog:   append([]CatalogEntry(nil), s.catalog...),
		Warnings:  append([]Warning(nil), s.warnings...),
	}
}

// ApplySnapshot replaces the collected sections with a fresh full snapshot, deriving the overall
// health from it. Clears any pending stale flags, since a full snapshot already refreshed everything.
func (s *ClusterState) ApplySnapshot(data SnapshotData) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.nodes = sortedNodes(data.Nodes)
	s.workloads = sortedWorkloads(data.Workloads)
	s.catalog = sortedCatalog(data.Catalog)

	summary := data.Summary
	summary.ClusterName = s.clusterName
	summary.Health = deriveHealth(s.nodes, s.workloads)
	s.summary = summary

	s.stale = map[Section]struct{}{}
	s.revision++
}

// AddWarning applies a single warning event, updating an existing entry for the same object/reason
// in place rather than appending a duplicate. The running count never walks backwards, since events
// can be delivered out of order.
func (s *ClusterState) AddWarning(w Warning) {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := w.key()
	for i, existing := range s.warnings {
		if existing.key() != key {
			continue
		}
		if w.Count > existing.Count {
			s.warnings[i].Count = w.Count
		}
		if w.Timestamp.After(existing.Timestamp) {
			s.warnings[i].Timestamp = w.Timestamp
			s.warnings[i].Message = w.Message
		}
		s.revision++
		return
	}

	s.warnings = append(s.warnings, w)
	s.warnings = capWarnings(s.warnings, s.maxWarnings)
	s.revision++
}

// MarkStale flags the given sections as needing a re-collect.
//
// This deliberately does not bump the revision: marking stale means "re-collect this", not "the
// canvas changed". Bumping here would publish an unchanged canvas on every event that carries no
// renderable data.
func (s *ClusterState) MarkStale(sections ...Section) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, section := range sections {
		s.stale[section] = struct{}{}
	}
}

// TakeStale returns the sections flagged stale since the last call and clears the flags.
func (s *ClusterState) TakeStale() []Section {
	s.mu.Lock()
	defer s.mu.Unlock()

	if len(s.stale) == 0 {
		return nil
	}

	out := make([]Section, 0, len(s.stale))
	for section := range s.stale {
		out = append(out, section)
	}
	s.stale = map[Section]struct{}{}

	slices.Sort(out)
	return out
}

// deriveHealth reduces the collected entries to a single cluster verdict.
func deriveHealth(nodes []Node, workloads []Workload) Health {
	if len(nodes) == 0 {
		return HealthUnknown
	}

	health := HealthOK
	for _, n := range nodes {
		health = WorseOf(health, n.Health)
	}
	for _, w := range workloads {
		health = WorseOf(health, w.Health)
	}
	return health
}

func sortedNodes(in []Node) []Node {
	out := append([]Node(nil), in...)
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func sortedWorkloads(in []Workload) []Workload {
	out := append([]Workload(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		if out[i].Kind != out[j].Kind {
			return out[i].Kind < out[j].Kind
		}
		return out[i].Name < out[j].Name
	})
	return out
}

func sortedCatalog(in []CatalogEntry) []CatalogEntry {
	out := append([]CatalogEntry(nil), in...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Namespace != out[j].Namespace {
			return out[i].Namespace < out[j].Namespace
		}
		return out[i].Name < out[j].Name
	})
	return out
}

// capWarnings trims to maxLen, dropping the oldest entries and keeping the newest first.
func capWarnings(in []Warning, maxLen int) []Warning {
	sort.Slice(in, func(i, j int) bool { return in[i].Timestamp.After(in[j].Timestamp) })
	if len(in) > maxLen {
		in = in[:maxLen]
	}
	return in
}
