package lib

import (
	"fmt"
	"log/slog"

	"neui.dev/api"
)

// Omitted is the note that takes the place of a content block an adapter cannot
// send, and the log line that says one was dropped.
//
// A block that simply vanishes leaves the model a turn that reads as if it had
// never been there, and a request that fails over one leaves the caller an error
// they cannot act on: what an adapter cannot send is the thread's rather than
// the caller's — another model's reasoning, a payload only the adapter that
// issued it can read, a type a later version of this protocol added. So the turn
// goes out with the shape it had and a note where the block was, and the drop is
// reported to whoever runs the process instead.
//
// The note names the kind and nothing else. The payload is gone and no wording
// brings it back, so all there is to say is that something was there.
//
// why is what the adapter cannot do with the block, for the log alone. The model
// is never told an adapter's reason for a wire it knows nothing about.
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
