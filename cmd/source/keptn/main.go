package main

import (
	"github.com/hashicorp/go-plugin"

	"github.com/iggy/botkube/internal/source/keptn"
	"github.com/iggy/botkube/pkg/api/source"
)

// version is set via ldflags by GoReleaser.
var version = "dev"

func main() {
	source.Serve(map[string]plugin.Plugin{
		keptn.PluginName: &source.Plugin{
			Source: keptn.NewSource(version),
		},
	})
}
