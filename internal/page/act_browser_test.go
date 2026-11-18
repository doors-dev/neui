//go:build browser

package page_test

import (
	"encoding/json"
	"math"
	"testing"
	"time"

	"github.com/doors-dev/gox"
	"github.com/go-rod/rod/lib/proto"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
)

type cue struct {
	Class      string
	Overlay    []float64
	Target     []float64
	Animations []string
	Outline    string
	Position   string
	Events     string
	InView     bool
	Added      float64
	Removed    float64
}

func cued() gox.Elem {
	return func(cur gox.Cursor) error {
		return (neui.Space{Name: "note", Desc: "a note"}).Proxy(cur, func(cur gox.Cursor) error {
			if err := (neui.Use{Desc: "the note"}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.InitVoid("input"); err != nil {
					return err
				}
				if err := cur.Set("name", "note"); err != nil {
					return err
				}
				return cur.Submit()
			}); err != nil {
				return err
			}
			if err := cur.Init("div"); err != nil {
				return err
			}
			if err := cur.Set("style", "height: 3000px"); err != nil {
				return err
			}
			if err := cur.Submit(); err != nil {
				return err
			}
			if err := cur.Close(); err != nil {
				return err
			}
			return (neui.Use{Desc: "sends the note"}).Proxy(cur, func(cur gox.Cursor) error {
				if err := cur.Init("button"); err != nil {
					return err
				}
				if err := cur.Set("name", "send"); err != nil {
					return err
				}
				if err := cur.Submit(); err != nil {
					return err
				}
				if err := cur.Text("Send"); err != nil {
					return err
				}
				return cur.Close()
			})
		})
	}
}

func TestUseActionCue(t *testing.T) {
	for _, reduce := range []bool{false, true} {
		name := "motion"
		if reduce {
			name = "reduced motion"
		}
		t.Run(name, func(t *testing.T) {
			inputs := map[string]string{}
			p := testkit.NewProvider(api.Model{},
				testkit.Call("page__sync", map[string]any{"path": "note"}),
				testkit.Then(func(r api.Request) testkit.Turn {
					var s state
					testkit.Decode(t, testkit.Results(r)[0], &s)
					for _, in := range s.Inputs {
						inputs[in.Desc] = in.ID
					}
					return testkit.Call("page__write", map[string]any{"path": "note", "id": inputs["the note"], "value": "hi"})
				}),
				testkit.Then(func(api.Request) testkit.Turn {
					return testkit.Call("page__click", map[string]any{"path": "note", "id": inputs["sends the note"]})
				}),
				testkit.Say("sent"),
			)
			c := stack.Browser(t, stack.Host{Provider: p, Page: cued()})
			if reduce {
				err := proto.EmulationSetEmulatedMedia{Features: []*proto.EmulationMediaFeature{{Name: "prefers-reduced-motion", Value: "reduce"}}}.Call(c.Page)
				if err != nil {
					t.Fatal(err)
				}
			}
			c.Page.MustEval(`() => {
				window.cues = []
				new MutationObserver((records) => {
					for (const r of records) {
						for (const n of r.addedNodes) {
							if (!(n instanceof HTMLElement) || !n.classList.contains("neui-act")) {
								continue
							}
							const target = document.querySelector(n.classList.contains("neui-act-click") ? "button[name=send]" : "input[name=note]")
							const o = n.getBoundingClientRect()
							const t = target.getBoundingClientRect()
							const s = getComputedStyle(n)
							window.cues.push({
								node: n,
								class: n.className,
								overlay: [o.left, o.top, o.width, o.height],
								target: [t.left, t.top, t.width, t.height],
								animations: n.getAnimations({ subtree: true }).map((a) => a.animationName),
								outline: s.outlineStyle,
								position: s.position,
								events: s.pointerEvents,
								inView: t.top >= 0 && t.bottom <= innerHeight,
								added: performance.now(),
							})
						}
						for (const n of r.removedNodes) {
							const c = window.cues.find((c) => c.node === n)
							if (c) {
								c.removed = performance.now()
							}
						}
					}
				}).observe(document.body, { childList: true })
			}`)
			c.Send("send a note")
			c.Wait()
			var cues []cue
			stack.Eventually(t, 3*time.Second, func() (bool, string) {
				raw := c.Page.MustEval(`() => JSON.stringify({
					cues: window.cues.map(({ node, ...c }) => c),
					left: document.querySelectorAll(".neui-act").length,
				})`).Str()
				var got struct {
					Cues []cue
					Left int
				}
				if err := json.Unmarshal([]byte(raw), &got); err != nil {
					t.Fatal(err)
				}
				cues = got.Cues
				done := got.Left == 0 && len(cues) == 2
				for _, c := range cues {
					done = done && c.Removed > 0
				}
				return done, raw
			}, "both overlays shown and removed")
			want := []struct {
				class     string
				animation string
				lasts     float64
			}{
				{"neui-act neui-act-write", "neui-act-glow", 600},
				{"neui-act neui-act-click", "neui-act-ripple", 400},
			}
			for i, w := range want {
				got := cues[i]
				if got.Class != w.class {
					t.Fatalf("overlay %d has class %q, want %q", i, got.Class, w.class)
				}
				for k := range got.Overlay {
					if math.Abs(got.Overlay[k]-got.Target[k]) > 0.5 {
						t.Fatalf("%s overlay sits at %v, want the target's rect %v", w.class, got.Overlay, got.Target)
					}
				}
				if got.Position != "fixed" || got.Events != "none" {
					t.Fatalf("%s overlay has position %s and pointer-events %s, want fixed and none", w.class, got.Position, got.Events)
				}
				if !got.InView {
					t.Fatalf("%s overlay drawn while its target is off screen", w.class)
				}
				if reduce {
					if len(got.Animations) != 0 || got.Outline != "solid" {
						t.Fatalf("%s overlay under reduced motion runs %v with outline %s, want no animation and a solid outline", w.class, got.Animations, got.Outline)
					}
					w.lasts = 600
				} else if len(got.Animations) != 1 || got.Animations[0] != w.animation {
					t.Fatalf("%s overlay runs %v, want %s", w.class, got.Animations, w.animation)
				}
				if lasted := got.Removed - got.Added; lasted < w.lasts*0.9 || lasted > w.lasts+1000 {
					t.Fatalf("%s overlay lasted %.0fms, want about %.0fms", w.class, lasted, w.lasts)
				}
			}
		})
	}
}
