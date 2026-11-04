package neui

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/modules/js"
	"neui.dev/internal/modules/mcp"
	"neui.dev/internal/modules/web"
	"neui.dev/internal/modules/xls"
	"neui.dev/internal/tools"
)

func ToolFS(ctx context.Context) api.FS {
	return ctx.Value(common.KeyController).(common.Controller).FS()
}

func ToolThreadID(ctx context.Context) api.ThreadID {
	return ctx.Value(common.KeyController).(common.Controller).ID()
}

func ToolThreadContext(ctx context.Context) context.Context {
	return ctx.Value(common.KeyController).(common.Controller).Context()
}

// ToolConfig is the configuration the app serving this tool call runs on.
func ToolConfig(ctx context.Context) api.ThreadConfig {
	return ctx.Value(common.KeyController).(common.Controller).Config()
}

// ToolModel is the model the thread streams with, and the limits it reads at
// once, accepts in one image and in one request. A zero field is a limit the
// API does not document.
func ToolModel(ctx context.Context) api.Model {
	return ctx.Value(common.KeyController).(common.Controller).Model()
}

// ToolFilesConfig is the file and workspace limits the app serving this tool
// call runs on, resolved. A tool producing or reading files honours the same
// caps the built-in file tools do by reading them here.
func ToolFilesConfig(ctx context.Context) api.FilesConfig {
	return ctx.Value(common.KeyController).(common.Controller).FilesConfig()
}

// Task is a background task registered with ToolTask. Ok or Err delivers its
// final result and ends it. Notify delivers an interim record while the task
// keeps running; it is dropped once the task has finished.
type Task = common.Task

// ToolTask registers a background task for the calling tool. comment is a
// short description shown to the user while the task runs and stored with its
// result. cancel is what the UI and a user stop invoke; the cause it receives,
// [api.ErrCanceledByUser] for those, is what the task should report back.
func ToolTask(ctx context.Context, comment string, attrs map[string]any, cancel context.CancelCauseFunc) (Task, error) {
	return ctx.Value(common.KeyController).(common.Controller).Task(comment, attrs, cancel)
}

// ToolFromJS reports whether the tool was called from a script rather than by
// the agent directly; a tool can refuse script calls or behave differently.
func ToolFromJS(ctx context.Context) bool {
	return tools.FromJS(ctx)
}

func ToolFromAgent(ctx context.Context) bool {
	return tools.FromAgent(ctx)
}

// ModuleFactory builds one module per thread. The built-in modules are
// factories of this shape, and a host's own module is registered by writing
// one.
type ModuleFactory = common.ModuleFactory

// WithModules registers modules for every session the app serves, as an app
// option for doors.NewApp. OptionModules adds more to one session.
func WithModules(m ...ModuleFactory) doors.With {
	return doors.WithValue(common.ModulesKey{}, m)
}

// OptionModules registers modules for one session, after the app's own.
func OptionModules(m ...ModuleFactory) Option {
	return common.OptionModules(m)
}

func NewModule(name string, desc string) Module {
	return Module{new(sync.Mutex), tools.NewModule(name, desc)}
}

var ModuleJS ModuleFactory = js.Module

// JSConfig tunes the js runtime; a zero field takes its default.
type JSConfig = js.Config

// WithJSConfig tunes the js runtime, as an app option for doors.NewApp. A
// configured app schedules scripts against its own memory and worker limits;
// without it the module runs on defaults.
func WithJSConfig(c JSConfig) doors.With {
	return doors.WithFunc(func() doors.With {
		return doors.WithValue(js.ConfigKey{}, js.NewSeed(c))
	})
}

// XLSConfig tunes the xls module; a zero field takes its default.
type XLSConfig = xls.Config

// ModuleXLS is the xls module: Excel workbooks read and written through the
// workspace. The zero XLSConfig takes every default.
func ModuleXLS(c XLSConfig) ModuleFactory {
	return xls.Module(c)
}

// ModuleWeb is the web module: web__fetch for one URL over HTTP and, when
// WebConfig.Search is set, web__search against that backend. No search
// backend ships with the module, so the zero WebConfig takes every default
// and registers fetch alone.
func ModuleWeb(c WebConfig) ModuleFactory {
	config := web.Config{
		HTTPClient:  c.HTTPClient,
		UserAgent:   c.UserAgent,
		MaxBytes:    c.MaxBytes,
		Timeout:     c.Timeout,
		MaxTimeout:  c.MaxTimeout,
		CacheTTL:    c.CacheTTL,
		CacheBytes:  c.CacheBytes,
		SearchChars: c.SearchChars,
	}
	if c.Search != nil {
		config.Search = searchBackend{c.Search}
	}
	return web.Module(config)
}

// ModuleMCP is a module backed by a remote MCP server over streamable HTTP.
// Its tools register under the module name, as name__tool, and follow the
// server's list-changed notifications. The connection opens when the module
// starts, retries with backoff, and closes after IdleTimeout without a call;
// the next call reconnects and waits for it.
func ModuleMCP(c MCPConfig) ModuleFactory {
	return mcp.Module(mcp.Config(c))
}

// MCPConfig describes one MCP server; a zero field takes its default.
type MCPConfig struct {
	// Name is the module name the server's tools register under.
	Name string
	// URL is the server's streamable HTTP endpoint.
	URL string
	// Headers go on every request to the server, e.g. Authorization.
	Headers map[string]string
	// Enable filters the server's tools by name; nil enables them all.
	Enable func(toolName string) bool
	// IdleTimeout closes the connection after this long without a tool call.
	// Default 10m.
	IdleTimeout time.Duration
	// Ping is the keepalive interval; a missed ping reconnects. Default 30s.
	Ping time.Duration
}

// WebConfig tunes the web module; a zero field takes its default.
type WebConfig struct {
	// HTTPClient is the client every fetch uses. Supplying one replaces the
	// module's own client and with it the whole redirect and address
	// policy: the dialer hook that refuses loopback, private, link-local,
	// unspecified and reserved addresses, the explicit 10-hop cap, the
	// per-hop http/https allowlist, and the stripping of credentials on a
	// redirect all live on the default client only, so Go's own defaults
	// apply instead. A host that passes its own client owns that policy and
	// exposes whatever its network can reach to the model. Leave Timeout
	// at zero: the module bounds each fetch with a context deadline.
	HTTPClient *http.Client
	// Search is the search backend behind web__search. Nil registers no
	// search tool.
	Search SearchBackend
	// UserAgent is the honest identification of this module: what a fetch
	// retries with after a bot wall answers the browser User-Agent with a
	// challenge. Default "neui/0.1 (+https://neui.dev)".
	UserAgent string
	// MaxBytes caps one response body, aborted mid-stream. Default 5<<20.
	MaxBytes int64
	// Timeout bounds a fetch that passes no timeoutMs, and every search.
	// Default 30s.
	Timeout time.Duration
	// MaxTimeout clamps timeoutMs. Default 120s.
	MaxTimeout time.Duration
	// CacheTTL is how long a converted page stays pageable without a second
	// request. Default 15m.
	CacheTTL time.Duration
	// CacheBytes caps the converted text held per thread. Default 8<<20.
	CacheBytes int64
	// SearchChars caps the total page content returned across one search's
	// results. Default 10000.
	SearchChars int
}

// SearchBackend is what web__search queries; supply one as WebConfig.Search.
// It returns results in rank order, best first, and no error when the query
// simply matched nothing.
type SearchBackend interface {
	Search(ctx context.Context, q SearchQuery) ([]SearchResult, error)
}

// SearchQuery is one search request handed to a SearchBackend.
type SearchQuery struct {
	// Text is what to search for, as words rather than a URL.
	Text string
	// Num is how many results the caller wants, best first.
	Num int
	// Domains restricts results to these hostnames; empty means no
	// restriction. A backend that cannot filter on it ignores it.
	Domains []string
	// Since keeps only pages published after it; zero means no window. A
	// backend that cannot filter on it ignores it.
	Since time.Time
}

// SearchResult is one search hit; a backend fills the fields it has and leaves
// the rest empty.
type SearchResult struct {
	// Title is the backend's title for the page.
	Title string
	// URL is the page address, what the model fetches to read it.
	URL string
	// Snippet is the backend's own one- or two-sentence extract; empty when
	// it returns Content instead.
	Snippet string
	// Content is passages from the page when the backend returns them; the
	// module shares WebConfig.SearchChars across all results and truncates.
	Content string
	// Published is the publication date the backend reports, as text; empty
	// when it reports none.
	Published string
}

type searchBackend struct {
	backend SearchBackend
}

func (b searchBackend) Search(ctx context.Context, q web.Query) ([]web.Result, error) {
	results, err := b.backend.Search(ctx, SearchQuery(q))
	out := make([]web.Result, len(results))
	for i, r := range results {
		out[i] = web.Result(r)
	}
	return out, err
}

type AnyModule = common.Module

type AnyTool = common.Tool

// Tooler is a tool definition: a Tool or an Ask. The same value can be added
// to a Module and, for a Tool, to an island Library or rendered on a page.
type Tooler interface {
	tool() (AnyTool, error)
}

// Module is a tool module a host assembles with Add and Remove; they, the
// Tool and Ask shorthands, and the AnyModule reads the runtime makes, are safe
// to call from any goroutine.
type Module struct {
	mu    *sync.Mutex
	inner tools.Module
}

var _ AnyModule = Module{}

func (m Module) Name() string {
	return m.inner.Name()
}

func (m Module) Desc() string {
	return m.inner.Desc()
}

func (m Module) List() []AnyTool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inner.List()
}

func (m Module) Get(name string) (AnyTool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inner.Get(name)
}

// ToolID identifies a tool within its own module, for Remove. A module numbers
// the tools it is given from 1, so an ID is only meaningful against the module
// that issued it.
type ToolID = tools.ToolID

// Add adds the tool t defines and returns its ID; a tool with the same name is
// replaced.
func (m Module) Add(t Tooler) (ToolID, error) {
	tool, err := t.tool()
	if err != nil {
		return 0, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inner.AddTool(tool), nil
}

// Tool is shorthand for Add(Tool[I, O]{name, desc, f}).
func (m Module) Tool[I any, O any](name string, desc string, f func(ctx context.Context, input I) (O, error)) (ToolID, error) {
	return m.Add(Tool[I, O]{Name: name, Desc: desc, Handler: f})
}

type Answer[O any] = common.Answer[O]

// Ask is shorthand for Add(Ask[I, O]{name, desc, f}).
func (m Module) Ask[I any, O any](name string, desc string, f func(input I, answer Answer[O]) api.ChatAsk) (ToolID, error) {
	return m.Add(Ask[I, O]{Name: name, Desc: desc, Handler: f})
}

func (m Module) Remove(id ToolID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner.Remove(id)
}
