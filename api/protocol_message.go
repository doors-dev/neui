package api

// Role is the author of a message. Only user and assistant exist: the system
// prompt is request configuration, and a tool result is a content block that an
// adapter places in whichever message its provider expects.
type Role string

const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

type Message struct {
	Role    Role     `json:"role"`
	Content Contents `json:"content"`
}
