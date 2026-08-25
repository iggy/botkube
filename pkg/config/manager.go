package config

import (
	"context"
	"errors"

	"github.com/sirupsen/logrus"
	"k8s.io/client-go/kubernetes"
)

var (
	supportedPlatformsSourceBindings = map[CommPlatformIntegration]struct{}{
		SocketSlackCommPlatformIntegration: {},
		DiscordCommPlatformIntegration:     {},
		MattermostCommPlatformIntegration:  {},
	}
	supportedPlatformsNotifications = map[CommPlatformIntegration]struct{}{
		SocketSlackCommPlatformIntegration: {},
		DiscordCommPlatformIntegration:     {},
		MattermostCommPlatformIntegration:  {},
	}
)

// PersistenceManager manages persistence of the configuration.
type PersistenceManager interface {
	PersistSourceBindings(ctx context.Context, commGroupName string, platform CommPlatformIntegration, channelAlias string, sourceBindings []string) error
	PersistNotificationsEnabled(ctx context.Context, commGroupName string, platform CommPlatformIntegration, channelAlias string, enabled bool) error
	PersistActionEnabled(ctx context.Context, name string, enabled bool) error
}

// ErrUnsupportedPlatform is an error returned when a platform is not supported.
var ErrUnsupportedPlatform = errors.New("unsupported platform to persist data")

// NewManager creates a new PersistenceManager instance.
func NewManager(log logrus.FieldLogger, cfg PersistentConfig, k8sCli kubernetes.Interface) PersistenceManager {
	return &K8sConfigPersistenceManager{
		log:    log,
		cfg:    cfg,
		k8sCli: k8sCli,
	}
}
