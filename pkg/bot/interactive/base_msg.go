package interactive

import (
	"github.com/iggy/botkube/pkg/api"
)

// CoreMessage holds Botkube internal message model. It's useful to add Botkube specific header or description to plugin messages.
type CoreMessage struct {
	Header      string
	Description string
	Metadata    any
	Messages    []api.Message
	api.Message
}
