package migrate

import (
	"time"

	"github.com/iggy/botkube/internal/cli/config"
)

// Options holds migrate possible configuration options.
type Options struct {
	Timeout           time.Duration
	Watch             bool
	Token             string
	InstanceName      string `survey:"instanceName"`
	CloudDashboardURL string
	CloudAPIURL       string
	ImageTag          string
	SkipConnect       bool
	SkipOpenBrowser   bool
	AutoApprove       bool
	ConfigExporter    config.ExporterOptions
}
