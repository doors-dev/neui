package mcp

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func TestBackoffCarriesAndResets(t *testing.T) {
	var b backoff
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	for _, want := range []time.Duration{2 * time.Second, 4 * time.Second, 8 * time.Second, 16 * time.Second, 30 * time.Second, 30 * time.Second} {
		if b.wait(ctx) {
			t.Fatal("wait succeeded on a cancelled context")
		}
		if b.delay != want {
			t.Fatalf("got delay %v, want %v", b.delay, want)
		}
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
