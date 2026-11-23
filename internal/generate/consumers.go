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
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/schema"
	"neui.dev/internal/utils"
)

const resultTool = "result"

var resultInstruction = utils.SystemInstruction("Respond by calling the result tool; text outside the call is not read.")

var resultReminder = utils.SystemInstruction("You did not call the result tool. Call it now.")

var errNoResultCall = errors.New("generate: the model answered without calling the result tool")

func newObjectConsumer[T any](attempts int, simpleSchema bool) (api.Tool, consumer[T], error) {
	s, err := schema.NewSchema[T]()
	if err != nil {
		return api.Tool{}, nil, err
	}
	s = s.Object()
	view := s.View()
	input := view.Raw
	if simpleSchema {
		if input = view.Simple; input == nil {
			return api.Tool{}, nil, errors.New("generate: the result schema cannot be simplified")
		}
	}
	tool := api.Tool{
		Name:   resultTool,
		Desc:   "Submit the result. Call it once, with the complete result as its input.",
		Schema: json.RawMessage(input),
	}
	failures := 0
	consumer := func(content []api.Content) (T, []api.Content, error) {
		var zero T
		var round []api.Content
		var failure error
		for _, c := range content {
			use, ok := c.(api.ContentToolUse)
			if !ok {
				continue
			}
			var err error
			switch {
			case use.Name != resultTool:
				err = fmt.Errorf("%q: %w", use.Name, api.ErrToolNotFound)
			case bytes.Equal(use.Input, api.DiscardedArguments):
				err = api.DiscardedArgumentsError{}
			default:
				input := use.Input
				if len(input) == 0 {
					input = json.RawMessage("{}")
				}
				var v T
				if v, err = s.Decode(input); err == nil {
					return v, nil, nil
				}
			}
			round = append(round, api.NewContentToolResult(use.ID, nil, err))
			failure = err
		}
		if failure == nil {
			failure = errNoResultCall
			round = append(round, api.ContentText{Text: resultReminder})
		}
		failures++
		if failures >= attempts {
			return zero, nil, fmt.Errorf("%w after %d attempts: %w", api.ErrInvalidOutput, failures, failure)
		}
		return zero, round, nil
	}
	return tool, consumer, nil
}

func textConsumer(content []api.Content) (string, []api.Content, error) {
	var b strings.Builder
	for _, c := range content {
		if text, ok := c.(api.ContentText); ok {
			b.WriteString(text.Text)
		}
	}
	return strings.TrimSpace(b.String()), nil, nil
}
