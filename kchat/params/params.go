package params

import (
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/kchat/css"
)

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

type Feature uint8

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
)

func (p *Params) Disable(features []Feature) {
	for _, f := range features {
		p.off |= 1 << f
	}
}

func (p Params) Off(f Feature) bool {
	return p.off&(1<<f) != 0
}

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
