package uninstall

import "github.com/iggy/botkube/internal/cli/uninstall/helm"

// Config holds parameters for Botkube deletion.
type Config struct {
	HelmParams  helm.Config
	AutoApprove bool
}
