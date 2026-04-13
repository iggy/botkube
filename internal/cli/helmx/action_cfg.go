package helmx

import (
	"fmt"

	"helm.sh/helm/v4/pkg/action"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	"github.com/kubeshop/botkube/internal/cli"
	"github.com/kubeshop/botkube/internal/kubex"
	"github.com/kubeshop/botkube/pkg/ptr"
)

const helmDriver = "secrets"

// GetActionConfiguration returns generic configuration for Helm actions.
func GetActionConfiguration(k8sCfg *kubex.ConfigWithMeta, forNamespace string) (*action.Configuration, error) {
	actionConfig := new(action.Configuration)
	helmCfg := &genericclioptions.ConfigFlags{
		APIServer:   &k8sCfg.K8s.Host,
		Insecure:    &k8sCfg.K8s.Insecure,
		CAFile:      &k8sCfg.K8s.CAFile,
		BearerToken: &k8sCfg.K8s.BearerToken,
		Context:     &k8sCfg.CurrentContext,
		Namespace:   ptr.FromType(forNamespace),
	}

	// Note: Helm v4 removed the debug logger parameter from Init.
	// Debug logging must now be handled separately if needed.
	err := actionConfig.Init(helmCfg, forNamespace, helmDriver)
	if err != nil {
		return nil, fmt.Errorf("while initializing Helm configuration: %v", err)
	}

	if cli.VerboseMode.IsTracing() {
		fmt.Printf("    Helm configuration initialized for namespace: %s\n", forNamespace)
	}

	return actionConfig, nil
}
