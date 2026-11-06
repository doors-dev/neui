package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBackoffCarriesAndResets(t *testing.T) {
	var b backoff
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for range 3 {
		if b.wait(ctx) {
			t.Fatal("wait succeeded on a cancelled context")
		}
		if b.delay != time.Second {
			t.Fatalf("got delay %v after a cancelled wait, want 1s", b.delay)
		}
	}
	if !b.wait(context.Background()) {
		t.Fatal("wait failed on a live context")
	}
	if b.delay != 2*time.Second {
		t.Fatalf("got delay %v after a completed wait, want 2s", b.delay)
	}
	b.delay = 30 * time.Second
	if b.wait(ctx) {
		t.Fatal("wait succeeded on a cancelled context")
	}
	if b.delay != 30*time.Second {
		t.Fatalf("got delay %v, want 30s", b.delay)
	}
	b.reset()
	if b.delay != 0 {
		t.Fatalf("got delay %v after reset, want 0", b.delay)
	}
}

func flapServe(t *testing.T, srv *mcp.Server) (string, func() []time.Time) {
	t.Helper()
	var mu sync.Mutex
	var inits []time.Time
	ts, _ := countServe(t, srv, func() {
		mu.Lock()
		inits = append(inits, time.Now())
		mu.Unlock()
	})
	return ts.URL, func() []time.Time {
		mu.Lock()
		defer mu.Unlock()
		return append([]time.Time(nil), inits...)
	}
}

func dropSessions(srv *mcp.Server) {
	for s := range srv.Sessions() {
		s.Close()
	}
}

func TestQuickDropReconnectsAfterBackoff(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	url, inits := flapServe(t, srv)
	startModule(t, srv, Config{Name: "flap", URL: url, IdleTimeout: time.Minute, Ping: 50 * time.Millisecond})
	if !waitFor(5*time.Second, func() bool { return sessions(srv) == 1 }) {
		t.Fatal("never connected")
	}
	dropSessions(srv)
	if !waitFor(5*time.Second, func() bool { return len(inits()) == 2 }) {
		t.Fatal("never reconnected")
	}
	got := inits()
	if gap := got[1].Sub(got[0]); gap < time.Second {
		t.Fatalf("reconnected after %v, want at least the 1s backoff", gap)
	}
}

func TestCallFailsDuringReconnectDelay(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	srv.AddTool(&mcp.Tool{
		Name:        "text",
		Description: "text",
		InputSchema: map[string]any{"type": "object"},
	}, func(ctx context.Context, req *mcp.CallToolRequest) (*mcp.CallToolResult, error) {
		return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: "hello"}}}, nil
	})
	url, inits := flapServe(t, srv)
	m, _ := startModule(t, srv, Config{Name: "delay", URL: url, IdleTimeout: time.Minute, Ping: 50 * time.Millisecond})
	tool := waitTool(t, m, "text")
	if !waitFor(5*time.Second, func() bool { return sessions(srv) == 1 }) {
		t.Fatal("never connected")
	}
	dropSessions(srv)
	if !waitFor(5*time.Second, func() bool {
		ch := make(chan bool, 1)
		if !m.(*module).send(apiCommand{func(m *module) {
			ch <- m.isConnecting() && m.connErrored
		}}) {
			return false
		}
		return <-ch
	}) {
		t.Fatal("drop never started a delayed reconnect")
	}
	started := time.Now()
	_, err := tool.Agent(context.Background(), json.RawMessage(`{}`))
	if err == nil || !strings.Contains(err.Error(), "unreachable") {
		t.Fatalf("got %v during the reconnect delay, want the unreachable error", err)
	}
	if took := time.Since(started); took >= 500*time.Millisecond {
		t.Fatalf("call took %v during the reconnect delay, want it to fail at once", took)
	}
	if n := len(inits()); n != 1 {
		t.Fatalf("got %d initializes during the delay, want 1", n)
	}
	if !waitFor(5*time.Second, func() bool { return len(inits()) == 2 && sessions(srv) == 1 }) {
		t.Fatal("never reconnected")
	}
	if !waitFor(5*time.Second, func() bool {
		_, err := tool.Agent(context.Background(), json.RawMessage(`{}`))
		return err == nil
	}) {
		t.Fatal("calls kept failing after the reconnect")
	}
}

func TestHealthyDropReconnectsAtOnce(t *testing.T) {
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "1"}, nil)
	url, inits := flapServe(t, srv)
	m, _ := startModule(t, srv, Config{Name: "healthy", URL: url, IdleTimeout: time.Minute, Ping: 50 * time.Millisecond})
	if !waitFor(5*time.Second, func() bool { return sessions(srv) == 1 }) {
		t.Fatal("never connected")
	}
	done := make(chan struct{})
	m.(*module).send(apiCommand{func(m *module) {
		m.connectedAt = m.connectedAt.Add(-time.Minute)
		m.backoff.delay = 30 * time.Second
		close(done)
	}})
	<-done
	dropped := time.Now()
	dropSessions(srv)
	if !waitFor(5*time.Second, func() bool { return len(inits()) == 2 }) {
		t.Fatal("never reconnected")
	}
	if gap := inits()[1].Sub(dropped); gap >= time.Second {
		t.Fatalf("reconnected after %v, want at once after a healthy session", gap)
	}
}
