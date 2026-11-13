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

// Space marks a region of the page for the agent: the tools, inputs, data,
// islands and spaces rendered inside it belong to it, and the agent reaches
// them by the space's path. Wrap the region's markup with it:
//
//	~>neui.Space{Name: "cart", Desc: "the user's shopping cart"} <section>...</section>
//
// Content rendered outside any Space belongs to the page root. A space lives
// as long as its render: when that part of the page re-renders or unmounts,
// the space and everything in it go away.
type Space struct {
	// Name is the space's segment of the path. Names are reduced to ASCII
	// letters, digits and _ (runs of other characters become one _); names
	// that reduce to the same value replace each other, and a name with no
	// letter or digit becomes empty. A sibling rendered later with the same
	// name replaces the earlier one.
	Name string
	// Desc tells the agent what the region is and what it is for.
	Desc string
}

// Proxy renders el inside the space. It fails when the enclosing render has
// already ended.
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
	// Name is the tool's name. Names are reduced to ASCII letters, digits
	// and _ (runs of other characters become one _); names that reduce to
	// the same value replace each other, and a name must keep at least one
	// letter or digit.
	Name string
	// Desc tells the agent what the tool does and when to call it; the
	// output schema is appended to it.
	Desc string
	// Handler runs the call with the input decoded from JSON into I; the
	// input and output schemas the agent sees come from I and O. On a direct
	// agent call to a module tool, a string or fmt.Stringer result reaches
	// the agent as text, an api content value as itself and anything else as
	// JSON; a script receives the value as JSON, and a page tool's output is
	// wrapped with the space's new seq and changes. An error is reported to the agent as the
	// call's failure. ctx works with the Tool* functions; for a tool rendered
	// on a page it is also a doors context of that render, so the handler
	// can update the page, and it runs once the page has settled.
	Handler func(ctx context.Context, i I) (O, error)
}

func (a Tool[I, O]) tool() (AnyTool, error) {
	return tools.NewTool(a.Name, a.Desc, a.Handler)
}

// Main adds the tool to the space it is rendered in, for as long as that
// render lives, replacing a tool of the same name there. It renders nothing;
// the render fails when I or O has no JSON schema.
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
	// Name is the tool's name. Names are reduced to ASCII letters, digits
	// and _ (runs of other characters become one _); names that reduce to
	// the same value replace each other, and a name must keep at least one
	// letter or digit.
	Name string
	// Desc tells the agent what the question is for; a note that the call
	// waits for the user and fails when they dismiss it is appended.
	Desc string
	// Handler builds the question from the decoded input: the api.ChatAsk
	// form shown in the chat, whose handlers call answer.Submit or
	// answer.Fail. The runtime sets ChatAsk.Cancel itself, so a dismissal
	// fails the call with api.ErrCanceledByUser.
	Handler func(input I, answer Answer[O]) api.ChatAsk
}

func (a Ask[I, O]) tool() (AnyTool, error) {
	return Tool[I, O]{Name: a.Name, Desc: a.Desc + tools.AskDesc, Handler: tools.Asking(a.Handler)}.tool()
}

func (a Ask[I, O]) ask() {}

// Data attaches information for the agent to the space it is rendered in,
// such as the records a view shows or hints on how to use it. The agent reads
// it when it syncs the page; it need not match what the screen displays.
type Data struct {
	// Text is a note in plain words, on its own or saying what Object is.
	Text string
	// Object is a structured payload, sent to the agent as JSON.
	Object any
}

// Main adds the data to the space it is rendered in, for as long as that
// render lives. It renders nothing.
func (d Data) Main() gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		ctx := cur.Context()
		return instance.GetSpace(ctx).AddData(ctx, d.Text, d.Object)
	})
}

var _ gox.Comp = Data{}

// Rights sets which page calls the agent may make on an Input.
type Rights int

const (
	// RW allows reading, writing, clicking and focusing; it is the zero value.
	RW Rights = iota
	// R allows reading and focusing only.
	R
	// W allows writing, clicking and focusing, but not reading the value.
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

// Input exposes one interactive element to the agent in the enclosing space,
// so it can read, fill or click it as the user would. Wrap exactly one
// element:
//
//	~>neui.Input{Desc: "delete this contact"} <button>Delete</button>
//
// Supported tags are button, a, input, textarea and select; an input of type
// submit, button, reset or image counts as a button, and a radio stands for
// its whole same-name group. The input goes away with its render.
type Input struct {
	// Desc tells the agent what the element does or holds.
	Desc string
	// Rights limits what the agent may do with it. Default RW.
	Rights Rights
}

// Proxy renders el and registers its element. When el does not start with a
// supported element, or the element is already wrapped by another Input, it
// logs the error and renders el unregistered.
func (u Input) Proxy(cur gox.Cursor, el gox.Elem) error {
	printer := &inputPrinter{
		input: u,
		cur:   cur,
	}
	return gox.NewCursor(cur.Context(), printer).Comp(el)
}

type inputPrinterState int

const (
	inputPrinterSeek inputPrinterState = iota
	inputPrinterInject
	inputPrinterDone
)

type inputPrinter struct {
	state  inputPrinterState
	input  Input
	cur    gox.Cursor
	script gox.Elem
	id     uint64
}

func (p *inputPrinter) Send(j gox.Job) error {
	switch p.state {
	case inputPrinterDone:
		return p.cur.Printer().Send(j)
	case inputPrinterSeek:
		open, ok := j.(*gox.JobOpen)
		if !ok {
			doors.Logger(j.Context()).Error("neui: Input must proxy an element directly", "desc", p.input.Desc)
			p.state = inputPrinterDone
			return p.cur.Printer().Send(j)
		}
		if open.Kind == gox.KindContainer {
			return p.cur.Printer().Send(open)
		}
		if err := open.Attrs.ApplyMods(open.Ctx, open.Tag); err != nil {
			return err
		}
		script, err := p.target(open)
		if err != nil {
			doors.Logger(open.Ctx).Error("neui: Input not registered", "desc", p.input.Desc, "err", err)
			p.state = inputPrinterDone
			return p.cur.Printer().Send(open)
		}
		if open.Kind == gox.KindVoid {
			p.state = inputPrinterDone
			if err := p.cur.Printer().Send(open); err != nil {
				return err
			}
			return p.cur.Comp(script)
		}
		p.script = script
		p.id = open.ID
		p.state = inputPrinterInject
		return p.cur.Printer().Send(open)
	case inputPrinterInject:
		close, ok := j.(*gox.JobClose)
		if !ok || close.ID != p.id {
			return p.cur.Printer().Send(j)
		}
		p.state = inputPrinterDone
		if err := p.cur.Printer().Send(close); err != nil {
			return err
		}
		return p.cur.Comp(p.script)
	default:
		panic("neui: unknown input printer state")
	}
}

func (p *inputPrinter) target(open *gox.JobOpen) (gox.Elem, error) {
	ctx, tag, attrs := open.Ctx, open.Tag, open.Attrs
	rights, desc := p.input.Rights.rights(), p.input.Desc
	space := instance.GetSpace(p.cur.Context())
	switch {
	case strings.EqualFold(tag, "button"):
		return space.Button(ctx, rights, desc, attrs)
	case strings.EqualFold(tag, "a"):
		return space.Link(ctx, rights, desc, attrs)
	case strings.EqualFold(tag, "input"):
		switch t, _ := attrs.Get("type").Value().(string); {
		case strings.EqualFold(t, "submit"), strings.EqualFold(t, "button"),
			strings.EqualFold(t, "reset"), strings.EqualFold(t, "image"):
			return space.Button(ctx, rights, desc, attrs)
		case strings.EqualFold(t, "radio"):
			return space.Radio(ctx, rights, desc, attrs)
		default:
			return space.Input(ctx, rights, desc, attrs)
		}
	case strings.EqualFold(tag, "textarea"):
		return space.Textarea(ctx, rights, desc, attrs)
	case strings.EqualFold(tag, "select"):
		return space.Select(ctx, rights, desc, attrs)
	default:
		return nil, fmt.Errorf("unsupported tag %q for neui.Input", tag)
	}
}

var _ gox.Proxy = Input{}

// Island is a region of the page where the agent runs an app: a script that
// draws a UI from the views, controls and layouts its Library offers and
// reacts to what the user does there. It appears to the agent as a space
// marked as an island. Template and Library are required.
type Island struct {
	// Name is the island space's segment of the path. Names are reduced to
	// ASCII letters, digits and _ (runs of other characters become one _);
	// names that reduce to the same value replace each other, and a name
	// with no letter or digit becomes empty.
	Name string
	// Desc tells the agent what the island is for.
	Desc string
	// Placeholder shows while no app runs, including after the user stops
	// one.
	Placeholder gox.Elem
	// Template wraps a running app: slot is where the app's UI goes.
	Template func(slot gox.Comp) gox.Elem
	// Success replaces the app when it ends without error; nil shows
	// Placeholder. Calling close shows Placeholder.
	Success func(close func(ctx context.Context)) gox.Elem
	// Failure replaces the app when it ends with err; nil shows Placeholder.
	// Calling close shows Placeholder.
	Failure func(close func(ctx context.Context), err error) gox.Elem
	// Library fills l with what scripts may use; it runs on every render.
	// Package neisland has built-in layouts and widgets to add.
	Library func(l Library) error
}

// Main renders the island and adds its space. When Library returns an error,
// the error is logged and Placeholder renders with no island space. A running
// app ends when the island unmounts.
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
// island script: the source of a single-argument JavaScript function, such as
// `(row) => app.price(row.sku) * row.qty`. Its syntax is checked when the
// params are decoded, so a broken source throws at the script's line. In the
// widget's schema it reads as a string holding a function from the schema of I
// to the schema of O.
//
// The widget runs it with Call(ctx, i), which decodes the result into an O.
// Calls of one app run one at a time on a JS runtime separate from the island
// script, limited by IslandConfig.FnMemory, with each call limited by
// IslandConfig.FnTime and IslandConfig.MaxCallDepth. Its globals are frozen,
// so no call leaves state for the next, and a call sees only its argument,
// the app functions, console and the standard built-ins. ctx must come from
// the island (a widget's render, Update or event handlers); elsewhere Call
// returns an error. A failure caused by the function, including a result that
// does not decode into O, is written to the app's log and, the first time for
// each source during a script turn and for at most 16 sources, reported to
// the agent as a call_error notification.
type IslandCall[I, O any] = island.Call[I, O]

// IslandReport reports a runtime error of a widget to the agent, as a
// widget_error notification naming the widget and its id. The widget keeps
// running; the agent decides whether to update or replace it. Each widget
// reports a given message once per script, within the same cap as
// call_error, and every report is written to the app's log. ctx must come
// from the island (a widget's render, Update or event handlers); elsewhere
// IslandReport does nothing.
func IslandReport(ctx context.Context, err error) {
	island.Report(ctx, err)
}

// IslandStore returns the store of the island app ctx renders or handles
// events for, shared by all the app's widgets; each app run gets a fresh
// one. It is nil outside an island.
func IslandStore(ctx context.Context) doors.Store {
	store, _ := ctx.Value(common.KeyIslandStore).(doors.Store)
	return store
}

// IslandFS returns the workspace filesystem of the thread running the island
// app, so a widget can read or write the files the agent works with. It is
// nil outside an island.
func IslandFS(ctx context.Context) api.FS {
	store := IslandStore(ctx)
	if store == nil {
		return nil
	}
	fs, _ := store.Load(common.IslandFSKey{}).(api.FS)
	return fs
}

// IslandDir returns the workspace dir of the island app ctx renders or
// handles events for, where widgets keep their files. It is emptied when the
// app starts and kept after it ends. It is "" outside an island.
func IslandDir(ctx context.Context) string {
	store := IslandStore(ctx)
	if store == nil {
		return ""
	}
	dir, _ := store.Load(common.IslandDirKey{}).(string)
	return dir
}

// IslandScope returns a blocking doors scope shared by all widgets of the
// island app: put it on their submit hooks so a new submit is rejected while
// an earlier one is still being handled. Leave it off hooks that only update
// the widget itself. It is nil outside an island.
func IslandScope(ctx context.Context) doors.Scopes {
	store := IslandStore(ctx)
	if store == nil {
		return nil
	}
	scope, _ := store.Load(common.IslandScopeKey{}).(doors.Scopes)
	return scope
}

// Library is what an island's scripts may use: app functions, views,
// controls, layouts and head elements. Island.Library fills it. Names of
// views, controls and layouts must be JavaScript identifiers, unique among
// them, and not slot, draw, notify or root; scripts reach them as ui.<name>.
type Library struct {
	inner island.Library
}

// Add adds the tool t defines as an app function, which scripts call as
// app.<name>, with the name sanitized as NewModule does. It fails when that
// name is empty, starts with a digit or is used by another app function,
// when the input or output type has no JSON schema, and with
// api.ErrAskInIsland for an Ask.
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

// View adds a widget scripts create with params decoded into I and later
// patch with values decoded into U; f builds it from the params, with the
// island ctx of the script call that creates it. Use it for
// UI that shows rather than collects, such as text, tables or charts.
func (l Library) View[I any, U any](name string, desc string, f func(ctx context.Context, i I) (api.Widget[U], error)) error {
	return l.inner.View(name, desc, f)
}

// Layout adds a container scripts use to arrange other items; f builds it
// from the params decoded into I and the rendered children, in order.
func (l Library) Layout[I any](name string, desc string, f func(i I, children []gox.Comp) gox.Comp) error {
	return l.inner.Layout(name, desc, f)
}

// Head adds elements, such as stylesheets, rendered ahead of every app the
// island runs.
func (l Library) Head(e ...gox.Elem) {
	l.inner.Head(e...)
}

// Control adds a widget like View that also hands the script a value O the
// user produced, such as a form submission. submit delivers o to the script
// waiting in ui.draw() and reports whether it was taken; it returns false
// when no script is waiting or the app has ended, so the widget can keep the
// user's input. Call it from an event handler, with that handler's ctx.
func (l Library) Control[I any, U any, O any](name string, desc string, f func(ctx context.Context, i I, submit func(ctx context.Context, o O) bool) (api.Widget[U], error)) error {
	return l.inner.Control(name, desc, f)
}
