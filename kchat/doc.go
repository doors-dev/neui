// Package kchat is the ready-made chat widget for neui. Render it on a doors
// page:
//
//	~kchat.Kchat{Disable: kchat.Simple}
//
// It shows a disabled chat until neui.Enable succeeds for the page's session,
// and again after neui.Disable. Kchat.Disable hides parts of the UI, with
// Simple as the preset for non-technical users, and ThemeDark and ThemeLight
// recolour it. ModuleAsk adds a tool that lets the agent ask the user to pick
// from a list inside the chat.
package kchat
