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

package generate

import (
	"context"

	"neui.dev/api"
	"neui.dev/internal/common"
)

// Generate calls a model from the host's own code, outside any chat.
type Generate struct {
	// Provider is the provider to call; nil uses the app's WithProvider from
	// ctx.
	Provider api.Provider
	// System is the system prompt.
	System string
	// Usage, when set, receives the tokens a call spent, retries included. It
	// is overwritten, not added to, so calls running at once need their own.
	Usage *api.EventUsage
	// Attempts is how many replies Object tries before giving up on output
	// that does not match T; zero or less means 3.
	Attempts int
}

// Object sends parts as one user message and returns the reply decoded into
// T. A part goes as it is when it is api content, as text when it is a string
// or fmt.Stringer, and as JSON otherwise.
//
// T's schema comes from its type, with the same tags as a tool's input; a
// field is required only when tagged required:"true". The model must support
// tool calls. Output that still does not match T after Attempts returns an
// error wrapping api.ErrInvalidOutput.
func (g Generate) Object[T any](ctx context.Context, parts ...any) (T, error) {
	provider := g.Provider
	if provider == nil {
		provider = common.AppProvider(ctx)
	}
	if provider == nil {
		var zero T
		return zero, api.ErrNoProvider
	}
	user, err := content(parts)
	if err != nil {
		var zero T
		return zero, err
	}
	user = append(user, api.ContentText{Text: resultInstruction})
	attempts := g.Attempts
	if attempts <= 0 {
		attempts = 3
	}
	tool, consumer, err := newObjectConsumer[T](attempts, provider.Model().SimpleSchemas)
	if err != nil {
		var zero T
		return zero, err
	}
	t := &thread[T]{
		provider: provider,
		usage:    g.Usage,
		consumer: consumer,
		system:   g.System,
		replay:   []api.Message{{Role: api.RoleUser, Content: user}},
		tools:    []api.Tool{tool},
	}
	return t.run(ctx)
}

// Text sends parts as one user message, as Object does, and returns the text
// of the reply without surrounding whitespace.
func (g Generate) Text(ctx context.Context, parts ...any) (string, error) {
	provider := g.Provider
	if provider == nil {
		provider = common.AppProvider(ctx)
	}
	if provider == nil {
		return "", api.ErrNoProvider
	}
	user, err := content(parts)
	if err != nil {
		return "", err
	}
	t := &thread[string]{
		provider: provider,
		usage:    g.Usage,
		consumer: textConsumer,
		system:   g.System,
		replay:   []api.Message{{Role: api.RoleUser, Content: user}},
	}
	return t.run(ctx)
}
