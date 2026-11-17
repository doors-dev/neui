// Package neui is an agent harness for Doors apps: it runs a chat agent
// inside the user's session that can see and operate the page.
//
// Configure the app with optional app options for doors.NewApp, such as
// WithProvider, WithModules and WithThreadConfig, and turn the agent on per
// session with Enable(ctx, opts...), typically from an auth hook or handler
// once the user is known; its Option values add to or take precedence over
// the app options. A session needs a provider from WithProvider or
// OptionProvider. Render a chat UI, such as nekit.Chat, where the user
// should talk to the agent.
//
// The agent acts through tools. A Module built with NewModule, or a built-in
// one such as ModuleJS or ModuleWeb, serves tools to the threads of the
// sessions it is registered for. On the page, Space, Tool, Data and Input
// expose regions, actions, context and elements of what the user sees, and
// Island gives the agent a region where it runs its own UI.
package neui
