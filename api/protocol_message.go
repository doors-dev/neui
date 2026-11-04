package api

// Role is the author of a message. Only user and assistant exist: the system
// prompt is request configuration, and a tool result is a content block that an
// adapter places in whichever message its provider expects.
type Role string

// The two roles.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of a conversation sent in a Request.
type Message struct {
	Role    Role     `json:"role"`    // who wrote the turn
	Content Contents `json:"content"` // blocks in order; tool results go in user messages
}
