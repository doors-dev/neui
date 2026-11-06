package mcp

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/schema"
	"neui.dev/internal/tools"
)

func serve(t *testing.T, srv *mcp.Server) *httptest.Server {
	t.Helper()
	ts := httptest.NewServer(mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, nil))
	t.Cleanup(ts.Close)
	return ts
}

type requests struct {
	deletes atomic.Int64
	inits   atomic.Int64
}

func countServe(t *testing.T, srv *mcp.Server, gate func()) (*httptest.Server, *requests) {
	t.Helper()
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, nil)
	reqs := &requests{}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete {
			reqs.deletes.Add(1)
		}
		if r.Method == http.MethodPost && r.Header.Get("Mcp-Session-Id") == "" {
			body, err := io.ReadAll(r.Body)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			r.Body = io.NopCloser(bytes.NewReader(body))
			if bytes.Contains(body, []byte(`"method":"initialize"`)) {
				reqs.inits.Add(1)
				if gate != nil {
					gate()
				}
			}
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts, reqs
}

func sessions(srv *mcp.Server) int {
	n := 0
	for range srv.Sessions() {
		n += 1
	}
	return n
}

func waitFor(d time.Duration, f func() bool) bool {
	deadline := time.Now().Add(d)
	for {
		if f() {
			return true
		}
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func startModule(t *testing.T, srv *mcp.Server, conf Config) (common.Module, context.Context) {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	m := Module(conf)(ctx)
	t.Cleanup(func() {
		cancel()
		waitFor(5*time.Second, func() bool {
			return sessions(srv) == 0
		})
	})
	return m, ctx
}

func waitTool(t *testing.T, m common.Module, name string) common.Tool {
	t.Helper()
	var tool common.Tool
	if !waitFor(5*time.Second, func() bool {
		found, ok := m.Get(name)
		tool = found
		return ok
	}) {
		t.Fatalf("tool %q never appeared", name)
	}
	return tool
}

func TestArgumentsVerbatim(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	seen := make(chan json.RawMessage, 1)
	srv.AddTool(&mcp.Tool{
		Name:        "echo",
		Description: "echo",
		InputSchema: map[string]any{
			"type": "object",
			"properties": map[string]any{
				"big": map[string]any{"type": "integer"},
				"num": map[string]any{"type": "number"},
			},
		},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		seen <- bytes.Clone(req.Params.Arguments)
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "ok"}}}, nil
	})
	ts := serve(t, srv)
	m, ctx := startModule(t, srv, Config{
		Name:        "echo",
		URL:         ts.URL,
		IdleTimeout: 5 * time.Second,
		Ping:        2 * time.Second,
	})
	tool := waitTool(t, m, "echo")
	args := json.RawMessage(`{"big":1234567890123456789,"num":1e21}`)
	if _, err := tool.Agent(ctx, args); err != nil {
		t.Fatalf("call: %v", err)
	}
	got := <-seen
	if !bytes.Equal(got, args) {
		t.Fatalf("server saw %s, sent %s", got, args)
	}
}

func TestResultAlwaysBlocks(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        "text",
		Description: "text",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil
	})
	ts := serve(t, srv)
	m, ctx := startModule(t, srv, Config{
		Name:        "text",
		URL:         ts.URL,
		IdleTimeout: 5 * time.Second,
		Ping:        2 * time.Second,
	})
	tool := waitTool(t, m, "text")
	out, err := tool.JS(ctx)([]any{json.RawMessage(`{}`)})
	if err != nil {
		t.Fatalf("call: %v", err)
	}
	value, ok := out.(tools.Value)
	if !ok {
		t.Fatalf("got %T, want tools.Value", out)
	}
	multi, ok := value.Value.(api.MultiContainer)
	if !ok {
		t.Fatalf("got %T (%v), want api.MultiContainer", value.Value, value.Value)
	}
	contents := multi.Contents()
	if len(contents) != 1 {
		t.Fatalf("got %d blocks, want 1", len(contents))
	}
	text, ok := contents[0].(api.ContentText)
	if !ok {
		t.Fatalf("got %T, want api.ContentText", contents[0])
	}
	if text.Text != "hello" {
		t.Fatalf("got %q, want %q", text.Text, "hello")
	}
}

func TestNoInputSchemaBecomesObject(t *testing.T) {
	m := &module{ctx: context.Background(), conf: Config{Name: "bare"}}
	module := m.buildModule([]*mcp.Tool{{Name: "bare", Description: "bare"}})
	tool, ok := module.Get("bare")
	if !ok {
		t.Fatal("tool not registered")
	}
	if !bytes.Equal(tool.Params(), schema.ObjectRawSchema.Raw()) {
		t.Fatalf("got %s, want %s", tool.Params(), schema.ObjectRawSchema.Raw())
	}
}

func TestCollidingAndEmptyNamesHashed(t *testing.T) {
	m := &module{ctx: context.Background(), conf: Config{Name: "srv"}}
	module := m.buildModule([]*mcp.Tool{
		{Name: "get.item", Description: "dot"},
		{Name: "get-item", Description: "dash"},
		{Name: "???", Description: "empty"},
		{Name: "plain", Description: "plain"},
	})
	hash := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:4])
	}
	for name, desc := range map[string]string{
		"get_item_" + hash("get.item"): "dot",
		"get_item_" + hash("get-item"): "dash",
		"t" + hash("???"):              "empty",
		"plain":                        "plain",
	} {
		tool, ok := module.Get(name)
		if !ok {
			t.Fatalf("%s not registered", name)
		}
		if !strings.HasPrefix(tool.Desc(), desc) {
			t.Fatalf("%s: got %q, want prefix %q", name, tool.Desc(), desc)
		}
	}
	if _, ok := module.Get("get_item"); ok {
		t.Fatal("colliding bare name registered")
	}
}

func TestNameSanitized(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	ts := serve(t, srv)
	m, _ := startModule(t, srv, Config{
		Name:        "my mcp server",
		URL:         ts.URL,
		IdleTimeout: 5 * time.Second,
		Ping:        2 * time.Second,
	})
	if m.Name() != "my_mcp_server" {
		t.Fatalf("got %q, want %q", m.Name(), "my_mcp_server")
	}
}

func TestCancelledWaiterKeepsNoInflight(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        "text",
		Description: "text",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil
	})
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return srv
	}, nil)
	var hold atomic.Bool
	gate := make(chan struct{})
	held := make(chan struct{}, 1)
	var opened atomic.Bool
	open := func() {
		if opened.CompareAndSwap(false, true) {
			close(gate)
		}
	}
	defer open()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hold.Load() && r.Header.Get("Mcp-Session-Id") == "" {
			select {
			case held <- struct{}{}:
			default:
			}
			<-gate
		}
		handler.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	idle := 150 * time.Millisecond
	m, _ := startModule(t, srv, Config{
		Name:        "gated",
		URL:         ts.URL,
		IdleTimeout: idle,
		Ping:        2 * time.Second,
	})
	tool := waitTool(t, m, "text")
	if !waitFor(5*time.Second, func() bool {
		return sessions(srv) == 0
	}) {
		t.Fatal("first connection never went idle")
	}
	hold.Store(true)
	callCtx, cancelCall := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := tool.Agent(callCtx, json.RawMessage(`{}`))
		done <- err
	}()
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("reconnect never reached the server")
	}
	cancelCall()
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled call returned no error")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("cancelled call never returned")
	}
	hold.Store(false)
	open()
	if !waitFor(5*time.Second, func() bool {
		return sessions(srv) == 1
	}) {
		t.Fatal("reconnect never completed")
	}
	if !waitFor(5*time.Second, func() bool {
		return sessions(srv) == 0
	}) {
		t.Fatal("session stayed open, the abandoned waiter was counted as inflight")
	}
}

func TestInflightCallHoldsSession(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	release := make(chan struct{})
	var released atomic.Bool
	free := func() {
		if released.CompareAndSwap(false, true) {
			close(release)
		}
	}
	defer free()
	srv.AddTool(&mcp.Tool{
		Name:        "block",
		Description: "block",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		select {
		case <-release:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "done"}}}, nil
	})
	srv.AddTool(&mcp.Tool{
		Name:        "text",
		Description: "text",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil
	})
	ts, reqs := countServe(t, srv, nil)
	idle := 150 * time.Millisecond
	m, _ := startModule(t, srv, Config{
		Name:        "blocking",
		URL:         ts.URL,
		IdleTimeout: idle,
		Ping:        2 * time.Second,
	})
	tool := waitTool(t, m, "block")
	probe := waitTool(t, m, "text")
	if !waitFor(5*time.Second, func() bool {
		return sessions(srv) == 0 && reqs.inits.Load() == 1 && reqs.deletes.Load() == 1
	}) {
		t.Fatalf(
			"first connection never closed (%d initializes, %d deletes, %d sessions)",
			reqs.inits.Load(), reqs.deletes.Load(), sessions(srv),
		)
	}
	done := make(chan error, 1)
	go func() {
		_, err := tool.Agent(context.Background(), json.RawMessage(`{}`))
		done <- err
	}()
	if !waitFor(5*time.Second, func() bool {
		return reqs.inits.Load() == 2
	}) {
		t.Fatalf("call never reconnected (%d initializes)", reqs.inits.Load())
	}
	deadline := time.Now().Add(4 * idle)
	for time.Now().Before(deadline) {
		if d := reqs.deletes.Load(); d != 1 {
			t.Fatalf("session deleted while a call is in flight (%d deletes)", d)
		}
		if i := reqs.inits.Load(); i != 2 {
			t.Fatalf("reconnected while a call is in flight (%d initializes)", i)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if _, err := probe.Agent(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("probe call: %v", err)
	}
	if i := reqs.inits.Load(); i != 2 {
		t.Fatalf("session was dropped while a call is in flight (%d initializes)", i)
	}
	free()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("call: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("call never returned")
	}
	if !waitFor(5*time.Second, func() bool {
		return reqs.deletes.Load() == 2
	}) {
		t.Fatalf("session never deleted after the call finished (%d deletes)", reqs.deletes.Load())
	}
}

func TestParkedWaiterHoldsAttempt(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        "text",
		Description: "text",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil
	})
	var hold atomic.Bool
	gate := make(chan struct{})
	held := make(chan struct{}, 1)
	var opened atomic.Bool
	open := func() {
		if opened.CompareAndSwap(false, true) {
			close(gate)
		}
	}
	defer open()
	ts, reqs := countServe(t, srv, func() {
		if !hold.Load() {
			return
		}
		select {
		case held <- struct{}{}:
		default:
		}
		<-gate
	})
	idle := 200 * time.Millisecond
	m, _ := startModule(t, srv, Config{
		Name:        "parked",
		URL:         ts.URL,
		IdleTimeout: idle,
		Ping:        25 * time.Millisecond,
	})
	mod := m.(*module)
	tool := waitTool(t, m, "text")
	if !waitFor(5*time.Second, func() bool {
		return sessions(srv) == 0 && reqs.inits.Load() == 1 && reqs.deletes.Load() == 1
	}) {
		t.Fatalf(
			"first connection never closed (%d initializes, %d deletes, %d sessions)",
			reqs.inits.Load(), reqs.deletes.Load(), sessions(srv),
		)
	}
	if _, err := tool.Agent(context.Background(), json.RawMessage(`{}`)); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if reqs.inits.Load() != 2 {
		t.Fatalf("first call did not reconnect exactly once (%d initializes)", reqs.inits.Load())
	}
	var session *mcp.ServerSession
	for s := range srv.Sessions() {
		session = s
	}
	if session == nil {
		t.Fatal("first call left no session to close")
	}
	hold.Store(true)
	session.Close()
	select {
	case <-held:
	case <-time.After(5 * time.Second):
		t.Fatal("reconnect never reached the server")
	}
	callCtx, cancelCall := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancelCall()
	done := make(chan error, 1)
	go func() {
		_, err := tool.Agent(callCtx, json.RawMessage(`{}`))
		done <- err
	}()
	if !waitFor(5*time.Second, func() bool {
		ch := make(chan int, 1)
		if !mod.send(apiCommand{func(m *module) {
			ch <- len(m.waiters)
		}}) {
			return false
		}
		return <-ch > 0
	}) {
		t.Fatal("call never parked as a waiter")
	}
	deadline := time.Now().Add(4 * idle)
	for time.Now().Before(deadline) {
		if d := reqs.deletes.Load(); d != 1 {
			t.Fatalf("attempt torn down while a waiter was parked (%d deletes)", d)
		}
		if i := reqs.inits.Load(); i != 3 {
			t.Fatalf("attempt restarted while a waiter was parked (%d initializes)", i)
		}
		time.Sleep(10 * time.Millisecond)
	}
	hold.Store(false)
	open()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("parked call: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("parked call never returned")
	}
}

func TestConnectClearsStaleDesc(t *testing.T) {
	m := &module{ctx: context.Background(), conf: Config{Name: "desc"}, initGuard: make(chan struct{})}
	old := "old guidance"
	m.desc.Store(&old)
	m.on(connectCommand{})
	if d := m.Desc(); d != "" {
		t.Fatalf("got %q, want empty desc after reconnect", d)
	}
}
