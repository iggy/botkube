package interactive

import (
	"fmt"

	"github.com/iggy/botkube/pkg/api"
	"github.com/iggy/botkube/pkg/config"
)

type pluginHelpProviderFn func(platform config.CommPlatformIntegration, btnBuilder *api.ButtonBuilder) api.Section

var pluginHelpProvider = map[string]pluginHelpProviderFn{
	"botkube/kubectl": func(platform config.CommPlatformIntegration, btnBuilder *api.ButtonBuilder) api.Section {
		if platform.IsInteractive() && platform != config.CloudTeamsCommPlatformIntegration {
			return api.Section{
				Base: api.Base{
					Header:      "🔮Run kubectl commands",
					Description: fmt.Sprintf("`%s kubectl [command] [TYPE] [NAME] [flags]` - run any of the supported kubectl commands directly from %s", api.MessageBotNamePlaceholder, platform.DisplayName()),
				},
				Buttons: []api.Button{
					btnBuilder.ForCommandWithoutDesc("Open the kubectl composer", "kubectl", api.ButtonStylePrimary),
					btnBuilder.ForCommandWithoutDesc("kubectl help", "View help"),
				},
			}
		}

		// without the kubectl command builder
		return api.Section{
			Buttons: []api.Button{
				btnBuilder.ForCommandWithBoldDesc("kubectl help", "Run kubectl commands (if enabled)", "kubectl help"),
			},
		}
	},
}
