package lib

import (
	"fmt"
	"log/slog"

	"neui.dev/api"
)

// Omitted logs a warning that an adapter dropped content c, which this wire
// cannot carry, and returns the placeholder text to send in its place, such as
// "[image omitted]". Use it for content the thread holds but this provider
// cannot take (another model's reasoning, a foreign opaque payload, an unknown
// type), so the turn keeps its shape instead of failing. message is the index
// of the message in the request, and why goes to the log only, never to the
// model.
func Omitted(provider string, message int, c api.Content, why string) string {
	slog.Default().Warn("content omitted",
		"provider", provider,
		"message", message,
		"content", fmt.Sprintf("%T", c),
		"reason", why)
	switch c.(type) {
	case api.ContentImage:
		return "[image omitted]"
	case api.ContentDocument:
		return "[document omitted]"
	case api.ContentReasoning:
		return "[reasoning omitted]"
	}
	return "[content omitted]"
}
