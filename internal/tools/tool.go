package tools

import (
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

type Value struct {
	Value any `json:"value"`
}

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
	var hint string
	{
		var o O
		switch o := any(o).(type) {
		case api.Container:
			hint = o.ContentType()
		case api.MultiContainer:
			output = schema.BlocksRawSchema.Raw()
			hint = "array of content blocks"
		case string, fmt.Stringer:
			hint = "text"
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
		rawCall:     rawCall,
		stringCall:  stringCall,
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
		rawCall:     rawCall,
		stringCall:  stringCall,
	}, nil
}

type tool struct {
	name        string
	desc        string
	summary     string
	inputSchema []byte
	output      []byte
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

func (c tool) Name() string {
	return c.name
}

func (c tool) Desc() string {
	return c.desc
}

func (c tool) Dispatched(ctx context.Context, arg json.RawMessage) (any, error) {
	switch true {
	case FromAgent(ctx):
		return c.Agent(ctx, arg)
	case FromJS(ctx):
		return c.JS(ctx)([]any{arg})
	default:
		panic("unknown tool call path")
	}
}

func (c tool) Agent(ctx context.Context, input json.RawMessage) (api.UserContents, error) {
	ctx = makeFromAgent(ctx)
	output, err := c.rawCall(ctx, input)
	if err != nil {
		return nil, err
	}
	switch output := output.(type) {
	case api.UserContents:
		return output, nil
	case api.UserContent:
		return api.UserContents{output}, nil
	case api.Container:
		return api.UserContents{output.Content()}, nil
	case api.MultiContainer:
		return output.Contents(), nil
	case string:
		return api.UserContents{api.ContentText{
			Text: output,
		}}, nil
	case fmt.Stringer:
		return api.UserContents{api.ContentText{
			Text: output.String(),
		}}, nil
	}
	json, err := json.Marshal(output)
	if err != nil {
		return nil, err
	}
	return api.UserContents{api.ContentText{
		Text: string(json),
	}}, nil
}

func (c tool) JS(ctx context.Context) quickjs.HostFunc {
	ctx = makeFromJS(ctx)
	return func(args []any) (output any, err error) {
		defer func() {
			if r := recover(); r != nil {
				output = nil
				err = fmt.Errorf("%s: tool panicked: %v", c.name, r)
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
			return nil, fmt.Errorf("%s: %w", c.name, err)
		}
		if _, ok := output.(Value); ok {
			return output, nil
		}
		return Value{output}, nil
	}
}
