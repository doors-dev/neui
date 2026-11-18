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

package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/utils"
)

func NewTool[I any, O any](name string, desc string, f func(ctx context.Context, input I) (O, error)) (common.Tool, error) {
	name = utils.SanitizeName(name)
	inputSchema, err := schema.NewSchema[I]()
	if err != nil {
		return tool{}, err
	}
	outputSchema, err := schema.NewSchema[O]()
	if err != nil {
		return tool{}, err
	}
	output := outputSchema.Raw()
	outputSpec, outputDefs := outputSchema.Root(), outputSchema.Defs()
	var hint string
	{
		var o O
		switch o := any(o).(type) {
		case api.Container:
			hint = "a " + o.ContentType() + " block"
			if strings.IndexAny(o.ContentType(), "aeiou") == 0 {
				hint = "an " + o.ContentType() + " block"
			}
		case api.MultiContainer:
			output = schema.BlocksRawSchema.Raw()
			outputSpec, outputDefs = output, nil
			hint = "content blocks"
		case fmt.Stringer:
			hint = "a text block"
		}
	}
	b := strings.Builder{}
	b.WriteString(desc)
	b.WriteString("\nOutput schema: ")
	b.Write(output)
	if hint != "" {
		b.WriteString("\nOn a direct tool call the result arrives as ")
		b.WriteString(hint)
	}
	rawCall := func(ctx context.Context, input json.RawMessage) (any, error) {
		arg, err := inputSchema.Decode(input)
		if err != nil {
			return nil, err
		}
		return f(ctx, arg)
	}
	stringCall := func(ctx context.Context, input string) (any, error) {
		arg, err := inputSchema.DecodeString(input)
		if err != nil {
			return nil, err
		}
		return f(ctx, arg)
	}
	return tool{
		name:        name,
		desc:        desc,
		summary:     b.String(),
		inputSchema: inputSchema.Raw(),
		output:      output,
		inputSpec:   inputSchema.Root(),
		inputDefs:   inputSchema.Defs(),
		outputSpec:  outputSpec,
		outputDefs:  outputDefs,
		decode: func(input json.RawMessage) (any, error) {
			return inputSchema.Decode(input)
		},
		call: func(ctx context.Context, input any) (any, error) {
			return f(ctx, input.(I))
		},
		rawCall:    rawCall,
		stringCall: stringCall,
	}, nil
}

func NewRawTool(
	name string,
	desc string,
	inputSchema schema.Schema[any],
	outputSchema schema.Schema[any],
	f func(ctx context.Context, input json.RawMessage) (any, error),
) (tool, error) {
	name = utils.SanitizeName(name)
	b := strings.Builder{}
	b.WriteString(desc)
	b.WriteString("\nOutput schema: ")
	b.Write(outputSchema.Raw())
	if bytes.Equal(outputSchema.Raw(), schema.BlocksRawSchema.Raw()) {
		b.WriteString("\nOn a direct tool call the result arrives as content blocks")
	}
	rawCall := func(ctx context.Context, input json.RawMessage) (any, error) {
		if err := inputSchema.ValidateRaw(input); err != nil {
			return nil, err
		}
		return f(ctx, input)
	}
	stringCall := func(ctx context.Context, input string) (any, error) {
		if err := inputSchema.ValidateString(input); err != nil {
			return nil, err
		}
		return f(ctx, json.RawMessage(input))
	}
	return tool{
		name:        name,
		desc:        desc,
		summary:     b.String(),
		inputSchema: inputSchema.Raw(),
		output:      outputSchema.Raw(),
		inputSpec:   inputSchema.Raw(),
		outputSpec:  outputSchema.Raw(),
		decode: func(input json.RawMessage) (any, error) {
			if err := inputSchema.ValidateRaw(input); err != nil {
				return nil, err
			}
			return input, nil
		},
		call: func(ctx context.Context, input any) (any, error) {
			return f(ctx, input.(json.RawMessage))
		},
		rawCall:    rawCall,
		stringCall: stringCall,
	}, nil
}

type tool struct {
	name        string
	desc        string
	summary     string
	inputSchema []byte
	output      []byte
	inputSpec   []byte
	inputDefs   map[string]json.RawMessage
	outputSpec  []byte
	outputDefs  map[string]json.RawMessage
	decode      func(input json.RawMessage) (any, error)
	call        func(ctx context.Context, input any) (any, error)
	rawCall     func(ctx context.Context, input json.RawMessage) (any, error)
	stringCall  func(ctx context.Context, input string) (any, error)
}

func (c tool) Params() []byte {
	return c.inputSchema
}

func (c tool) Summary() string {
	return c.summary
}

func (c tool) Output() []byte {
	return c.output
}

func (c tool) ParamsSpec() ([]byte, map[string]json.RawMessage) {
	return c.inputSpec, c.inputDefs
}

func (c tool) OutputSpec() ([]byte, map[string]json.RawMessage) {
	return c.outputSpec, c.outputDefs
}

func (c tool) Name() string {
	return c.name
}

func (c tool) Desc() string {
	return c.desc
}

func (c tool) Decode(input json.RawMessage) (any, error) {
	return c.decode(input)
}

func (c tool) Call(ctx context.Context, input any) (any, error) {
	return c.call(ctx, input)
}

func (c tool) Agent(ctx context.Context, input json.RawMessage) (api.UserContents, error) {
	ctx = makeFromAgent(ctx)
	output, err := c.rawCall(ctx, input)
	if err != nil {
		return nil, err
	}
	return api.NewToolContents(output)
}

func (c tool) JS(ctx context.Context) quickjs.HostFunc {
	ctx = makeFromJS(ctx)
	return func(args []any) (output any, err error) {
		defer func() {
			if r := recover(); r != nil {
				output = nil
				err = fmt.Errorf("tool panicked: %v", r)
			}
		}()
		if len(args) != 1 {
			return nil, fmt.Errorf("single arg must be supplied")
		}
		switch arg := args[0].(type) {
		case string:
			output, err = c.stringCall(ctx, arg)
		case json.RawMessage:
			output, err = c.rawCall(ctx, arg)
		default:
			return nil, fmt.Errorf("arg must be a JSON string")
		}
		if err != nil {
			return nil, err
		}
		return output, nil
	}
}
