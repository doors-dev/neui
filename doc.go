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
// The agent acts through tools. A Module built with NewModule, or one the
// package ships such as ModuleWeb or ModuleXLS, serves tools to the threads
// of the sessions it is registered for. On the page, Space, Tool, Data and Use
// expose regions, actions, context and elements of what the user sees, and
// Island gives the agent a region where it runs its own UI.
package neui
