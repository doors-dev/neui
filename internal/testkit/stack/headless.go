package stack

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
	"neui.dev/internal/thread"
)

const guard = 5 * time.Second

type Host struct {
	Provider *testkit.Provider
	Config   api.ThreadConfig
	Options  []common.Option
	Page     gox.Elem
}

type Chat struct {
	t      *testing.T
	link   thread.Link
	events chan api.ChatEvent
	inits  chan api.ChatState
}

func Headless(t *testing.T, h Host) *Chat {
	t.Helper()
	ctx := context.WithValue(context.Background(), common.ConfigKey{}, h.Config)
	return open(t, h, ctx, page.NewPage(ctx))
}

func open(t *testing.T, h Host, ctx context.Context, p page.Page) *Chat {
	t.Helper()
	store, err := local.NewFileStorage()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	options := append([]common.Option{common.OptionProvider(h.Provider)}, h.Options...)
	backend := common.NewBackend(ctx, local.NewMessageStorage(), store, store, options)
	c := &Chat{
		t:      t,
		link:   thread.NewLink(backend, p),
		events: make(chan api.ChatEvent, 256),
		inits:  make(chan api.ChatState, 1),
	}
	t.Cleanup(c.Close)
	receive, done := c.link.Receiver()
	go func() {
		for {
			select {
			case <-done:
				return
			case m := <-receive:
				switch m := m.(type) {
				case thread.MessageEvent:
					select {
					case c.events <- m.Event:
					case <-done:
					}
				case thread.MessageInit:
					select {
					case c.inits <- m.State:
					case <-done:
					}
				}
				m.Ack()
			}
		}
	}()
	c.State()
	return c
}

func (c *Chat) Do(command func(l thread.Link, op thread.Operation) thread.Command) error {
	op := thread.NewOperation(context.Background())
	c.link.Sender() <- command(c.link, op)
	return op.Receive()
}

func (c *Chat) Send(text string) {
	c.t.Helper()
	if err := c.Do(func(l thread.Link, op thread.Operation) thread.Command { return l.Send(op, api.Input{Text: text}) }); err != nil {
		c.t.Fatalf("send %q: %v", text, err)
	}
}

func (c *Chat) Stop() {
	c.t.Helper()
	if err := c.Do(thread.Link.Stop); err != nil {
		c.t.Fatalf("stop: %v", err)
	}
}

func (c *Chat) Retry() {
	c.t.Helper()
	if err := c.Do(thread.Link.Retry); err != nil {
		c.t.Fatalf("retry: %v", err)
	}
}

func (c *Chat) Rewind(round api.RoundID, text string) {
	c.t.Helper()
	if err := c.Do(func(l thread.Link, op thread.Operation) thread.Command {
		return l.Rewind(op, round, api.Input{Text: text})
	}); err != nil {
		c.t.Fatalf("rewind to %v with %q: %v", round, text, err)
	}
}

func (c *Chat) State() api.ChatState {
	c.t.Helper()
	c.link.Sender() <- c.link.Init()
	select {
	case state := <-c.inits:
		return state
	case <-time.After(guard):
		c.t.Fatalf("no init after %v", guard)
		return api.ChatState{}
	}
}

func (c *Chat) Wait() []api.ChatEvent {
	c.t.Helper()
	return c.Until(func(e api.ChatEvent) bool {
		end, ok := e.(api.ChatEventRoundEnd)
		return ok && end.Final
	})
}

func (c *Chat) Until(match func(api.ChatEvent) bool) []api.ChatEvent {
	c.t.Helper()
	var events []api.ChatEvent
	timeout := time.After(guard)
	for {
		select {
		case e := <-c.events:
			events = append(events, e)
			if match(e) {
				return events
			}
		case <-timeout:
			c.t.Fatalf("no matching event after %v: %v", guard, Trace(events))
			return events
		}
	}
}

func (c *Chat) Close() {
	c.link.Close()
}

func Trace(events []api.ChatEvent) []string {
	trace := make([]string, len(events))
	for i, e := range events {
		switch e := e.(type) {
		case api.ChatEventRoundEnd:
			parts := []string{string(e.State)}
			if e.Error != "" {
				parts = append(parts, e.Error)
			}
			if e.Final {
				parts = append(parts, "final")
			}
			trace[i] = "RoundEnd(" + strings.Join(parts, ",") + ")"
		case api.ChatEventRoundStart:
			trace[i] = "RoundStart"
			if e.Truncate != api.NoRoundID {
				trace[i] = "RoundStart(truncate)"
			}
		case api.ChatEventInputPending:
			trace[i] = fmt.Sprintf("InputPending(%v)", e.Pending)
		case api.ChatEventCompacting:
			trace[i] = fmt.Sprintf("Compacting(%v)", e.Running)
		default:
			trace[i] = strings.TrimPrefix(fmt.Sprintf("%T", e), "api.ChatEvent")
		}
	}
	return trace
}
