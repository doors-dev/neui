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

package nekit

import (
	"time"

	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/nekit/internal/chat"
	"neui.dev/nekit/internal/chat/css"
	"neui.dev/nekit/internal/chat/feature"
)

// Chat is the ready-made chat widget: the chat list sidebar, the thread, the
// composer and its popovers. Render it on a page as ~nekit.Chat{}; the zero
// value is a complete chat. It fills its container, so give that container a
// height.
type Chat struct {
	// MaxAttachmentSize is the largest file, in bytes, a user may attach. Zero means 12 MiB.
	MaxAttachmentSize int
	// MaxAttachments is how many files one draft may hold. Zero means 10.
	MaxAttachments int
	// AttachmentTimeout bounds one file upload. Zero means 10 minutes.
	AttachmentTimeout time.Duration
	// AsideRight puts the chat list sidebar on the right instead of the left.
	AsideRight bool
	// Framed draws a rounded border around the widget.
	Framed bool
	// ThemeDark recolours the dark face, shown where the page's color-scheme
	// is dark. The zero value keeps the built-in palette.
	ThemeDark ChatTheme
	// ThemeLight recolours the light face, shown where the page's color-scheme
	// is light or not set. The zero value keeps the built-in palette.
	ThemeLight ChatTheme
	// Layout sets the chat's side, sizes and the mode it opens as; the zero
	// value keeps the defaults.
	Layout ChatLayout
	// GreetingTitle is the heading a new chat shows before its first message:
	// a string or a GoX element. nil shows "How can I help?"; "" leaves it out.
	GreetingTitle any
	// GreetingBody is what shows under GreetingTitle: a string or a GoX element,
	// such as the host's own template. nil shows "Ask a question or describe
	// what you need."; "" leaves it out.
	GreetingBody any
	// Disable lists what is turned off: parts of the UI and modes of the open
	// chat; nil keeps everything. ChatSimple is the preset for non-technical
	// users and grows with technical features added later.
	Disable []ChatFeature
}

// Main renders the widget for the doors instance (browser tab) being
// rendered. It shows a disabled chat until neui.Enable succeeds for the
// session, and again after neui.Disable.
func (c Chat) Main() gox.Elem {
	disable := make([]feature.Feature, 0, len(c.Disable))
	for _, f := range c.Disable {
		if f != nil {
			disable = append(disable, f.chatFeature())
		}
	}
	return chat.Chat{
		MaxAttachmentSize: c.MaxAttachmentSize,
		MaxAttachments:    c.MaxAttachments,
		AttachmentTimeout: c.AttachmentTimeout,
		AsideRight:        c.AsideRight,
		Framed:            c.Framed,
		ThemeDark:         css.Theme(c.ThemeDark),
		ThemeLight:        css.Theme(c.ThemeLight),
		Layout: css.Layout{
			Side:      css.Side(c.Layout.Side),
			Width:     c.Layout.Width,
			Height:    c.Layout.Height,
			DockWidth: c.Layout.DockWidth,
			Start:     feature.Mode(c.Layout.Start),
		},
		GreetingTitle: c.GreetingTitle,
		GreetingBody:  c.GreetingBody,
		Disable:       disable,
	}.Main()
}

// ChatTheme overrides colours of one face of the chat palette, dark or light;
// set it as Chat.ThemeDark or Chat.ThemeLight. Each field takes any CSS colour
// value, such as "#18181b" or "var(--my-bg)"; an empty field keeps the
// default, and a value containing any of {};< is ignored.
type ChatTheme struct {
	// AppBg is the widget background.
	AppBg string
	// AppBorder is the colour of borders and dividers; hover backgrounds derive from it.
	AppBorder string
	// AppInk is the main text colour.
	AppInk string
	// AppInkQuiet is the colour of secondary text, form accents and focus rings.
	AppInkQuiet string
	// PanelBg is the background of the composer, popovers, form fields and message bubbles.
	PanelBg string
	// PanelBorderFocus is the border of input fields, the composer, checkboxes and radio buttons,
	// hovered and pressed buttons, and focused surfaces.
	PanelBorderFocus string
	// ButtonBorder is the border of buttons and the background of pressed buttons and controls.
	ButtonBorder string
	// ButtonInk is the text and icon colour of buttons.
	ButtonInk string
	// ButtonInkOff is the text and icon colour of disabled buttons.
	ButtonInkOff string
	// SecondaryBg is the background of secondary buttons, the context gauge and attached files.
	SecondaryBg string
	// DangerInk is the text colour of destructive actions and errors.
	DangerInk string
	// DangerBgHover is the fill of a hovered destructive button.
	DangerBgHover string
	// DangerBgActive is the fill of a pressed destructive button.
	DangerBgActive string
	// DangerInkOnFill is the text colour on a danger fill.
	DangerInkOnFill string
	// LauncherBg is the fill of the round button that opens a collapsed chat; it defaults to AppInk.
	LauncherBg string
	// LauncherInk is the icon colour of the round button that opens a collapsed chat; it defaults to AppBg.
	LauncherInk string
}

// ChatLayout sets where the chat sits, how big it is and the mode it opens as;
// set it as Chat.Layout. Lengths are CSS lengths such as "30rem" (px, rem, em,
// vw, vh or %); an empty or invalid one keeps the default. On screens under
// 40rem wide the open chat fills the screen.
type ChatLayout struct {
	// Side is the page edge the chat sits at; the zero value is ChatRight.
	Side ChatSide
	// Width is the floating window's width; default 25rem.
	Width string
	// Height is the floating window's height; default 40rem.
	Height string
	// DockWidth is the docked panel's width, which the page makes room for; default 28rem.
	DockWidth string
	// Start is the mode the chat opens as for a viewer with no remembered
	// layout; the zero value is ChatFloat. The chat itself starts collapsed.
	Start ChatMode
}

// ChatSide is the page edge the chat sits at.
type ChatSide uint8

const (
	// ChatRight puts the chat at the right edge, the default.
	ChatRight = ChatSide(css.Right)
	// ChatLeft puts the chat at the left edge.
	ChatLeft = ChatSide(css.Left)
)

// ChatFeature is something Chat.Disable turns off: a ChatPart of the UI or a
// ChatMode of the open chat.
type ChatFeature interface {
	chatFeature() feature.Feature
}

// ChatPart is a part of the chat UI that can be hidden from the user.
type ChatPart uint8

const (
	// ChatTokens hides the context gauge, token counts and the Compact now button; a plain "compacting" chip still shows while compaction runs.
	ChatTokens = ChatPart(feature.Tokens)
	// ChatThinking hides reasoning and summary blocks and the Reasoning stat.
	ChatThinking = ChatPart(feature.Thinking)
	// ChatTools hides tool call blocks and pinned record counts.
	ChatTools = ChatPart(feature.Tools)
	// ChatTasks hides the background task chip and popover; the run chip shows "working" while only tasks run.
	ChatTasks = ChatPart(feature.Tasks)
	// ChatEdit hides the edit-and-resend button on user messages.
	ChatEdit = ChatPart(feature.Edit)
	// ChatRetry hides the retry button on failed rounds.
	ChatRetry = ChatPart(feature.Retry)
	// ChatAttachments hides the attach button and tray; large pastes stay in the draft as text.
	ChatAttachments = ChatPart(feature.Attachments)
	// ChatSidebar hides the chat list sidebar and its menu button.
	ChatSidebar = ChatPart(feature.Sidebar)
	// ChatThreadEdit hides rename, clone and delete in the chat list.
	ChatThreadEdit = ChatPart(feature.ThreadEdit)
	// ChatMessageSearch limits search to chat names; message contents are not searched.
	ChatMessageSearch = ChatPart(feature.MessageSearch)
	// ChatGreeting hides the greeting a new chat shows before its first message.
	ChatGreeting = ChatPart(feature.Greeting)
	// // ChatFiles hides the files popover in the top bar.
	// ChatFiles = ChatPart(feature.Files)
)

func (p ChatPart) chatFeature() feature.Feature {
	return feature.Part(p)
}

// ChatMode is how the open chat sits on the page. ChatLayout.Start sets the
// one it opens as; turning one off in Chat.Disable keeps the chat in the other
// and hides the button that switches between them. Turning both off keeps
// ChatFloat.
type ChatMode uint8

const (
	// ChatFloat is the floating window, the default.
	ChatFloat = ChatMode(feature.Float)
	// ChatDock is the panel docked to the page edge, which the page makes room for.
	ChatDock = ChatMode(feature.Dock)
)

func (m ChatMode) chatFeature() feature.Feature {
	return feature.Mode(m)
}

// ChatSimple is the preset for non-technical users. It grows as technical features are added.
var ChatSimple = []ChatFeature{ChatTokens, ChatThinking, ChatTools, ChatTasks}

// AskOption is one answer the user can pick in an AskOptions question.
type AskOption = chat.AskOption

// AskInput is what the agent sends to the ask tool: a question and the
// answers to offer.
type AskInput = chat.AskInput

// AskOutput is the user's answer as the agent receives it. At least one of
// Picked and Typed is set.
type AskOutput = chat.AskOutput

// ModuleAsk is the built-in module that lets the agent ask the user to pick
// from a list, rendered in the chat by Chat. Register it with
// neui.WithModules or neui.OptionModules:
//
//	neui.Enable(ctx, neui.OptionModules(nekit.ModuleAsk()))
func ModuleAsk() common.ModuleFactory {
	return chat.ModuleAsk()
}

// AskOptions is the Ask handler behind ModuleAsk. It builds the form and send
// button for input. When the user sends with Enter or the button, it submits
// an AskOutput to answer, but only if at least one option is picked or own
// text is selected, and the own text is within 256 KiB. Use it to add the
// same question to a module of your own with neui.Module.Ask.
func AskOptions(input AskInput, answer common.Answer[AskOutput]) api.ChatAsk {
	return chat.AskOptions(input, answer)
}
