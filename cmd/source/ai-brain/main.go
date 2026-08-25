package main

import (
	_ "embed"

	"github.com/hashicorp/go-plugin"

	aibrain "github.com/iggy/botkube/internal/source/ai-brain"
	"github.com/iggy/botkube/pkg/api/source"
)

// version is set via ldflags by GoReleaser.
var version = "dev"

func main() {
	source.Serve(map[string]plugin.Plugin{
		aibrain.PluginName: &source.Plugin{
			Source: aibrain.NewSource(version),
		},
	})
}
