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
	"fmt"

	"neui.dev/api"
	"neui.dev/internal/response"
)

type consumer[T any] = func([]api.Content) (T, []api.Content, error)

type thread[T any] struct {
	provider api.Provider
	usage    *api.EventUsage
	consumer consumer[T]
	system   string
	replay   []api.Message
	tools    []api.Tool
}

func (t *thread[T]) run(ctx context.Context) (T, error) {
	b := response.NewBuilder()
	if t.usage != nil {
		defer func() { *t.usage = b.Spent() }()
	}
	for {
		turn, cancel := context.WithCancel(ctx)
		ch := t.provider.Stream(turn, api.Request{
			System:   t.system,
			Messages: t.replay,
			Tools:    t.tools,
		})
		err := request(turn, ch, b)
		cancel()
		if err != nil {
			var t T
			return t, err
		}
		response := b.Flush()
		b.Discard()
		if code, isErr := response.Err(); isErr {
			var t T
			if response.Cause != nil {
				return t, fmt.Errorf("%w: %w", code, response.Cause)
			}
			return t, code
		}
		content := response.Content.Ordered()
		result, round, err := t.consumer(content)
		if err != nil {
			var t T
			return t, err
		}
		if len(round) != 0 {
			t.replay = append(t.replay, api.Message{
				Role:    api.RoleAssistant,
				Content: content,
			}, api.Message{
				Role:    api.RoleUser,
				Content: round,
			})
			continue
		}
		return result, nil
	}
}

func request(ctx context.Context, ch <-chan api.Event, b response.Builder) error {
	for {
		select {
		case <-ctx.Done():
			return context.Cause(ctx)
		case e, more := <-ch:
			if !more {
				return context.Cause(ctx)
			}
			_, more = b.On(e)
			if !more {
				return nil
			}
		}
	}
}
