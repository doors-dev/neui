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
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/nekit/internal/chat"
	"neui.dev/nekit/internal/chat/css"
	"neui.dev/nekit/internal/chat/params"
)

// Chat is the ready-made chat widget: the chat list sidebar, the thread, the
// composer and its popovers. Render it on a page as ~nekit.Chat{}; the zero
// value is a complete chat. It fills its container, so give that container a
// height.
type Chat = chat.Chat

// ChatTheme overrides colours of the built-in palette for one face, dark or
// light; set it as Chat.ThemeDark or Chat.ThemeLight.
type ChatTheme = css.Theme

// ChatLayout sets where the chat sits and how big it is; set it as Chat.Layout.
// Lengths are CSS lengths such as "30rem" (px, rem, em, vw, vh or %); an empty
// or invalid one keeps the default. On screens under 40rem wide the open chat
// fills the screen.
type ChatLayout = css.Layout

// ChatSide is the page edge the chat sits at.
type ChatSide = css.Side

const (
	// ChatRight puts the chat at the right edge, the default.
	ChatRight = ChatSide(css.Right)
	// ChatLeft puts the chat at the left edge.
	ChatLeft = ChatSide(css.Left)
)

// ChatFeature is a part of the chat UI that can be hidden from the user.
type ChatFeature = params.Feature

const (
	// ChatTokens hides the context gauge, token counts and the Compact now button; a plain "compacting" chip still shows while compaction runs.
	ChatTokens = ChatFeature(params.Tokens)
	// ChatThinking hides reasoning and summary blocks and the Reasoning stat.
	ChatThinking = ChatFeature(params.Thinking)
	// ChatTools hides tool call blocks and pinned record counts.
	ChatTools = ChatFeature(params.Tools)
	// ChatTasks hides the background task chip and popover; the run chip shows "working" while only tasks run.
	ChatTasks = ChatFeature(params.Tasks)
	// ChatEdit hides the edit-and-resend button on user messages.
	ChatEdit = ChatFeature(params.Edit)
	// ChatRetry hides the retry button on failed rounds.
	ChatRetry = ChatFeature(params.Retry)
	// ChatAttachments hides the attach button and tray; large pastes stay in the draft as text.
	ChatAttachments = ChatFeature(params.Attachments)
	// // ChatFiles hides the files popover in the top bar.
	// ChatFiles = ChatFeature(params.Files)
	// ChatSidebar hides the chat list sidebar and its menu button.
	ChatSidebar = ChatFeature(params.Sidebar)
	// ChatThreadEdit hides rename, clone and delete in the chat list.
	ChatThreadEdit = ChatFeature(params.ThreadEdit)
	// ChatMessageSearch limits search to chat names; message contents are not searched.
	ChatMessageSearch = ChatFeature(params.MessageSearch)
	// ChatGreeting hides the greeting a new chat shows before its first message.
	ChatGreeting = ChatFeature(params.Greeting)
	// ChatDock hides the dock button, so the chat only floats.
	ChatDock = ChatFeature(params.Dock)
)

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
