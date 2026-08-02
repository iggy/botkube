package status

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/slack-go/slack"

	"github.com/iggy/botkube/pkg/conversation"
)

// CanvasClient is the narrow slice of the Slack API the status canvas needs.
//
// Keeping it an interface rather than taking *slack.Client directly lets the publisher be tested
// without a Slack server, and documents exactly which four calls this feature makes.
type CanvasClient interface {
	// GetConversationInfoContext resolves channel metadata, including any existing channel canvas.
	GetConversationInfoContext(ctx context.Context, input *slack.GetConversationInfoInput) (*slack.Channel, error)

	// GetConversationsContext lists channels, used to resolve configured channel names to IDs.
	GetConversationsContext(ctx context.Context, params *slack.GetConversationsParameters) ([]slack.Channel, string, error)

	// CreateChannelCanvasContext creates a channel canvas.
	CreateChannelCanvasContext(ctx context.Context, channel string, documentContent slack.DocumentContent, options ...slack.CreateChannelCanvasOption) (string, error)

	// EditCanvasContext applies changes to an existing canvas.
	EditCanvasContext(ctx context.Context, params slack.EditCanvasParams) error
}

// Slack errors that mean the workspace cannot host a Botkube status canvas at all. These are
// configuration problems, not transient failures, so the publisher reports them and gives up on the
// channel rather than retrying forever.
var fatalCanvasErrors = map[string]string{
	"team_tier_cannot_create_channel_canvases": "the Slack workspace plan does not allow creating channel canvases",
	"canvas_disabled_user_team":                "canvases are disabled for the Slack workspace",
	"canvases_disabled_user_team":              "canvases are disabled for the Slack workspace",
	"missing_scope":                            "the Slack bot token is missing the 'canvases:write' scope",
	"not_allowed_token_type":                   "the Slack token type cannot manage canvases",
}

// Slack errors that mean the canvas already exists. Botkube then adopts the existing canvas instead
// of failing: creating and adopting are the same intent, and on free plans only one canvas tab per
// channel is permitted.
var canvasExistsErrors = map[string]struct{}{
	"channel_canvas_already_exists":       {},
	"free_team_canvas_tab_already_exists": {},
}

// slackErrorCode extracts the Slack API error code from err.
//
// Canvas methods return a SlackErrorResponse carrying the code, but the code can also arrive as a
// plain error when it comes from a wrapped or synthesized failure, so both are handled.
func slackErrorCode(err error) string {
	if err == nil {
		return ""
	}

	var slackErr slack.SlackErrorResponse
	if errors.As(err, &slackErr) {
		return slackErr.Err
	}
	return err.Error()
}

// fatalCanvasError returns a descriptive error when err is one Botkube cannot recover from.
func fatalCanvasError(err error) error {
	if reason, ok := fatalCanvasErrors[slackErrorCode(err)]; ok {
		return fmt.Errorf("%s (Slack error %q)", reason, slackErrorCode(err))
	}
	return nil
}

// isCanvasExistsError reports whether err means the channel already has a canvas.
func isCanvasExistsError(err error) bool {
	_, ok := canvasExistsErrors[slackErrorCode(err)]
	return ok
}

// channelResolver maps configured channel names or IDs to Slack channel IDs.
//
// Resolved IDs are cached because channel IDs are stable for the lifetime of a channel, and the
// alternative would be paging conversations.list on every debounce tick.
type channelResolver struct {
	client CanvasClient
	cache  map[string]string
}

func newChannelResolver(client CanvasClient) *channelResolver {
	return &channelResolver{client: client, cache: map[string]string{}}
}

// maxConversationPages bounds paging through conversations.list so an enormous workspace cannot
// stall startup indefinitely.
const maxConversationPages = 20

// Resolve returns the channel ID for the given configured channel reference.
func (r *channelResolver) Resolve(ctx context.Context, channel string) (string, error) {
	name, _ := conversation.NormalizeChannelIdentifier(channel)
	if name == "" {
		return "", errors.New("channel must not be empty")
	}

	if id, ok := r.cache[name]; ok {
		return id, nil
	}

	// A Slack channel ID is usable directly. Names cannot start with C/G/D and be all-caps, so
	// trying conversations.info first is unambiguous and costs one call instead of paging the list.
	if looksLikeChannelID(name) {
		info, err := r.client.GetConversationInfoContext(ctx, &slack.GetConversationInfoInput{ChannelID: name})
		if err == nil && info != nil {
			r.cache[name] = info.ID
			return info.ID, nil
		}
	}

	var cursor string
	for range maxConversationPages {
		channels, next, err := r.client.GetConversationsContext(ctx, &slack.GetConversationsParameters{
			Cursor:          cursor,
			Limit:           200,
			ExcludeArchived: true,
			Types:           []string{"public_channel", "private_channel"},
		})
		if err != nil {
			return "", fmt.Errorf("while listing Slack conversations: %w", err)
		}

		for _, ch := range channels {
			// Cache every channel seen; a later lookup for a different configured channel is then free.
			if _, ok := r.cache[ch.Name]; !ok {
				r.cache[ch.Name] = ch.ID
			}
			if ch.Name == name || ch.ID == name {
				r.cache[name] = ch.ID
				return ch.ID, nil
			}
		}

		if next == "" {
			break
		}
		cursor = next
	}

	return "", fmt.Errorf("channel %q not found; make sure Botkube is invited to it", channel)
}

// looksLikeChannelID reports whether the value is shaped like a Slack conversation ID.
func looksLikeChannelID(in string) bool {
	if len(in) < 9 {
		return false
	}

	switch in[0] {
	case 'C', 'G', 'D':
	default:
		return false
	}

	// IDs are uppercase alphanumeric; channel names are lowercase, so any lowercase rules it out.
	return in == strings.ToUpper(in)
}
