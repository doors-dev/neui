package neui

import (
	"context"
	"fmt"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/instance"
	"neui.dev/internal/island"
	"neui.dev/internal/page"
	"neui.dev/internal/tools"
)

type Space struct {
	Name string
	Desc string
}

func (s Space) Proxy(cur gox.Cursor, el gox.Elem) error {
	ctx := cur.Context()
	ctx, err := instance.GetSpace(ctx).AddSpace(ctx, s.Name, s.Desc)
	if err != nil {
		return err
	}
	return doors.Ctx(ctx).Proxy(cur, el)
}

// Tool defines a tool. Add it to a Module or a Library, or render it to add it
// to the page space it is rendered in.
type Tool[I any, O any] struct {
	Name    string
	Desc    string
	Handler func(ctx context.Context, i I) (O, error)
}

func (a Tool[I, O]) tool() (AnyTool, error) {
	return tools.NewTool(a.Name, a.Desc, a.Handler)
}

func (a Tool[I, O]) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		ctx := cur.Context()
		return instance.GetSpace(ctx).AddTool(ctx, a.Name, a.Desc, a.Handler)
	})
}

var _ gox.Comp = Tool[any, any]{}

// Ask defines a tool that shows the user a question in the chat and returns
// their answer. Add it to a Module; a Library refuses it with ErrAskInIsland.
type Ask[I any, O any] struct {
	Name    string
	Desc    string
	Handler func(input I, answer Answer[O]) api.ChatAsk
}

func (a Ask[I, O]) tool() (AnyTool, error) {
	return Tool[I, O]{Name: a.Name, Desc: a.Desc + tools.AskDesc, Handler: tools.Asking(a.Handler)}.tool()
}

func (a Ask[I, O]) ask() {}

type Data struct {
	Text   string
	Object any
}

func (d Data) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		ctx := cur.Context()
		return instance.GetSpace(ctx).AddData(ctx, d.Text, d.Object)
	})
}

var _ gox.Comp = Data{}

type Rights int

const (
	RW Rights = iota
	R
	W
)

func (r Rights) rights() page.Rights {
	switch r {
	case RW:
		return page.RW
	case R:
		return page.R
	case W:
		return page.W
	default:
		panic("unknown rights")
	}
}

type Input struct {
	Desc   string
	Rights Rights
}

func (u Input) Proxy(cur gox.Cursor, el gox.Elem) error {
	var script gox.Elem
	space := instance.GetSpace(cur.Context())
	mod := gox.ModifyFunc(func(ctx context.Context, tag string, attrs gox.Attrs) error {
		s, err := u.target(ctx, space, tag, attrs)
		if err != nil {
			return err
		}
		script = s
		return nil
	})
	if err := doors.ProxyMod(mod).Proxy(cur, el); err != nil {
		return err
	}
	return cur.Comp(script)
}

func (u Input) target(ctx context.Context, space page.Space, tag string, attrs gox.Attrs) (gox.Elem, error) {
	switch {
	case strings.EqualFold(tag, "button"):
		return space.Button(ctx, u.Rights.rights(), u.Desc, attrs)
	case strings.EqualFold(tag, "a"):
		return space.Link(ctx, u.Rights.rights(), u.Desc, attrs)
	case strings.EqualFold(tag, "input"):
		switch t, _ := attrs.Get("type").Value().(string); {
		case strings.EqualFold(t, "submit"), strings.EqualFold(t, "button"),
			strings.EqualFold(t, "reset"), strings.EqualFold(t, "image"):
			return space.Button(ctx, u.Rights.rights(), u.Desc, attrs)
		case strings.EqualFold(t, "radio"):
			return space.Radio(ctx, u.Rights.rights(), u.Desc, attrs)
		default:
			return space.Input(ctx, u.Rights.rights(), u.Desc, attrs)
		}
	case strings.EqualFold(tag, "textarea"):
		return space.Textarea(ctx, u.Rights.rights(), u.Desc, attrs)
	case strings.EqualFold(tag, "select"):
		return space.Select(ctx, u.Rights.rights(), u.Desc, attrs)
	default:
		return nil, fmt.Errorf("unsupported tag %q for neui.Use", tag)
	}
}

var _ gox.Proxy = Input{}

type Island struct {
	Name        string
	Desc        string
	Placeholder gox.Elem
	Template    func(slot gox.Comp) gox.Elem
	Success     func(close func(ctx context.Context)) gox.Elem
	Failure     func(close func(ctx context.Context), err error) gox.Elem
	Library     func(l Library) error
}

func (i Island) Main() gox.Elem {
	return func(cur gox.Cursor) error {
		lib := island.NewLibrary()
		err := i.Library(Library{lib})
		if err != nil {
			doors.Logger(cur.Context()).Error("neui: island library failed", "island", i.Name, "err", err)
			return cur.Any(i.Placeholder)
		}
		island := island.NewIsland(
			i.Placeholder,
			i.Template,
			i.Success,
			i.Failure,
			lib,
		)
		ctx, err := instance.GetSpace(cur.Context()).AddIsland(cur.Context(), i.Name, i.Desc, island)
		if err != nil {
			return err
		}
		return doors.Ctx(ctx).Proxy(cur, island.Main())
	}
}

// IslandCall is a widget param or patch field that takes a function from the
// island script. See [island.Call].
type IslandCall[I, O any] = island.Call[I, O]

func IslandStore(ctx context.Context) doors.Store {
	store, _ := ctx.Value(common.KeyIslandStore).(doors.Store)
	return store
}

func IslandFS(ctx context.Context) api.FS {
	store := IslandStore(ctx)
	if store == nil {
		return nil
	}
	fs, _ := store.Load(common.IslandFSKey{}).(api.FS)
	return fs
}

func IslandScope(ctx context.Context) doors.Scopes {
	store := IslandStore(ctx)
	if store == nil {
		return nil
	}
	scope, _ := store.Load(common.IslandScopeKey{}).(doors.Scopes)
	return scope
}

type Library struct {
	inner island.Library
}

// Add adds the tool t defines as an app function, which scripts call as
// app.<name>.
func (l Library) Add(t Tooler) error {
	if _, ok := t.(interface{ ask() }); ok {
		return api.ErrAskInIsland
	}
	tool, err := t.tool()
	if err != nil {
		return err
	}
	return l.inner.Fetch(tool)
}

// Tool is shorthand for Add(Tool[I, O]{name, desc, f}).
func (l Library) Tool[I any, O any](name string, desc string, f func(ctx context.Context, input I) (O, error)) error {
	return l.Add(Tool[I, O]{Name: name, Desc: desc, Handler: f})
}

func (l Library) View[I any, U any](name string, desc string, f func(i I) api.Widget[U]) error {
	return l.inner.View(name, desc, f)
}

func (l Library) Layout[I any](name string, desc string, f func(i I, children []gox.Comp) gox.Comp) error {
	return l.inner.Layout(name, desc, f)
}

func (l Library) Head(e gox.Elem) {
	l.inner.Head(e)
}

func (l Library) Control[I any, U any, O any](name string, desc string, f func(i I, submit func(ctx context.Context, o O) bool) api.Widget[U]) error {
	return l.inner.Control(name, desc, f)
}
