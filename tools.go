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

package neui

import (
	"context"
	"errors"
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

// ToolFS returns the thread's workspace filesystem, the same one the files
// module and scripts read and write. A tool that produces a file writes it
// here so the agent can open it by path. It panics when ctx comes from
// neither a tool call nor ToolThreadContext.
func ToolFS(ctx context.Context) api.FS {
	return ctx.Value(common.KeyController).(common.Controller).FS()
}

// ToolThreadID returns the ID of the thread the tool call belongs to, e.g. to
// key host-side state per conversation. It panics when ctx comes from
// neither a tool call nor ToolThreadContext.
func ToolThreadID(ctx context.Context) api.ThreadID {
	return ctx.Value(common.KeyController).(common.Controller).ID()
}

// ToolThreadContext returns a context that outlives the tool call: for a
// module tool it ends when the thread closes, for a page tool when the thread
// is no longer linked to the page that rendered it. ToolFS, ToolThreadID,
// ToolConfig, ToolModel, ToolFilesConfig and ToolTask work on it; ToolFromJS
// and ToolFromAgent report false on it. Run background work registered with
// ToolTask on it. It panics when ctx comes from neither a tool call nor
// ToolThreadContext.
func ToolThreadContext(ctx context.Context) context.Context {
	return ctx.Value(common.KeyController).(common.Controller).Context()
}

// ToolConfig returns the thread configuration of the session serving the
// call: the app's WithThreadConfig with defaults filled in. It panics when ctx
// comes from neither a tool call nor ToolThreadContext.
func ToolConfig(ctx context.Context) api.ThreadConfig {
	return ctx.Value(common.KeyController).(common.Controller).Config()
}

// ToolModel returns the model of the session's provider, the one the thread
// streams with: its name and its context, image and request limits, where a
// zero limit means unknown. It panics when ctx comes from neither a tool call
// nor ToolThreadContext.
func ToolModel(ctx context.Context) api.Model {
	return ctx.Value(common.KeyController).(common.Controller).Model()
}

// ToolFilesConfig returns the file and workspace limits of the session
// serving the call, with defaults filled in: OptionFilesConfig if Enable got
// one, else the app's WithFilesConfig. A tool producing or reading files
// honours the same caps the built-in file tools do by reading them here. It
// panics when ctx comes from neither a tool call nor ToolThreadContext.
func ToolFilesConfig(ctx context.Context) api.FilesConfig {
	return ctx.Value(common.KeyController).(common.Controller).FilesConfig()
}

// Task is a background task registered with ToolTask. Ok or Err delivers its
// final result and ends it. Notify delivers an interim record while the task
// keeps running; it is dropped once the task has finished.
type Task = common.Task

// ToolTask registers a background task for the calling tool; the tool usually
// returns at once and the task delivers its result later through the returned
// Task. comment is a short description shown to the user while the task runs
// and stored with its result; attrs become the task's meta, nil meaning none.
// cancel is what a chat UI calls through api.Task.Cancel to stop the task
// (nekit.Chat passes api.ErrCanceledByUser), and the cause it receives is what the
// task should report with Err; the runtime calls it with ErrCanceledByUser
// when the round that started the task is rewound, and the work must also stop
// when ToolThreadContext ends; cancel must not be nil. Run the work on
// ToolThreadContext, not the call's ctx, which ends with the call. Call it
// before the tool returns: it fails with context.Canceled when the thread is
// closed, with an error when cancel is nil or the round of tool calls it
// belongs to has ended, and with an error when the tool was called from a
// script (ToolFromJS), since scripts cannot start background tasks. It panics
// when ctx comes from neither a tool call nor ToolThreadContext.
func ToolTask(ctx context.Context, comment string, attrs map[string]any, cancel context.CancelCauseFunc) (Task, error) {
	if tools.FromJS(ctx) {
		return nil, errors.New("a background task cannot be started from a script")
	}
	return ctx.Value(common.KeyController).(common.Controller).Task(comment, attrs, cancel)
}

// ToolFromJS reports whether the tool was called from a script rather than by
// the agent directly; a tool can refuse script calls or behave differently.
// It is false outside a tool call.
func ToolFromJS(ctx context.Context) bool {
	return tools.FromJS(ctx)
}

// ToolFromAgent reports whether the agent called the tool directly as a tool
// call, rather than from a script. It is false outside a tool call.
func ToolFromAgent(ctx context.Context) bool {
	return tools.FromAgent(ctx)
}

// ModuleFactory returns the module a thread uses: a fresh one or a shared
// one. The runtime calls it each time a thread opens, with a context that
// lasts while the thread stays open and carries no tool call. When two
// modules of a thread share a name, the later one is skipped and an error is
// logged; the built-in chat, time and files modules come first. Register
// factories with WithModules or OptionModules.
type ModuleFactory = common.ModuleFactory

// WithModules registers modules for every session the app serves, as an app
// option for doors.NewApp. OptionModules adds more to one session.
func WithModules(m ...ModuleFactory) doors.With {
	return doors.WithValue(common.ModulesKey{}, m)
}

// OptionModules registers modules for one session, after the built-in and
// the app's own; a module whose name is already taken is skipped.
func OptionModules(m ...ModuleFactory) Option {
	return common.OptionModules(m)
}

// NewModule returns an empty module to fill with Add, Tool and Ask. Its tools
// reach the agent as name__tool, and desc is added to the system prompt as
// the module's guidance; leave it empty to add nothing. Names are reduced to
// ASCII letters, digits and _ (runs of other characters become one _);
// names that reduce to the same value replace each other, and a name with
// no letter or digit becomes empty. Leading and trailing runs are dropped.
// Register it with a factory that returns the same value for every thread:
//
//	m := neui.NewModule("crm", "Customer records of the signed-in user.")
//	neui.WithModules(func(context.Context) neui.AnyModule { return m })
func NewModule(name string, desc string) Module {
	return Module{new(sync.Mutex), tools.NewModule(name, desc)}
}

// ModuleJS is the js module: js__run lets the agent run a JavaScript file
// from the workspace or inline code, with the thread's tools callable as
// synchronous functions (a tool can refuse script calls, as js's own tools
// do), and js__stop cancels a run that outlived its call. Scripts run on
// QuickJS (plain ES2023: no Node, browser APIs or promises), limited by
// JSConfig.
var ModuleJS ModuleFactory = js.Module

// JSConfig tunes the js runtime; a zero or negative field takes its default.
// The limits are shared by every session of an app that sets WithJSConfig,
// and by every app in the process that does not: a script waits to start
// until a worker and enough of TotalMemory are free.
type JSConfig struct {
	// TotalMemory is the sum in bytes of the memory budgets of all scripts
	// executing at once across the app. Default 512 MiB; a value below
	// WorkerMemory is raised to it.
	TotalMemory int
	// WorkerMemory is the largest memory budget in bytes one script may
	// take. Default 128 MiB; a value below DefaultMemory is raised to it.
	WorkerMemory int
	// DefaultMemory is the memory budget in bytes of a script that asks for
	// less or none. Default 16 MiB, at least 4 MiB.
	DefaultMemory int
	// MaxWorkers is how many scripts may execute at once across the app.
	// Default 16.
	MaxWorkers int
	// JobTimeout caps how long one script may execute, its tool calls
	// included: tool calls in flight are cancelled at the limit. A script may
	// ask for less. Default 30s.
	JobTimeout time.Duration
	// AsyncAfter is how long a run keeps the tool call waiting before it
	// detaches and delivers its result later as a background task. Default
	// 10s.
	AsyncAfter time.Duration
	// MaxQueue is how many scripts may wait to start across the app; one
	// submitted past it is rejected. Default 100.
	MaxQueue int
	// MaxCallDepth is how many nested JS calls a script may make before an
	// error is thrown. Default 1000.
	MaxCallDepth int
}

// WithJSConfig tunes the js runtime of ModuleJS, as an app option for
// doors.NewApp: the app then schedules scripts against its own memory and
// worker limits. Without it, the app runs on defaults and shares one set of
// limits with every other app in the process that does not set it. The
// runtime and its scheduling goroutine live for the rest of the process, even
// after the app is dropped, so build each app once per process.
func WithJSConfig(c JSConfig) doors.With {
	config := js.Config{
		TotalMemory:   c.TotalMemory,
		WorkerMemory:  c.WorkerMemory,
		DefaultMemory: c.DefaultMemory,
		MaxWorkers:    c.MaxWorkers,
		JobTimeout:    c.JobTimeout,
		AsyncAfter:    c.AsyncAfter,
		MaxQueue:      c.MaxQueue,
		MaxCallDepth:  c.MaxCallDepth,
	}
	return doors.WithFunc(func() doors.With {
		return doors.WithValue(js.ConfigKey{}, js.NewSeed(config))
	})
}

// XLSConfig tunes the xls module; a zero field takes its default.
type XLSConfig struct {
	// MaxFileSize is the largest workbook file on disk, in bytes, the module
	// opens; a larger one is refused. Default 50 MiB.
	MaxFileSize int64
	// MaxUnzipSize caps the uncompressed contents of a workbook in bytes; a
	// workbook that expands past it, or holds a part over 1 MiB compressed
	// more than 100x, is refused. Default 1 GiB.
	MaxUnzipSize int64
	// MaxReadRows caps the rows returned, or matches collected, by one call;
	// it also caps the rows profile reads to guess column types. Default 200.
	MaxReadRows uint
}

// ModuleXLS is the xls module: Excel workbooks read and written through the
// workspace. The zero XLSConfig takes every default.
func ModuleXLS(c XLSConfig) ModuleFactory {
	return xls.Module(xls.Config{
		MaxFileSize:  c.MaxFileSize,
		MaxUnzipSize: c.MaxUnzipSize,
		MaxReadRows:  c.MaxReadRows,
	})
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
// Its tools register under MCPConfig.Name, sanitized as NewModule does, as
// name__tool, and follow the server's list-changed notifications. Each thread
// gets its own connection, opened when the thread opens and retried with
// backoff while it fails; a call made while it is failing fails at once. The
// connection, live or still retrying, closes after IdleTimeout without a call,
// or after at most a minute when no call has been made yet, which also stops
// the retries; the next call reconnects and waits for it.
func ModuleMCP(c MCPConfig) ModuleFactory {
	return mcp.Module(mcp.Config(c))
}

// MCPConfig describes one MCP server. Name and URL must be set; any other
// zero field takes its default.
type MCPConfig struct {
	// Name is the module name the server's tools register under, sanitized as
	// NewModule does.
	Name string
	// URL is the server's streamable HTTP endpoint.
	URL string
	// Headers go on every request to the server, e.g. Authorization.
	Headers map[string]string
	// Enable filters the server's tools by name; nil enables them all.
	Enable func(toolName string) bool
	// IdleTimeout closes the connection after this long without a tool call,
	// or after at most a minute when no call has been made yet. Default 10m.
	IdleTimeout time.Duration
	// Ping is the keepalive interval; a missed ping reconnects. Default 30s.
	Ping time.Duration
}

// WebConfig tunes the web module; a zero field takes its default.
type WebConfig struct {
	// HTTPClient is the client every fetch uses. The default client refuses,
	// at connect time and on every redirect hop, any address IANA marks not
	// globally reachable (loopback, private, link-local including cloud
	// metadata, CGNAT, documentation, benchmarking, multicast, reserved,
	// IPv6 outside 2000::/3, 6to4, Teredo) and any address of the host's own
	// network interfaces, judges NAT64 64:ff9b::/96 by the IPv4 address
	// inside it, refuses the ports the WHATWG Fetch standard bars, ignores
	// HTTP_PROXY/HTTPS_PROXY because a proxy hides the real destination, caps
	// redirects at 10, allows only http/https per hop and strips
	// credentials. Passing a client replaces all of it; behind a
	// mandatory proxy, pass a client that uses it and let that proxy enforce
	// destination policy. Leave Timeout at zero: the module bounds each fetch
	// with a context deadline.
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
	// Timeout bounds a fetch the agent sets no timeout for, and every search.
	// Default 30s; a value above MaxTimeout is lowered to it.
	Timeout time.Duration
	// MaxTimeout caps the timeout the agent sets for a fetch, and Timeout.
	// Default 120s.
	MaxTimeout time.Duration
	// CacheTTL is how long a converted page stays pageable without a second
	// request. Default 15m.
	CacheTTL time.Duration
	// CacheBytes caps the converted text held per thread. Default 8<<20; a
	// value below twice MaxBytes is raised to it, so with the default MaxBytes
	// the cap is 10<<20.
	CacheBytes int64
	// SearchChars caps the page content of each search result on its own,
	// in characters; a result cut there ends with "… (content truncated)".
	// Default 2000.
	SearchChars int
}

// SearchBackend is what web__search queries; supply one as WebConfig.Search.
// It returns results in rank order, best first, and no error when the query
// simply matched nothing.
type SearchBackend interface {
	// Search runs q and returns up to q.Num results, best first.
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
	// module caps each result's Content at WebConfig.SearchChars.
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

// AnyModule is what a ModuleFactory returns: a named set of tools. Module
// implements it; a host implements it directly only to serve tools it builds
// itself. The runtime calls its methods from several goroutines and reads
// Desc and List again for every request, so a change shows up at the next
// one.
type AnyModule = common.Module

// AnyTool is one tool as the runtime sees it, the result of a Tool or Ask
// definition.
type AnyTool = common.Tool

// Tooler is a tool definition: a Tool or an Ask. The same value can be added
// to a Module and, for a Tool, to an island Library or rendered on a page.
type Tooler interface {
	tool(object bool) (AnyTool, error)
}

// Module is a tool module a host assembles with Add and Remove; they, the
// Tool and Ask shorthands, and the AnyModule reads the runtime makes, are safe
// to call from any goroutine. Module is a handle: copies share one tool set,
// so one Module can back every thread and a change reaches all of them. Make
// one with NewModule; the zero Module is not usable.
type Module struct {
	mu    *sync.Mutex
	inner tools.Module
}

var _ AnyModule = Module{}

// Name is the module's sanitized name, the prefix of its tools' names.
func (m Module) Name() string {
	return m.inner.Name()
}

// Desc is the module's guidance for the system prompt.
func (m Module) Desc() string {
	return m.inner.Desc()
}

// List returns the module's tools in name order.
func (m Module) List() []AnyTool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inner.List()
}

// Get returns the tool with that name, sanitized as NewModule does, and
// whether there is one.
func (m Module) Get(name string) (AnyTool, bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.inner.Get(name)
}

// ToolID identifies a tool within its own module, for Remove. An ID is only
// meaningful to the module that issued it.
type ToolID = tools.ToolID

// Add adds the tool t defines and returns its ID; a tool with the same name is
// replaced. It fails when the input or output type has no JSON schema.
func (m Module) Add(t Tooler) (ToolID, error) {
	tool, err := t.tool(true)
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

// Answer is how an Ask's form returns the user's answer to the waiting tool
// call: Submit with the answer or Fail with an error.
type Answer[O any] = common.Answer[O]

// Ask is shorthand for Add(Ask[I, O]{name, desc, f}).
func (m Module) Ask[I any, O any](name string, desc string, f func(input I, answer Answer[O]) api.ChatAsk) (ToolID, error) {
	return m.Add(Ask[I, O]{Name: name, Desc: desc, Handler: f})
}

// Remove removes the tool Add returned id for. It does nothing when that tool
// is gone or was replaced by a later Add of the same name.
func (m Module) Remove(id ToolID) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.inner.Remove(id)
}
