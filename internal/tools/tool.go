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
			hint = o.ContentType()
		case api.MultiContainer:
			output = schema.BlocksRawSchema.Raw()
			outputSpec, outputDefs = output, nil
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
		inputSpec:   inputSchema.Root(),
		inputDefs:   inputSchema.Defs(),
		outputSpec:  outputSpec,
		outputDefs:  outputDefs,
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
	if bytes.Equal(outputSchema.Raw(), schema.BlocksRawSchema.Raw()) {
		b.WriteString("\nOn a direct tool call the result arrives as array of content blocks")
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
	inputSpec   []byte
	inputDefs   map[string]json.RawMessage
	outputSpec  []byte
	outputDefs  map[string]json.RawMessage
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
		return output, nil
	}
}
