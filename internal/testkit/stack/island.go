//go:build browser

package stack

import (
	"context"
	"net"
	"net/http"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/proto"
	"neui.dev/internal/devkit"
)

type IslandApp struct {
	*devkit.Kit
	Page    *rod.Page
	t       *testing.T
	guard   time.Duration
	reports chan devkit.Report
}

func Island(t *testing.T, app devkit.App) *IslandApp {
	t.Helper()
	a := &IslandApp{t: t, guard: 10 * time.Second, reports: make(chan devkit.Report, 256)}
	report := app.Report
	app.Report = func(r devkit.Report) {
		if report != nil {
			report(r)
		}
		select {
		case a.reports <- r:
		default:
		}
	}
	kit, err := devkit.New(app)
	if err != nil {
		t.Fatal(err)
	}
	a.Kit = kit
	t.Cleanup(func() { kit.Close() })
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	server := &http.Server{Handler: kit}
	go server.Serve(listener)
	t.Cleanup(func() { server.Close() })
	page, err := browser.Page(proto.TargetCreateTarget{URL: "http://" + listener.Addr().String() + "/"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), a.guard)
		defer cancel()
		kit.Stop(ctx)
		page.Close()
	})
	if err := page.WaitLoad(); err != nil {
		t.Fatal(err)
	}
	a.Page = page.Timeout(a.guard)
	return a
}

func (a *IslandApp) Until(match func(devkit.Report) bool) []devkit.Report {
	a.t.Helper()
	var reports []devkit.Report
	timeout := time.After(a.guard)
	for {
		select {
		case r := <-a.reports:
			reports = append(reports, r)
			if match(r) {
				return reports
			}
		case <-timeout:
			a.t.Fatalf("no matching report after %v: %v", a.guard, reports)
			return reports
		}
	}
}

func (a *IslandApp) Notified() devkit.Report {
	a.t.Helper()
	r := a.Until(func(devkit.Report) bool { return true })[0]
	if r.Kind != "notify" || r.Meta["kind"] != "notifications_list" {
		a.t.Fatalf("got %v, want a notifications_list", r)
	}
	return r
}
