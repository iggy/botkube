package main

import (
	"context"
	"time"

	"github.com/allegro/bigcache/v3"
	"github.com/hashicorp/go-plugin"

	"github.com/iggy/botkube/internal/executor/flux"
	"github.com/iggy/botkube/pkg/api/executor"
	"github.com/iggy/botkube/pkg/loggerx"
)

// version is set via ldflags by GoReleaser.
var version = "dev"

func main() {
	cache, err := bigcache.New(context.Background(), bigcache.DefaultConfig(30*time.Minute))
	loggerx.ExitOnError(err, "while creating big cache")

	executor.Serve(map[string]plugin.Plugin{
		flux.PluginName: &executor.Plugin{
			Executor: flux.NewExecutor(cache, version),
		},
	})
}
