// Package params carries nechat settings through the widget's components.
// Hosts use it only through nechat.Feature.
package params

import (
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/nechat/css"
)

// Params are the widget settings shared by nechat's components. The exported
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
	ID                string
	off               uint32
}

// Feature is a part of the chat UI that can be hidden; hosts use the
// constants nechat re-exports, such as nechat.Tokens, in Chat.Disable.
type Feature uint8

// The features nechat can hide; each is documented on its nechat constant.
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

// InitDefault fills zero attachment limits with their defaults and assigns a fresh ID.
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
	p.ID = doors.IDRand()
}
