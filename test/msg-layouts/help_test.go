package msg_layouts

import (
	"encoding/json"
	"path/filepath"
	"testing"

	"github.com/iggy/botkube/pkg/bot"
	"github.com/iggy/botkube/pkg/bot/interactive"
	"github.com/iggy/botkube/pkg/config"
	"github.com/slack-go/slack"
	"github.com/stretchr/testify/require"
	"gotest.tools/v3/golden"
)

// TestNewHelpMessage generates help message directly in Discord and Mattermost format.
// The output is stored in 'testdata/TestNewHelpMessage/' folder. You can just copy-paste it into dedicated editors to see the message layout:
//   - Discord: it's only markdown, just post as a normal message in discord channel
//   - Mattermost: it's only markdown, just post as a normal message in mattermost channel
//
// To update the golden files:
//
//	go test -v -run TestNewHelpMessage -update
func TestNewHelpMessage(t *testing.T) {
	msg := interactive.NewHelpMessage(config.DiscordCommPlatformIntegration, "Stage US", []string{"botkube/kubectl"}).Build(false)
	msg.ReplaceBotNamePlaceholder("@Botkube")

	// discord - we have only markdown formatter
	md := bot.NewDiscordRenderer().MessageToMarkdown(msg)
	golden.Assert(t, md, filepath.Join(t.Name(), "discord-help.golden.md"))

	// mattermost - we have only markdown formatter
	md = bot.NewMattermostRenderer().MessageToMarkdown(msg)
	golden.Assert(t, md, filepath.Join(t.Name(), "mattermost-help.golden.md"))
}

// TestNewHelpMessageSlack generates a Slack help message and saves it as a golden file.
// You can paste the output into https://app.slack.com/block-kit-builder/ to see the layout.
//
// To update the golden files:
//
//	go test -v -run TestNewHelpMessageSlack -update
func TestNewHelpMessageSlack(t *testing.T) {
	msg := interactive.NewHelpMessage(config.SocketSlackCommPlatformIntegration, "Stage US", []string{"botkube/kubectl"}).Build(false)
	msg.ReplaceBotNamePlaceholder("@Botkube")

	blocks := bot.NewSlackRenderer().RenderAsSlackBlocks(msg)
	assertJSONGoldenFiles(t, SlackBuiltKit{Blocks: blocks}, "slack-help.golden.json")
}

type SlackBuiltKit struct {
	Blocks []slack.Block `json:"blocks"`
}

func assertJSONGoldenFiles(t *testing.T, in any, goldenFile string) {
	t.Helper()

	raw, err := json.MarshalIndent(in, "", "  ")
	require.NoError(t, err)
	golden.Assert(t, string(raw), filepath.Join(t.Name(), goldenFile))
}
