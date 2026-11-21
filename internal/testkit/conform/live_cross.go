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

//go:build live

package conform

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/thread/response"
	"neui.dev/provider/lib"
)

func RunLiveCross(t *testing.T, a Adapter) {
	key := os.Getenv(a.Live.Env)
	if key == "" {
		t.Skipf("%s is not set", a.Live.Env)
	}
	m := a.Live.Model
	p := a.Live.Params
	p.Key = key
	declared := append(tools(), api.Tool{Name: "get_weather", Desc: "Current weather for a city.", Schema: map[string]any{
		"type":       "object",
		"properties": map[string]any{"city": map[string]any{"type": "string"}},
		"required":   []string{"city"},
	}})
	send := func(t *testing.T, msgs []api.Message) ([]api.Content, api.EventUsage) {
		t.Helper()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
		defer cancel()
		var events []api.Event
		for e := range a.build(t, a.Live.Base, m, p).Stream(ctx, api.Request{Tools: declared, Messages: msgs}) {
			events = append(events, e)
		}
		if len(events) == 0 {
			t.Fatalf("the turn produced no events: %v", ctx.Err())
		}
		var usage api.EventUsage
		b := response.NewBulder()
		for _, e := range events {
			if u, ok := e.(api.EventUsage); ok {
				usage = u
			}
			b.On(e)
		}
		switch e := events[len(events)-1].(type) {
		case api.EventOk:
		case api.EventErr:
			if e.Code != api.CodeCapped {
				t.Fatalf("turn failed: %s", Describe(e))
			}
		default:
			t.Fatalf("turn ended with no terminal event: %v", DescribeAll(events))
		}
		return b.Flush().Content.Ordered(), usage
	}
	weather := map[string]string{
		"paris": `{"city":"Paris","temp_c":14,"conditions":"light rain","wind_kph":18}`,
		"tokyo": `{"city":"Tokyo","temp_c":21,"conditions":"clear","wind_kph":7}`,
	}
	answers := func(content []api.Content) []api.Content {
		var results []api.Content
		for _, c := range content {
			u, ok := c.(api.ContentToolUse)
			if !ok {
				continue
			}
			var in struct {
				City string `json:"city"`
			}
			_ = json.Unmarshal(u.Input, &in)
			out, ok := weather[strings.ToLower(in.City)]
			if !ok {
				out = `{"error":"unknown city"}`
			}
			results = append(results, result(u.ID, api.ToolOk, text(out)))
		}
		return results
	}
	own := []api.Message{user(text("What is the weather in Paris and in Tokyo right now? Call get_weather once per city, both in one turn, then answer in one sentence."))}
	t.Run("own", func(t *testing.T) {
		for range 3 {
			content, _ := send(t, own)
			own = append(own, assistant(content...))
			results := answers(content)
			if len(results) == 0 {
				break
			}
			own = append(own, user(results...))
		}
		if !*record {
			return
		}
		lines := make([]string, len(own))
		for i, msg := range own {
			data, err := lib.MarshalRaw(msg)
			if err != nil {
				t.Fatal(err)
			}
			lines[i] = string(data)
		}
		dir := filepath.Join(crossDir(), "live")
		if err := os.MkdirAll(dir, 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, a.Name+".json"), []byte("[\n"+strings.Join(lines, ",\n")+"\n]\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	})
	if len(own) < 3 || own[len(own)-1].Role == api.RoleUser {
		t.Fatalf("the own loop did not end in an answer: %d messages", len(own))
	}
	var measured bool
	for _, name := range providers {
		if name == a.Name {
			continue
		}
		var history []api.Message
		if data, err := os.ReadFile(filepath.Join(crossDir(), "live", name+".json")); err == nil {
			if err := json.Unmarshal(data, &history); err != nil {
				t.Fatalf("live/%s.json: %v", name, err)
			}
		} else {
			turns := loadTurns(t, name)
			var first []turn
			for _, tr := range turns {
				if tr.Model == turns[0].Model {
					first = append(first, tr)
				}
			}
			history = thread(first...)
		}
		t.Run(name+"/history", func(t *testing.T) {
			msgs := history[:len(history):len(history)]
			if msgs[len(msgs)-1].Role == api.RoleUser {
				msgs = append(msgs, assistant(text("Noted.")))
			}
			send(t, append(msgs, user(text("Thanks. In one line: what did you find?"))))
		})
		t.Run(name+"/current", func(t *testing.T) {
			last := -1
			for i, msg := range history {
				for _, c := range msg.Content {
					if _, ok := c.(api.ContentToolUse); ok && msg.Role == api.RoleAssistant && i+1 < len(history) {
						last = i
					}
				}
			}
			if last < 0 {
				t.Skip("the history makes no call")
			}
			send(t, history[:last+2])
		})
		var spoken api.Message
		for _, msg := range history {
			if msg.Role != api.RoleAssistant {
				continue
			}
			calls := false
			for _, c := range msg.Content {
				_, call := c.(api.ContentToolUse)
				calls = calls || call
			}
			if !calls {
				spoken = msg
			}
		}
		if spoken.Role == "" {
			continue
		}
		back := append(own[:len(own):len(own)], user(text("Another assistant answered the next question.")), spoken, user(text("Summarise this conversation in one line.")))
		t.Run(name+"/switch-back", func(t *testing.T) {
			_, with := send(t, back)
			if measured {
				return
			}
			measured = true
			stripped := make([]api.Message, len(back))
			found := false
			for i, msg := range back {
				stripped[i] = api.Message{Role: msg.Role}
				for _, c := range msg.Content {
					if r, ok := c.(api.ContentReasoning); ok {
						if _, mine := lib.OwnMeta(r.Meta, a.Name); mine {
							found = true
							continue
						}
					}
					stripped[i].Content = append(stripped[i].Content, c)
				}
			}
			if !found {
				t.Logf("the own loop carries no reasoning of %s, so there is nothing to see read", a.Name)
				return
			}
			_, without := send(t, stripped)
			total := func(u api.EventUsage) int { return u.Input + u.CacheRead + u.CacheWrite }
			t.Logf("input with own reasoning %d, without %d", total(with), total(without))
			if total(with) <= total(without) {
				t.Errorf("own reasoning was not read after the switch back: %d input tokens with it, %d without", total(with), total(without))
			}
		})
	}
}
