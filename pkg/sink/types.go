package sink

import (
	"github.com/iggy/botkube/pkg/notifier"
)

// Sink sends messages to communication channels. It is a one-way integration.
type Sink interface {
	notifier.Sink
}
