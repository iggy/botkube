package notifier

import (
	"github.com/iggy/botkube/internal/health"
	"github.com/iggy/botkube/pkg/config"
)

// Platform represents platform notifier
type Platform interface {
	GetStatus() health.PlatformStatus
	IntegrationName() config.CommPlatformIntegration
}
