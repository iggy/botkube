package status

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"text/template"
	"time"

	"github.com/sirupsen/logrus"
	"github.com/slack-go/slack"

	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/multierror"
)

const (
	defaultUpdateInterval   = 10 * time.Second
	defaultSnapshotInterval = 5 * time.Minute

	// minUpdateInterval floors the debounce window. canvases.edit is a Tier 3 method, so a very
	// small window configured by mistake would spend the rate limit budget without adding value.
	minUpdateInterval = 2 * time.Second

	// canvasMarkdownType is the only document content type canvases accept.
	canvasMarkdownType = "markdown"
)

// Publisher keeps one Slack channel canvas per configured channel in sync with the cluster state.
type Publisher struct {
	log       logrus.FieldLogger
	cfg       config.StatusCanvas
	client    CanvasClient
	collector *Collector
	renderer  *Renderer
	state     *ClusterState
	resolver  *channelResolver

	// canvasIDs caches the resolved canvas per configured channel.
	canvasIDs map[string]string

	// disabled records channels Botkube has given up on, so a permanent failure is logged once
	// rather than on every tick.
	disabled map[string]struct{}

	// lastPublished is the last body written per channel, used to skip no-op writes.
	lastPublished map[string]string

	mu sync.Mutex
}

// NewPublisher returns a Publisher for the given configuration.
func NewPublisher(log logrus.FieldLogger, cfg config.StatusCanvas, client CanvasClient, collector *Collector, state *ClusterState) *Publisher {
	return &Publisher{
		log:           log,
		cfg:           cfg,
		client:        client,
		collector:     collector,
		renderer:      NewRenderer(cfg),
		state:         state,
		resolver:      newChannelResolver(client),
		canvasIDs:     map[string]string{},
		disabled:      map[string]struct{}{},
		lastPublished: map[string]string{},
	}
}

// Start collects an initial snapshot, publishes it, and then keeps the canvas up to date until the
// context is cancelled.
//
// Two timers drive it. The update ticker is the debounce window: it publishes only when the state
// revision moved, so a burst of events produces one canvas write. The snapshot ticker re-lists the
// cluster, which both picks up changes no event described and corrects any drift.
func (p *Publisher) Start(ctx context.Context) error {
	updateInterval := p.cfg.UpdateInterval
	if updateInterval < minUpdateInterval {
		if updateInterval > 0 {
			p.log.Warnf("Status canvas updateInterval %s is below the %s minimum; using the minimum instead.", updateInterval, minUpdateInterval)
		}
		updateInterval = max(defaultUpdateInterval, minUpdateInterval)
	}

	snapshotInterval := p.cfg.SnapshotInterval
	if snapshotInterval <= 0 {
		snapshotInterval = defaultSnapshotInterval
	}

	p.log.WithFields(logrus.Fields{
		"updateInterval":   updateInterval.String(),
		"snapshotInterval": snapshotInterval.String(),
		"channels":         strings.Join(p.cfg.Channels, ","),
	}).Info("Starting cluster status canvas")

	// Collect and publish immediately so the canvas is correct as soon as Botkube is up, instead of
	// showing nothing until the first tick.
	p.collect(ctx)
	if err := p.publish(ctx); err != nil {
		// A first-publish failure is logged rather than returned: it is often a missing invite to
		// one channel, which should not take the whole agent down.
		p.log.Errorf("while publishing initial cluster status canvas: %s", err)
	}

	updateTicker := time.NewTicker(updateInterval)
	defer updateTicker.Stop()
	snapshotTicker := time.NewTicker(snapshotInterval)
	defer snapshotTicker.Stop()

	var lastRevision = p.state.Revision()

	for {
		select {
		case <-ctx.Done():
			p.log.Info("Shutdown requested. Finishing...")
			return nil

		case <-snapshotTicker.C:
			p.collect(ctx)

		case <-updateTicker.C:
			// Sections flagged by the event path need object status the event could not carry, so
			// ask the collector for fresh data before rendering.
			if stale := p.state.TakeStale(); len(stale) > 0 {
				p.log.WithField("sections", stale).Debug("Re-collecting cluster state for stale sections")
				p.collect(ctx)
			}

			revision := p.state.Revision()
			if revision == lastRevision {
				continue
			}
			lastRevision = revision

			if err := p.publish(ctx); err != nil {
				p.log.Errorf("while publishing cluster status canvas: %s", err)
			}
		}
	}
}

// collect runs a full snapshot and applies it to the state.
func (p *Publisher) collect(ctx context.Context) {
	data, err := p.collector.Collect(ctx)
	if err != nil {
		// Collect returns partial data alongside its error, so the snapshot is still applied: a
		// canvas missing one section beats a canvas frozen at an older state.
		p.log.Errorf("while collecting cluster state: %s", err)
	}

	data.Summary.SnapshotAt = time.Now()
	p.state.ApplySnapshot(data)
}

// publish renders the current state and writes it to every configured channel canvas.
func (p *Publisher) publish(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	body := p.renderer.Render(p.state.Snapshot())
	errs := multierror.New()

	for _, channel := range p.cfg.Channels {
		if _, skip := p.disabled[channel]; skip {
			continue
		}

		if err := p.publishToChannel(ctx, channel, body); err != nil {
			// A workspace-level problem such as a plan restriction or a missing scope will never
			// resolve on its own, so stop retrying that channel and say so once.
			if fatal := fatalCanvasError(err); fatal != nil {
				p.disabled[channel] = struct{}{}
				errs = multierror.Append(errs, fmt.Errorf("status canvas disabled for channel %q: %w", channel, fatal))
				continue
			}
			errs = multierror.Append(errs, fmt.Errorf("while publishing canvas to channel %q: %w", channel, err))
			continue
		}

		p.lastPublished[channel] = body
	}

	return errs.ErrorOrNil()
}

func (p *Publisher) publishToChannel(ctx context.Context, channel, body string) error {
	// Rendering is deterministic, so an identical body means the cluster view did not change and
	// the write would be a no-op that still costs rate limit budget.
	if p.lastPublished[channel] == body {
		return nil
	}

	canvasID, err := p.canvasIDFor(ctx, channel, body)
	if err != nil {
		return err
	}

	// A freshly created canvas already holds this body.
	if p.lastPublished[channel] == body {
		return nil
	}

	// replace without a section_id replaces the whole canvas, which matches the full-rewrite model:
	// the rendered body is always the complete current state.
	err = p.client.EditCanvasContext(ctx, slack.EditCanvasParams{
		CanvasID: canvasID,
		Changes: []slack.CanvasChange{
			{
				Operation: "replace",
				DocumentContent: slack.DocumentContent{
					Type:     canvasMarkdownType,
					Markdown: body,
				},
			},
		},
	})
	if err != nil {
		// The canvas may have been deleted in Slack since it was cached. Drop the cached ID so the
		// next tick rediscovers or recreates it instead of editing a canvas that is gone.
		if isCanvasNotFoundError(err) {
			delete(p.canvasIDs, channel)
		}
		return fmt.Errorf("while editing canvas %q: %w", canvasID, err)
	}

	return nil
}

// canvasIDFor returns the channel's canvas, adopting an existing one or creating it if absent.
//
// No canvas ID is persisted anywhere: a channel canvas is discoverable from the channel itself, so
// the ID survives restarts without Botkube storing state.
func (p *Publisher) canvasIDFor(ctx context.Context, channel, body string) (string, error) {
	if id, ok := p.canvasIDs[channel]; ok {
		return id, nil
	}

	channelID, err := p.resolver.Resolve(ctx, channel)
	if err != nil {
		return "", err
	}

	if id, err := p.existingCanvasID(ctx, channelID); err != nil {
		return "", err
	} else if id != "" {
		p.canvasIDs[channel] = id
		return id, nil
	}

	title, err := p.canvasTitle()
	if err != nil {
		return "", err
	}

	// Create with the rendered body so a new canvas is never briefly empty.
	id, err := p.client.CreateChannelCanvasContext(ctx, channelID, slack.DocumentContent{
		Type:     canvasMarkdownType,
		Markdown: body,
	}, slack.CreateChannelCanvasOptionTitle(title))
	if err != nil {
		// Losing a create race, or hitting the free-plan one-tab-per-channel limit, both mean a
		// canvas exists to adopt.
		if isCanvasExistsError(err) {
			existing, lookupErr := p.existingCanvasID(ctx, channelID)
			if lookupErr != nil {
				return "", fmt.Errorf("while looking up existing canvas after %q: %w", err, lookupErr)
			}
			if existing == "" {
				return "", fmt.Errorf("Slack reports a canvas already exists for channel %q but it could not be found: %w", channel, err)
			}
			p.canvasIDs[channel] = existing
			return existing, nil
		}
		return "", fmt.Errorf("while creating channel canvas: %w", err)
	}

	p.canvasIDs[channel] = id
	p.lastPublished[channel] = body
	p.log.WithFields(logrus.Fields{"channel": channel, "canvasID": id}).Info("Created cluster status channel canvas")

	return id, nil
}

// existingCanvasID returns the channel's current canvas ID, or an empty string when it has none.
func (p *Publisher) existingCanvasID(ctx context.Context, channelID string) (string, error) {
	info, err := p.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{ChannelID: channelID})
	if err != nil {
		return "", fmt.Errorf("while getting info for channel %q: %w", channelID, err)
	}
	if info == nil || info.Properties == nil {
		return "", nil
	}

	return info.Properties.Canvas.FileId, nil
}

// canvasTitle renders the configured canvas title template.
func (p *Publisher) canvasTitle() (string, error) {
	raw := strings.TrimSpace(p.cfg.Title)
	if raw == "" {
		raw = "{{ .ClusterName }} cluster status"
	}

	tmpl, err := template.New("canvasTitle").Parse(raw)
	if err != nil {
		return "", fmt.Errorf("while parsing status canvas title %q: %w", raw, err)
	}

	clusterName := p.state.Snapshot().Summary.ClusterName
	if clusterName == "" {
		clusterName = "Kubernetes"
	}

	var out strings.Builder
	if err := tmpl.Execute(&out, struct{ ClusterName string }{ClusterName: clusterName}); err != nil {
		return "", fmt.Errorf("while rendering status canvas title: %w", err)
	}

	return out.String(), nil
}

// isCanvasNotFoundError reports whether err means the canvas no longer exists.
func isCanvasNotFoundError(err error) bool {
	switch slackErrorCode(err) {
	case "canvas_not_found", "file_not_found", "channel_canvas_not_found":
		return true
	default:
		return false
	}
}
