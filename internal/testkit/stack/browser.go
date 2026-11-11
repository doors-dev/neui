//go:build browser

package stack

import (
	"cmp"
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
	"github.com/go-rod/rod/lib/proto"
	"neui.dev"
	"neui.dev/api"
)

var browser *rod.Browser

func Main(m *testing.M) {
	os.Exit(func() int {
		dir, err := os.MkdirTemp("", "neui-browser")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return 1
		}
		defer os.RemoveAll(dir)
		os.Setenv("TMPDIR", dir)
		l := launcher.New().Headless(true).NoSandbox(true).Leakless(false).UserDataDir(filepath.Join(dir, "chrome"))
		shells, _ := filepath.Glob(filepath.Join(os.Getenv("HOME"), "Library/Caches/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-mac-arm64/chrome-headless-shell"))
		slices.SortFunc(shells, func(a, b string) int {
			return cmp.Or(cmp.Compare(len(a), len(b)), strings.Compare(a, b))
		})
		if bin := os.Getenv("NEUI_TEST_CHROME"); bin != "" {
			l = l.Bin(bin)
		} else if len(shells) > 0 {
			l = l.Bin(shells[len(shells)-1]).Set("use-mock-keychain").Set("password-store", "basic")
		} else if bin, ok := launcher.LookPath(); ok {
			l = l.Bin(bin)
		}
		control, err := l.Launch()
		if err != nil {
			fmt.Fprintln(os.Stderr, "stack: chrome did not start:", err)
			return 1
		}
		defer l.Kill()
		browser = rod.New().ControlURL(control)
		if err := browser.Connect(); err != nil {
			fmt.Fprintln(os.Stderr, "stack: chrome did not connect:", err)
			return 1
		}
		defer browser.Close()
		return m.Run()
	}())
}

type factory struct {
	instance api.Instance
	events   chan<- api.ChatEvent
	linked   chan<- linked
}

func (f factory) Chat(thread api.ChatThread, state api.ChatState) api.Chat {
	f.linked <- linked{thread: thread, instance: f.instance, state: state}
	return view{f.events}
}

func (f factory) Placeholder(doors.Beam[bool]) gox.Elem {
	return func(cur gox.Cursor) error { return cur.Text("no chat") }
}

type view struct {
	events chan<- api.ChatEvent
}

func (v view) Update(ctx context.Context, e api.ChatEvent) {
	v.events <- e
}

func (v view) Main() gox.Elem {
	return func(cur gox.Cursor) error { return cur.Text("chat") }
}

func Browser(t *testing.T, h Host) *Chat {
	t.Helper()
	c := &Chat{
		t:      t,
		guard:  10 * time.Second,
		events: make(chan api.ChatEvent, 256),
		links:  make(chan linked, 4),
	}
	c.send = func(input api.Input) error {
		return c.current.thread.Send(context.Background(), input)
	}
	app := doors.NewApp(func(context.Context, doors.Request) gox.Elem {
		return func(cur gox.Cursor) error {
			ctx := cur.Context()
			if err := neui.Enable(ctx, append([]neui.Option{neui.OptionProvider(h.Provider)}, h.Options...)...); err != nil {
				return err
			}
			if err := cur.Init("html"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Init("head"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Close(); err != nil {
				return err
			}
			if err := cur.Init("body"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if h.Page != nil {
				if err := cur.Any(h.Page); err != nil {
					return err
				}
			}
			instance := neui.Instance(ctx)
			if err := cur.Any(instance.Render(factory{instance: instance, events: c.events, linked: c.links})); err != nil {
				return err
			}
			if err := cur.Close(); err != nil {
				return err
			}
			return cur.Close()
		}
	}, doors.WithID(doors.IDRand()), neui.WithThreadConfig(h.Config), neui.WithIslandConfig(h.Island))
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: app}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	page, err := browser.Page(proto.TargetCreateTarget{URL: "http://" + listener.Addr().String() + "/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { page.Close() })
	c.Linked()
	return c
}

func (c *Chat) Linked() api.ChatState {
	c.t.Helper()
	select {
	case c.current = <-c.links:
		return c.current.state
	case <-time.After(c.guard):
		c.t.Fatalf("no chat linked after %v", c.guard)
		return api.ChatState{}
	}
}

func (c *Chat) Instance() api.Instance {
	return c.current.instance
}
