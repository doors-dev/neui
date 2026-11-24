// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package params carries nekit chat settings through the widget's components.
package params

import (
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/nekit/internal/chat/css"
	"neui.dev/nekit/internal/chat/feature"
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
	Layout            css.Layout
	GreetingTitle     any
	GreetingBody      any
	ID                string
	off               feature.Set
}

type Feature = feature.Feature

const (
	Tokens        = feature.Tokens
	Thinking      = feature.Thinking
	Tools         = feature.Tools
	Tasks         = feature.Tasks
	Edit          = feature.Edit
	Retry         = feature.Retry
	Attachments   = feature.Attachments
	Files         = feature.Files
	Sidebar       = feature.Sidebar
	ThreadEdit    = feature.ThreadEdit
	MessageSearch = feature.MessageSearch
	Greeting      = feature.Greeting
	Float         = feature.Float
	Dock          = feature.Dock
)

func (p *Params) Disable(features []Feature) {
	p.off |= feature.NewSet(features)
}

func (p Params) Off(f Feature) bool {
	return p.off.Has(f)
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
