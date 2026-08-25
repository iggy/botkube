package bot

import (
	"context"

	"github.com/iggy/botkube/internal/health"
	"github.com/iggy/botkube/pkg/config"
	"github.com/iggy/botkube/pkg/execute"
	"github.com/iggy/botkube/pkg/notifier"
)

const (
	platformMessageWorkersCount = 10
	platformMessageChannelSize  = 100
)

// Bot connects to communication channels and reads/sends messages. It is a two-way integration.
type Bot interface {
	Start(ctx context.Context) error
	GetStatus() health.PlatformStatus
	notifier.Bot
}

type Status struct {
	Status   health.PlatformStatusMsg
	Restarts string
	Reason   health.FailureReasonMsg
}

// ExecutorFactory facilitates creation of execute.Executor instances.
type ExecutorFactory interface {
	NewDefault(cfg execute.NewDefaultInput) execute.Executor
}

type channelConfigByID struct {
	config.ChannelBindingsByID

	alias  string
	notify bool
	name   string
}

type channelConfigByName struct {
	config.ChannelBindingsByName

	alias  string
	notify bool
}

type CommGroupMetadata struct {
	Name  string
	Index int
}

func AsNotifiers(bots map[string]Bot) []notifier.Bot {
	notifiers := make([]notifier.Bot, 0, len(bots))
	for _, bot := range bots {
		notifiers = append(notifiers, bot)
	}
	return notifiers
}
