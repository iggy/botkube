package main

import (
	"github.com/hashicorp/go-plugin"

	"github.com/iggy/botkube/internal/executor/kubectl"
	"github.com/iggy/botkube/pkg/api/executor"
)

// version is set via ldflags by GoReleaser.
var version = "dev"

func main() {
	executor.Serve(map[string]plugin.Plugin{
		kubectl.PluginName: &executor.Plugin{
			Executor: kubectl.NewExecutor(version, kubectl.NewBinaryRunner()),
		},
	})
}
