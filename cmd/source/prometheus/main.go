package main

import (
	"github.com/hashicorp/go-plugin"

	"github.com/iggy/botkube/internal/source/prometheus"
	"github.com/iggy/botkube/pkg/api/source"
)

// version is set via ldflags by GoReleaser.
var version = "dev"

func main() {
	source.Serve(map[string]plugin.Plugin{
		prometheus.PluginName: &source.Plugin{
			Source: prometheus.NewSource(version),
		},
	})
}
