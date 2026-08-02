package status

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/slack-go/slack"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/client-go/kubernetes/fake"

	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/loggerx"
)

// fakeCanvasClient records the canvas calls the publisher makes and replays scripted errors, so the
// publisher's decisions can be asserted without a Slack workspace.
type fakeCanvasClient struct {
	mu sync.Mutex

	// existingCanvasID is returned by conversations.info; empty means the channel has no canvas yet.
	existingCanvasID string
	// createdCanvasID is returned by a successful create.
	createdCanvasID string
	// existingAfterCreate becomes the existing canvas once a create has been attempted, which models
	// another client winning the create race.
	existingAfterCreate string

	// createErr is returned by the next create call, then cleared.
	createErr error
	// editErr is returned by the next edit call, then cleared.
	editErr error
	// infoErr is returned by every conversations.info call.
	infoErr error

	channels []slack.Channel

	creates []slack.DocumentContent
	edits   []slack.EditCanvasParams
	infos   int
	lists   int
}

func (f *fakeCanvasClient) GetConversationInfoContext(_ context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.infos++
	if f.infoErr != nil {
		return nil, f.infoErr
	}

	ch := &slack.Channel{}
	ch.ID = input.ChannelID
	ch.Properties = &slack.Properties{Canvas: slack.Canvas{FileId: f.existingCanvasID}}
	return ch, nil
}

func (f *fakeCanvasClient) GetConversationsContext(_ context.Context, _ *slack.GetConversationsParameters) ([]slack.Channel, string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.lists++
	return f.channels, "", nil
}

func (f *fakeCanvasClient) CreateChannelCanvasContext(_ context.Context, _ string, content slack.DocumentContent, _ ...slack.CreateChannelCanvasOption) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.creates = append(f.creates, content)
	if f.existingAfterCreate != "" {
		f.existingCanvasID = f.existingAfterCreate
	}
	if err := f.createErr; err != nil {
		f.createErr = nil
		return "", err
	}
	return f.createdCanvasID, nil
}

func (f *fakeCanvasClient) EditCanvasContext(_ context.Context, params slack.EditCanvasParams) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.edits = append(f.edits, params)
	if err := f.editErr; err != nil {
		f.editErr = nil
		return err
	}
	return nil
}

func (f *fakeCanvasClient) snapshotCalls() (creates []slack.DocumentContent, edits []slack.EditCanvasParams) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.creates, f.edits
}

// namedChannel builds the conversations.list entry for a channel name.
func namedChannel(id, name string) slack.Channel {
	ch := slack.Channel{}
	ch.ID = id
	ch.Name = name
	return ch
}

// newTestPublisher wires a publisher over a fake Slack client and an empty cluster.
func newTestPublisher(t *testing.T, client CanvasClient, cfg config.StatusCanvas) (*Publisher, *ClusterState) {
	t.Helper()

	collector, err := NewCollector(loggerx.NewNoop(), fake.NewSimpleClientset(), cfg)
	require.NoError(t, err)

	state := NewClusterState("prod-eu", 10)
	return NewPublisher(loggerx.NewNoop(), cfg, client, collector, state), state
}

func TestPublisherCreatesCanvasWithBody(t *testing.T) {
	// given
	client := &fakeCanvasClient{createdCanvasID: "F123", channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	// when
	require.NoError(t, publisher.publish(context.Background()))

	// then
	creates, edits := client.snapshotCalls()
	require.Len(t, creates, 1)
	assert.Equal(t, canvasMarkdownType, creates[0].Type)
	assert.Contains(t, creates[0].Markdown, "node-1", "a new canvas should be created with content, never briefly empty")
	assert.Empty(t, edits, "the created canvas already holds the body, so no edit is needed")
}

func TestPublisherAdoptsExistingCanvas(t *testing.T) {
	// A canvas created by a previous Botkube run is discoverable from the channel, so there is no
	// persisted ID and nothing to recreate.
	client := &fakeCanvasClient{existingCanvasID: "F999", channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	require.NoError(t, publisher.publish(context.Background()))

	creates, edits := client.snapshotCalls()
	assert.Empty(t, creates)
	require.Len(t, edits, 1)
	assert.Equal(t, "F999", edits[0].CanvasID)
}

func TestPublisherAdoptsCanvasAfterCreateRace(t *testing.T) {
	// Slack rejects a second canvas for the channel. Creating and adopting are the same intent, so
	// the publisher must fall back to the existing canvas rather than failing.
	tests := []string{"channel_canvas_already_exists", "free_team_canvas_tab_already_exists"}

	for _, slackErr := range tests {
		t.Run(slackErr, func(t *testing.T) {
			client := &fakeCanvasClient{
				channels:  []slack.Channel{namedChannel("C123", "ops")},
				createErr: slack.SlackErrorResponse{Err: slackErr},
				// The canvas appears between the first lookup and the create.
				existingAfterCreate: "F777",
			}
			publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
			state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

			require.NoError(t, publisher.publish(context.Background()))

			creates, edits := client.snapshotCalls()
			require.Len(t, creates, 1)
			require.Len(t, edits, 1)
			assert.Equal(t, "F777", edits[0].CanvasID)
		})
	}
}

func TestPublisherSkipsUnchangedBody(t *testing.T) {
	// Rendering is deterministic, so an unchanged cluster must not spend canvases.edit rate limit.
	client := &fakeCanvasClient{existingCanvasID: "F999", channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	require.NoError(t, publisher.publish(context.Background()))
	require.NoError(t, publisher.publish(context.Background()))

	_, edits := client.snapshotCalls()
	assert.Len(t, edits, 1, "a second publish of identical state should be a no-op")
}

func TestPublisherWritesChangedBody(t *testing.T) {
	// given
	client := &fakeCanvasClient{existingCanvasID: "F999", channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	// when
	require.NoError(t, publisher.publish(context.Background()))
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthFailing, Reason: "KubeletNotReady"}}})
	require.NoError(t, publisher.publish(context.Background()))

	// then
	_, edits := client.snapshotCalls()
	require.Len(t, edits, 2)
	assert.Equal(t, "replace", edits[1].Changes[0].Operation)
	// No section_id means a whole-canvas replace, which matches the full-rewrite model.
	assert.Empty(t, edits[1].Changes[0].SectionID)
	assert.Contains(t, edits[1].Changes[0].DocumentContent.Markdown, "KubeletNotReady")
}

func TestPublisherFatalErrorDisablesChannel(t *testing.T) {
	// A plan restriction or missing scope never resolves on its own, so the channel is dropped and
	// reported once instead of failing on every tick.
	client := &fakeCanvasClient{
		channels:  []slack.Channel{namedChannel("C123", "ops")},
		createErr: slack.SlackErrorResponse{Err: "team_tier_cannot_create_channel_canvases"},
	}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	err := publisher.publish(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not allow creating channel canvases")

	// A later publish must not retry the doomed channel.
	client.createdCanvasID = "F123"
	require.NoError(t, publisher.publish(context.Background()))

	creates, _ := client.snapshotCalls()
	assert.Len(t, creates, 1, "a permanently failing channel should be attempted only once")
}

func TestPublisherRetriesTransientError(t *testing.T) {
	// A rate limit or a network blip is not a reason to give up on the channel.
	client := &fakeCanvasClient{
		existingCanvasID: "F999",
		channels:         []slack.Channel{namedChannel("C123", "ops")},
		editErr:          slack.SlackErrorResponse{Err: "ratelimited"},
	}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	require.Error(t, publisher.publish(context.Background()))
	require.NoError(t, publisher.publish(context.Background()), "the next attempt should succeed")

	_, edits := client.snapshotCalls()
	assert.Len(t, edits, 2)
}

func TestPublisherRediscoversDeletedCanvas(t *testing.T) {
	// A canvas deleted in Slack leaves a stale cached ID. Editing it would fail forever, so the ID is
	// dropped and the next attempt recreates the canvas.
	client := &fakeCanvasClient{
		existingCanvasID: "F999",
		createdCanvasID:  "F111",
		channels:         []slack.Channel{namedChannel("C123", "ops")},
		editErr:          slack.SlackErrorResponse{Err: "canvas_not_found"},
	}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	require.Error(t, publisher.publish(context.Background()))
	client.existingCanvasID = ""

	require.NoError(t, publisher.publish(context.Background()))

	creates, _ := client.snapshotCalls()
	require.Len(t, creates, 1, "the canvas should be recreated once the cached ID is known to be gone")
}

func TestPublisherPublishesToEveryChannel(t *testing.T) {
	// given
	client := &fakeCanvasClient{
		existingCanvasID: "F999",
		channels:         []slack.Channel{namedChannel("C123", "ops"), namedChannel("C456", "alerts")},
	}
	publisher, state := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"ops", "#alerts"}})
	state.ApplySnapshot(SnapshotData{Nodes: []Node{{Name: "node-1", Health: HealthOK}}})

	// when
	require.NoError(t, publisher.publish(context.Background()))

	// then
	_, edits := client.snapshotCalls()
	assert.Len(t, edits, 2, "a leading # in the configured channel should not prevent resolution")
}

func TestPublisherReportsUnknownChannel(t *testing.T) {
	// given
	client := &fakeCanvasClient{channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher, _ := newTestPublisher(t, client, config.StatusCanvas{Channels: []string{"missing"}})

	// when
	err := publisher.publish(context.Background())

	// then
	require.Error(t, err)
	assert.Contains(t, err.Error(), "make sure Botkube is invited to it")
}

func TestCanvasTitle(t *testing.T) {
	tests := []struct {
		name        string
		title       string
		clusterName string
		expected    string
	}{
		{
			name:        "cluster name is interpolated",
			title:       "{{ .ClusterName }} cluster status",
			clusterName: "prod-eu",
			expected:    "prod-eu cluster status",
		},
		{
			name:        "static title is kept",
			title:       "Cluster status",
			clusterName: "prod-eu",
			expected:    "Cluster status",
		},
		{
			name:        "empty title falls back to the default",
			clusterName: "prod-eu",
			expected:    "prod-eu cluster status",
		},
		{
			// A canvas titled " cluster status" would read as a bug, so an unset cluster name gets a
			// placeholder.
			name:     "missing cluster name falls back",
			title:    "{{ .ClusterName }} cluster status",
			expected: "Kubernetes cluster status",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			collector, err := NewCollector(loggerx.NewNoop(), fake.NewSimpleClientset(), config.StatusCanvas{})
			require.NoError(t, err)

			publisher := NewPublisher(
				loggerx.NewNoop(),
				config.StatusCanvas{Title: tc.title},
				&fakeCanvasClient{},
				collector,
				NewClusterState(tc.clusterName, 10),
			)

			out, err := publisher.canvasTitle()
			require.NoError(t, err)
			assert.Equal(t, tc.expected, out)
		})
	}
}

func TestCanvasTitleRejectsBrokenTemplate(t *testing.T) {
	collector, err := NewCollector(loggerx.NewNoop(), fake.NewSimpleClientset(), config.StatusCanvas{})
	require.NoError(t, err)

	publisher := NewPublisher(
		loggerx.NewNoop(),
		config.StatusCanvas{Title: "{{ .ClusterName "},
		&fakeCanvasClient{},
		collector,
		NewClusterState("prod-eu", 10),
	)

	_, err = publisher.canvasTitle()
	require.Error(t, err)
}

func TestPublisherStartPublishesImmediately(t *testing.T) {
	// The canvas must be correct as soon as Botkube is up, not blank until the first tick.
	k8sCli := fake.NewSimpleClientset(namespace("app"), readyNode("node-1"))
	cfg := config.StatusCanvas{
		Channels:         []string{"ops"},
		UpdateInterval:   minUpdateInterval,
		SnapshotInterval: time.Minute,
	}
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, cfg)
	require.NoError(t, err)

	client := &fakeCanvasClient{createdCanvasID: "F123", channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher := NewPublisher(loggerx.NewNoop(), cfg, client, collector, NewClusterState("prod-eu", 10))

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- publisher.Start(ctx) }()

	assert.Eventually(t, func() bool {
		creates, _ := client.snapshotCalls()
		return len(creates) == 1
	}, 5*time.Second, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-done, "a cancelled context should shut down cleanly")
}

func TestPublisherStartCoalescesEvents(t *testing.T) {
	// The debounce window is the point of the update ticker: a burst of events must produce one canvas
	// write, not one per event.
	k8sCli := fake.NewSimpleClientset(namespace("app"), readyNode("node-1"))
	cfg := config.StatusCanvas{
		Channels:         []string{"ops"},
		UpdateInterval:   minUpdateInterval,
		SnapshotInterval: time.Hour,
	}
	collector, err := NewCollector(loggerx.NewNoop(), k8sCli, cfg)
	require.NoError(t, err)

	client := &fakeCanvasClient{existingCanvasID: "F999", channels: []slack.Channel{namedChannel("C123", "ops")}}
	state := NewClusterState("prod-eu", 10)
	publisher := NewPublisher(loggerx.NewNoop(), cfg, client, collector, state)
	observer := NewObserver(loggerx.NewNoop(), state, cfg)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- publisher.Start(ctx) }()

	// Wait for the initial publish so later writes are attributable to the burst.
	require.Eventually(t, func() bool {
		_, edits := client.snapshotCalls()
		return len(edits) == 1
	}, 5*time.Second, 10*time.Millisecond)

	// A hundred warnings for distinct pods, all inside one debounce window.
	for i := range 100 {
		observer.ObserveEvent("botkube/kubernetes", map[string]any{
			"Kind":      "Pod",
			"Name":      "pod-" + string(rune('a'+i%26)),
			"Namespace": "app",
			"Level":     string(config.Error),
			"Reason":    "BackOff",
			"Messages":  []string{"burst"},
			"Count":     i,
		})
	}

	require.Eventually(t, func() bool {
		_, edits := client.snapshotCalls()
		return len(edits) == 2
	}, 5*time.Second, 10*time.Millisecond)

	// Give the ticker room to fire again; a correct debounce writes nothing more once state settles.
	time.Sleep(3 * minUpdateInterval)

	_, edits := client.snapshotCalls()
	assert.Len(t, edits, 2, "a burst of 100 events should collapse into a single canvas write")

	cancel()
	require.NoError(t, <-done)
}

func TestPublisherStartFloorsUpdateInterval(t *testing.T) {
	// canvases.edit is rate limited, so a sub-second interval configured by mistake must not be
	// honoured as written.
	cfg := config.StatusCanvas{Channels: []string{"ops"}, UpdateInterval: time.Millisecond}
	collector, err := NewCollector(loggerx.NewNoop(), fake.NewSimpleClientset(), cfg)
	require.NoError(t, err)

	client := &fakeCanvasClient{existingCanvasID: "F999", channels: []slack.Channel{namedChannel("C123", "ops")}}
	publisher := NewPublisher(loggerx.NewNoop(), cfg, client, collector, NewClusterState("prod-eu", 10))

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	require.NoError(t, publisher.Start(ctx))

	// With the interval floored to seconds, a one-second run cannot produce a flood of writes.
	_, edits := client.snapshotCalls()
	assert.LessOrEqual(t, len(edits), 2)
}

func TestExistingCanvasIDHandlesChannelWithoutProperties(t *testing.T) {
	// conversations.info omits properties for a channel that never had a canvas; that is an absent
	// canvas, not an error.
	client := &fakeCanvasClient{}
	publisher, _ := newTestPublisher(t, client, config.StatusCanvas{})

	id, err := publisher.existingCanvasID(context.Background(), "C123")

	require.NoError(t, err)
	assert.Empty(t, id)
}

func TestFatalCanvasError(t *testing.T) {
	tests := []struct {
		slackErr string
		fatal    bool
	}{
		{slackErr: "team_tier_cannot_create_channel_canvases", fatal: true},
		{slackErr: "canvas_disabled_user_team", fatal: true},
		{slackErr: "missing_scope", fatal: true},
		{slackErr: "not_allowed_token_type", fatal: true},
		// Transient failures must stay retryable.
		{slackErr: "ratelimited", fatal: false},
		{slackErr: "channel_not_found", fatal: false},
		{slackErr: "channel_canvas_already_exists", fatal: false},
	}

	for _, tc := range tests {
		t.Run(tc.slackErr, func(t *testing.T) {
			err := fatalCanvasError(slack.SlackErrorResponse{Err: tc.slackErr})
			assert.Equal(t, tc.fatal, err != nil)
		})
	}

	assert.Nil(t, fatalCanvasError(nil))
}

func TestLooksLikeChannelID(t *testing.T) {
	tests := []struct {
		in       string
		expected bool
	}{
		{in: "C0123456789", expected: true},
		{in: "G0123456789", expected: true},
		{in: "D0123456789", expected: true},
		// Channel names are lowercase, so any lowercase rules out an ID.
		{in: "channel-name", expected: false},
		{in: "C0123456789a", expected: false},
		{in: "ops", expected: false},
		{in: "", expected: false},
		{in: "X0123456789", expected: false},
	}

	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			assert.Equal(t, tc.expected, looksLikeChannelID(tc.in))
		})
	}
}

func TestChannelResolverCachesLookups(t *testing.T) {
	// Channel IDs are stable, so paging conversations.list on every debounce tick would be waste.
	// "alerts" is listed first so resolving "ops" walks past it, which is what gets it cached.
	client := &fakeCanvasClient{channels: []slack.Channel{namedChannel("C456", "alerts"), namedChannel("C123", "ops")}}
	resolver := newChannelResolver(client)

	for range 3 {
		id, err := resolver.Resolve(context.Background(), "ops")
		require.NoError(t, err)
		assert.Equal(t, "C123", id)
	}

	// Channels seen while paging are cached too, so a second configured channel is free.
	id, err := resolver.Resolve(context.Background(), "alerts")
	require.NoError(t, err)
	assert.Equal(t, "C456", id)

	client.mu.Lock()
	defer client.mu.Unlock()
	assert.Equal(t, 1, client.lists)
}

func TestChannelResolverAcceptsChannelID(t *testing.T) {
	// given
	client := &fakeCanvasClient{}
	resolver := newChannelResolver(client)

	// when
	id, err := resolver.Resolve(context.Background(), "C0123456789")

	// then
	require.NoError(t, err)
	assert.Equal(t, "C0123456789", id)

	client.mu.Lock()
	defer client.mu.Unlock()
	assert.Zero(t, client.lists, "an ID needs no conversations.list paging")
}

func TestChannelResolverRejectsEmptyChannel(t *testing.T) {
	_, err := newChannelResolver(&fakeCanvasClient{}).Resolve(context.Background(), "")
	require.Error(t, err)
}
