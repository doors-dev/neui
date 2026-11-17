// Package params carries nekit chat settings through the widget's components.
// Hosts use it only through nekit.ChatFeature.
package params

import (
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/nekit/internal/chat/css"
)

// Params are the widget settings shared by the nekit chat components. The exported
// fields mirror the Chat fields of the same name; ID is a random per-widget
// id set by InitDefault.
type Params struct {
	MaxAttachmentSize int
	MaxAttachments    int
	AttachmentTimeout time.Duration
	AsideRight        bool
	Framed            bool
	ThemeDark         css.Theme
	ThemeLight        css.Theme
	GreetingTitle     any
	GreetingBody      any
	ID                string
	off               uint32
}

// Feature is a part of the chat UI that can be hidden; hosts use the
// constants nekit re-exports, such as nekit.ChatTokens, in Chat.Disable.
type Feature uint8

// The features the nekit chat can hide; each is documented on its nekit constant.
const (
	Tokens Feature = iota + 1
	Thinking
	Tools
	Tasks
	Edit
	Retry
	Attachments
	Files
	Sidebar
	ThreadEdit
	MessageSearch
	Greeting
)

// Disable hides features.
func (p *Params) Disable(features []Feature) {
	for _, f := range features {
		p.off |= 1 << f
	}
}

// Off reports whether f is hidden.
func (p Params) Off(f Feature) bool {
	return p.off&(1<<f) != 0
}

// InitDefault fills zero attachment limits and nil greeting parts with their defaults and assigns a fresh ID.
func (p *Params) InitDefault() {
	if p.MaxAttachmentSize == 0 {
		p.MaxAttachmentSize = 12 << 20
	}
	if p.MaxAttachments == 0 {
		p.MaxAttachments = 10
	}
	if p.AttachmentTimeout == 0 {
		p.AttachmentTimeout = 10 * time.Minute
	}
	if p.GreetingTitle == nil {
		p.GreetingTitle = "How can I help?"
	}
	if p.GreetingBody == nil {
		p.GreetingBody = "Ask a question or describe what you need."
	}
	p.ID = doors.IDRand()
}
