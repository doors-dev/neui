package core

import (
	"context"
	_ "embed"
	"encoding/json"
	"fmt"
	"maps"
	"slices"

	"github.com/doors-dev/gox"
	"modernc.org/quickjs"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/schema"
	"neui.dev/internal/tools"
)

//go:embed ui.js
var UIJS string

//go:embed app.js
var appJS string

type Library = *library

func NewLibrary() Library {
	return &library{
		widgets: map[string]anyWidgetFactory{},
		layouts: map[string]anyLayoutFactory{},
		fetch:   tools.NewModule("", ""),
	}
}

type library struct {
	widgets map[string]anyWidgetFactory
	layouts map[string]anyLayoutFactory
	fetch   tools.Module
	head    []gox.Elem
	spec    string
	ends    []int
	items   []string
}

func (l Library) Head(e ...gox.Elem) {
	l.head = append(l.head, e...)
}

func (l *library) RegisterApp(ctx func() context.Context, vm *quickjs.VM) error {
	if err := js.RegisterTools(vm, ctx, "__fetch__", l.fetch.List()); err != nil {
		return err
	}
	m := tools.NewModule("", "")
	_, err := m.Add("fetches", "", func(ctx context.Context, _ struct{}) ([]string, error) {
		return l.fetchNames(), nil
	})
	if err != nil {
		panic(err)
	}
	if err := js.RegisterTools(vm, ctx, "__", m.List()); err != nil {
		return err
	}
	_, err = vm.Eval(appJS, quickjs.EvalGlobal)
	return err
}

func (l Library) widgetNames() []string {
	return slices.AppendSeq(make([]string, 0, len(l.widgets)), maps.Keys(l.widgets))
}

func (l Library) layoutNames() []string {
	return slices.AppendSeq(make([]string, 0, len(l.layouts)), maps.Keys(l.layouts))
}

func (l Library) fetchNames() []string {
	list := l.fetch.List()
	names := make([]string, len(list))
	for i, t := range list {
		names[i] = t.Name()
	}
	return names
}

func (l Library) newLayout(name string, params json.RawMessage, content []gox.Comp) (gox.Comp, error) {
	factory, ok := l.layouts[name]
	if !ok {
		return nil, fmt.Errorf("layout %q: %w", name, errLayoutNotFound)
	}
	layout, err := factory.new(params, content)
	if err != nil {
		return nil, fmt.Errorf("ui.%s params: %w", name, err)
	}
	return layout, nil
}

func (l Library) validateLayout(name string, params json.RawMessage) error {
	factory, ok := l.layouts[name]
	if !ok {
		return fmt.Errorf("layout %q: %w", name, errLayoutNotFound)
	}
	if err := factory.validate(params); err != nil {
		return fmt.Errorf("ui.%s params: %w", name, err)
	}
	return nil
}

func (l Library) newWidget(ctx context.Context, name string, params json.RawMessage, submit func(context.Context, any) bool) (anyWidget, error) {
	factory, ok := l.widgets[name]
	if !ok {
		return nil, fmt.Errorf("widget %q: %w", name, errWidgetKindNotFound)
	}
	widget, err := factory.new(ctx, params, submit)
	if err != nil {
		return nil, fmt.Errorf("ui.%s: %w", name, err)
	}
	return widget, nil
}

func (l Library) Fetch(tool common.Tool) error {
	name := tool.Name()
	if !identifier.MatchString(name) {
		return fmt.Errorf("%q: %w", name, errName)
	}
	if _, ok := l.fetch.Get(name); ok {
		return fmt.Errorf("%q: %w", name, errNameTaken)
	}
	l.fetch.AddTool(tool)
	return nil
}

func (l Library) View[I any, U any](name string, desc string, f func(ctx context.Context, i I) (api.Widget[U], error)) error {
	if err := l.checkName(name); err != nil {
		return err
	}
	inputSchema, err := schema.NewSchema[I]()
	if err != nil {
		return err
	}
	updateSchema, err := schema.NewSchema[U]()
	if err != nil {
		return err
	}
	l.widgets[name] = widgetFactory[I, U, struct{}]{
		desc:         desc,
		updateSchema: updateSchema,
		inputSchema:  inputSchema,
		specs:        []specSchema{{"params", inputSchema.Root(), inputSchema.Defs()}, {"patch", updateSchema.Root(), updateSchema.Defs()}},
		factory: func(ctx context.Context, i I, _ func(context.Context, struct{}) bool) (api.Widget[U], error) {
			return f(ctx, i)
		},
	}
	return nil
}

func (l Library) Layout[I any](name string, desc string, f func(i I, children []gox.Comp) gox.Comp) error {
	if err := l.checkName(name); err != nil {
		return err
	}
	inputSchema, err := schema.NewSchema[I]()
	if err != nil {
		return err
	}
	l.layouts[name] = layoutFactory[I]{
		desc:        desc,
		inputSchema: inputSchema,
		specs:       []specSchema{{"params", inputSchema.Root(), inputSchema.Defs()}},
		factory:     f,
	}
	return nil
}

func (l Library) Control[I any, U any, O any](name string, desc string, f func(ctx context.Context, i I, submit func(ctx context.Context, o O) bool) (api.Widget[U], error)) error {
	if err := l.checkName(name); err != nil {
		return err
	}
	inputSchema, err := schema.NewSchema[I]()
	if err != nil {
		return err
	}
	updateSchema, err := schema.NewSchema[U]()
	if err != nil {
		return err
	}
	outputSchema, err := schema.NewSchema[O]()
	if err != nil {
		return err
	}
	l.widgets[name] = widgetFactory[I, U, O]{
		desc:         desc,
		updateSchema: updateSchema,
		inputSchema:  inputSchema,
		outputSchema: &outputSchema,
		specs:        []specSchema{{"params", inputSchema.Root(), inputSchema.Defs()}, {"patch", updateSchema.Root(), updateSchema.Defs()}, {"output", outputSchema.Root(), outputSchema.Defs()}},
		factory:      f,
	}
	return nil
}

type anyWidgetFactory interface {
	new(ctx context.Context, params json.RawMessage, submit func(context.Context, any) bool) (anyWidget, error)
	control() bool
	spec() (string, []specSchema)
}

type widgetFactory[I, U, O any] struct {
	desc         string
	updateSchema schema.Schema[U]
	inputSchema  schema.Schema[I]
	outputSchema *schema.Schema[O]
	specs        []specSchema
	factory      func(ctx context.Context, i I, submit func(ctx context.Context, o O) bool) (api.Widget[U], error)
}

func (w widgetFactory[I, U, O]) new(ctx context.Context, params json.RawMessage, submit func(context.Context, any) bool) (anyWidget, error) {
	input, err := w.inputSchema.Decode(params)
	if err != nil {
		return nil, fmt.Errorf("params: %w", err)
	}
	inner, err := w.factory(ctx, input, func(ctx context.Context, o O) bool {
		return submit(ctx, o)
	})
	if err != nil {
		return nil, err
	}
	return widget[I, U, O]{
		factory: w,
		inner:   inner,
	}, nil
}

func (w widgetFactory[I, U, O]) control() bool {
	return w.outputSchema != nil
}

func (w widgetFactory[I, U, O]) spec() (string, []specSchema) {
	return w.desc, w.specs
}

type anyLayoutFactory interface {
	new(params json.RawMessage, children []gox.Comp) (gox.Comp, error)
	validate(params json.RawMessage) error
	spec() (string, []specSchema)
}

type layoutFactory[I any] struct {
	desc        string
	inputSchema schema.Schema[I]
	specs       []specSchema
	factory     func(i I, children []gox.Comp) gox.Comp
}

func (l layoutFactory[I]) new(params json.RawMessage, children []gox.Comp) (gox.Comp, error) {
	input, err := l.inputSchema.Decode(params)
	if err != nil {
		return nil, err
	}
	return l.factory(input, children), nil
}

func (l layoutFactory[I]) validate(params json.RawMessage) error {
	_, err := l.inputSchema.Decode(params)
	return err
}

func (l layoutFactory[I]) spec() (string, []specSchema) {
	return l.desc, l.specs
}

type widget[I, U, O any] struct {
	factory widgetFactory[I, U, O]
	inner   api.Widget[U]
}

func (w widget[I, U, O]) IsControl() bool {
	return w.factory.outputSchema != nil
}

func (w widget[I, U, O]) Update(patch json.RawMessage) (func(context.Context), error) {
	p, err := w.factory.updateSchema.Decode(patch)
	if err != nil {
		return nil, err
	}
	return func(ctx context.Context) {
		w.inner.Update(ctx, p)
	}, nil
}

func (w widget[I, U, O]) Main() gox.Elem {
	return w.inner.Main()
}
