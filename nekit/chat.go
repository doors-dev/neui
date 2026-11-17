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
	// ChatFiles hides the files popover in the top bar.
	ChatFiles = ChatFeature(params.Files)
	// ChatSidebar hides the chat list sidebar and its menu button.
	ChatSidebar = ChatFeature(params.Sidebar)
	// ChatThreadEdit hides rename, clone and delete in the chat list.
	ChatThreadEdit = ChatFeature(params.ThreadEdit)
	// ChatMessageSearch limits search to chat names; message contents are not searched.
	ChatMessageSearch = ChatFeature(params.MessageSearch)
	// ChatGreeting hides the greeting a new chat shows before its first message.
	ChatGreeting = ChatFeature(params.Greeting)
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
