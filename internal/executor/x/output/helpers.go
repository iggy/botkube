package output

import "github.com/iggy/botkube/pkg/api"

func noItemsMsg() api.Message {
	return api.Message{
		Sections: []api.Section{
			{
				Base: api.Base{
					Description: "Not found.",
				},
			},
		},
	}
}
