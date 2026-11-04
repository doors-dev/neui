// Package neui is an agent harness for Doors apps: it runs a chat agent
// inside the user's session that can see and operate the page.
//
// Configure the app once with options for doors.NewApp, such as WithProvider,
// WithModules and WithThreadConfig, and turn the agent on per session with
// Enable(ctx, opts...), typically from an auth hook or handler once the user
// is known. Render a chat UI, such as kchat.Kchat, where the user should talk
// to the agent.
//
// The agent acts through tools. A Module built with NewModule, or a built-in
// one such as ModuleJS or ModuleWeb, serves tools to every thread. On the
// page, Space, Tool, Data and Input expose regions, actions, context and
// elements of what the user sees, and Island gives the agent a region where
// it runs its own UI.
package neui
