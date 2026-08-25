package config

import (
	"os"

	"github.com/iggy/botkube/pkg/config"
)

// GetProvider resolves and returns paths for config files.
// It reads them the 'BOTKUBE_CONFIG_PATHS' env variable. If not found, then it uses '--config' flag.
func GetProvider() config.Provider {
	if os.Getenv(EnvProviderConfigPathsEnvKey) != "" {
		return NewEnvProvider()
	}

	return NewFileSystemProvider(configPathsFlag)
}
