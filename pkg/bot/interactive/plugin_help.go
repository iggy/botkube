package interactive

import (
	"fmt"

	"github.com/iggy/botkube/pkg/api"
	"github.com/iggy/botkube/pkg/config"
)

type pluginHelpProviderFn func(platform config.CommPlatformIntegration, btnBuilder *api.ButtonBuilder) api.Section

var pluginHelpProvider = map[string]pluginHelpProviderFn{
	"botkube/ai": func(_ config.CommPlatformIntegration, btnBuilder *api.ButtonBuilder) api.Section {
		return api.Section{
			Base: api.Base{
				Header:      "🤖 AI powered Kubernetes assistant",
				Description: fmt.Sprintf("`%s ai` use natural language to ask any questions\n`%s ai scan` perform a cluster-wide scan for issues", api.MessageBotNamePlaceholder, api.MessageBotNamePlaceholder),
			},
			Buttons: []api.Button{
				btnBuilder.ForCommandWithoutDesc("Cluster Scan", "ai scan", api.ButtonStylePrimary),
			},
		}
	},
	"botkube/kubectl": func(platform config.CommPlatformIntegration, btnBuilder *api.ButtonBuilder) api.Section {
		if platform.IsInteractive() {
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
