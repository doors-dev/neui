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

//go:build browser

package notify_test

import (
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit/internal/chat/css"
	"neui.dev/nekit/internal/chat/notify"
)

const lifetime = 4 * time.Second

func TestToastShowsOncePerKindAndLeavesALifetimeAfterItsLastShow(t *testing.T) {
	c := stack.Browser(t, stack.Host{Provider: testkit.NewProvider(api.Model{}), Page: func(cur gox.Cursor) error {
		if err := cur.Any(css.Style(css.Theme{}, css.Theme{}, css.Layout{})); err != nil {
			return err
		}
		if err := cur.Init("div"); err != nil {
			return err
		}
		if err := cur.Set("class", "nechat-widget"); err != nil {
			return err
		}
		if err := cur.Submit(); err != nil {
			return err
		}
		if err := cur.Many(notify.Toasts(), notify.Script()); err != nil {
			return err
		}
		return cur.Close()
	}})
	page := c.Page.Timeout(time.Minute)
	page.MustWaitLoad()
	page.MustEval(`() => {
		window.toasts = []
		new MutationObserver((records) => {
			const at = performance.now()
			for (const r of records) {
				for (const n of r.addedNodes) if (n.dataset) toasts.push({ key: n.dataset.key, shown: true, at })
				for (const n of r.removedNodes) if (n.dataset) toasts.push({ key: n.dataset.key, shown: false, at })
			}
		}).observe(document.querySelector(".nechat-toasts"), { childList: true })
	}`)
	show := func(kind string) float64 {
		return page.MustEval(`(kind) => {
			const at = performance.now()
			document.querySelector(".nechat-widget").dispatchEvent(new CustomEvent("nechat_error", { detail: kind }))
			return at
		}`, kind).Num()
	}
	shown := func() string {
		return page.MustEval(`() => Array.from(document.querySelectorAll(".nechat-toast"), (n) => n.dataset.key).join(" ")`).Str()
	}
	network := show("network")
	show("server")
	if got := shown(); got != "network error" {
		t.Fatalf("the toasts shown are %q, want network and error", got)
	}
	time.Sleep(lifetime / 2)
	repeat := show("bad_request")
	if got := shown(); got != "network error" {
		t.Fatalf("after a repeated error the toasts shown are %q, want one per kind", got)
	}
	stack.Eventually(t, 2*lifetime, func() (bool, string) {
		got := shown()
		return got == "", got
	}, "the toasts leave")
	var records []struct {
		Key   string
		Shown bool
		At    float64
	}
	if err := page.MustEval(`() => toasts`).Unmarshal(&records); err != nil {
		t.Fatal(err)
	}
	added := map[string]int{}
	left := map[string]float64{}
	for _, r := range records {
		if r.Shown {
			added[r.Key]++
		} else {
			left[r.Key] = r.At
		}
	}
	if len(added) != 2 || added["network"] != 1 || added["error"] != 1 {
		t.Fatalf("toasts were added %v times, want once per kind", added)
	}
	ms := float64(lifetime.Milliseconds())
	for key, last := range map[string]float64{"network": network, "error": repeat} {
		if after := left[key] - last; after < ms-5 || after > ms+1500 {
			t.Fatalf("the %s toast left %.0fms after its last show, want %v", key, after, lifetime)
		}
	}
}
