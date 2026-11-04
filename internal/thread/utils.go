package thread

import (
	"neui.dev/api"
)

func filterTools(content api.Indexed[api.Content]) (tools []api.ContentToolUse) {
	for content := range content.Iter() {
		if tool, ok := content.(api.ContentToolUse); ok {
			tools = append(tools, tool)
		}
	}
	return
}
