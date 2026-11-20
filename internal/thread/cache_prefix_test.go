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

package thread_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"regexp"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/internal/modules/files"
	"neui.dev/internal/modules/js"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/provider/claude"
)

type wireBody struct {
	Tools    json.RawMessage `json:"tools"`
	System   json.RawMessage `json:"system"`
	Messages []struct {
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	} `json:"messages"`
	CacheControl json.RawMessage `json:"cache_control"`
}

var cacheMark = regexp.MustCompile(`,"cache_control":\{[^}]*\}|"cache_control":\{[^}]*\},?`)

func claudeBodies(t *testing.T, requests []api.Request, opts ...claude.Option) [][]byte {
	t.Helper()
	var bodies [][]byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		bodies = append(bodies, body)
		w.WriteHeader(http.StatusBadRequest)
	}))
	defer srv.Close()
	p, err := claude.New(claude.Conf{Model: claude.Haiku55}, append([]claude.Option{claude.WithAPIKey("k"), claude.WithBaseURL(srv.URL)}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	for _, req := range requests {
		for range p.Stream(context.Background(), req) {
		}
	}
	if len(bodies) != len(requests) {
		t.Fatalf("%d bodies for %d requests", len(bodies), len(requests))
	}
	return bodies
}

func flatten(t *testing.T, body []byte) (wireBody, []string, []int) {
	t.Helper()
	var w wireBody
	if err := json.Unmarshal(body, &w); err != nil {
		t.Fatal(err)
	}
	var blocks []string
	var marks []int
	for i, m := range w.Messages {
		for j, c := range m.Content {
			if bytes.Contains(c, []byte(`"cache_control"`)) && !bytes.Contains(cacheMark.ReplaceAll(c, nil), []byte(`"cache_control"`)) {
				marks = append(marks, len(blocks))
			}
			blocks = append(blocks, fmt.Sprintf("%d/%d %s %s", i, j, m.Role, cacheMark.ReplaceAll(c, nil)))
		}
	}
	return w, blocks, marks
}

func cacheThread(t *testing.T) []api.Request {
	t.Helper()
	run := probe(t, func(ctx context.Context, input json.RawMessage) (any, error) {
		return map[string]any{"zeta": 1, "alpha": []string{"b", "a"}, "echo": input}, nil
	})
	thinkAndCall := testkit.Events(
		api.EventStart{ID: "r1", Model: "fake"},
		api.EventThinking{Index: 0, Text: "I should call the probe twice."},
		api.EventMeta{Index: 0, Data: json.RawMessage(`"c2lnbmF0dXJl"`)},
		api.EventClose{Index: 0},
		api.EventText{Index: 1, Text: "Checking."},
		api.EventClose{Index: 1},
		api.EventTool{Index: 2, ID: "toolu_01", Name: "probe__run", Args: `{"q":"a"}`},
		api.EventClose{Index: 2},
		api.EventTool{Index: 3, ID: "toolu_02", Name: "probe__run", Args: `{"q":"b"}`},
		api.EventClose{Index: 3},
		api.EventUsage{Input: 1, Output: 1},
		api.EventOk{Code: api.CodeTurn},
	)
	p := testkit.NewProvider(api.Model{},
		thinkAndCall,
		testkit.Call("probe__run", map[string]string{"q": "c"}),
		testkit.Call("time__now", map[string]string{}),
		testkit.Say("done"),
		testkit.Say("second answer"),
		testkit.Say("after relink"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{
		common.OptionModule(chatmod.Module),
		common.OptionModule(timemod.Module),
		common.OptionModule(files.Module),
		common.OptionModule(js.Module),
		common.OptionSystem([]string{"You are the assistant of the back office app."}),
		run,
	}})
	c.Send("check the probe")
	c.Wait()
	c.Send("and again")
	c.Wait()
	c.Relink()
	c.Send("after navigation")
	c.Wait()
	requests := p.Requests()
	if len(requests) != 6 {
		t.Fatalf("%d requests, want 6", len(requests))
	}
	return requests
}

func TestClaudePrefixStableAcrossRounds(t *testing.T) {
	requests := cacheThread(t)
	for _, variant := range []struct {
		name string
		opts []claude.Option
	}{
		{"default", nil},
		{"without cache", []claude.Option{claude.WithoutCache()}},
	} {
		bodies := claudeBodies(t, requests, variant.opts...)
		t.Logf("== %s", variant.name)
		var prev wireBody
		var prevBlocks []string
		prevMark := -1
		for i, body := range bodies {
			w, blocks, marks := flatten(t, body)
			tools := cacheMark.ReplaceAll(w.Tools, nil)
			system := cacheMark.ReplaceAll(w.System, nil)
			line := fmt.Sprintf("request %d: %6d bytes, tools %5d bytes (marked %v), system %5d bytes (marked %v), %2d message blocks, markers on blocks %v, top-level %s",
				i+1, len(body), len(tools), bytes.Contains(w.Tools, []byte(`"cache_control"`)), len(system), bytes.Contains(w.System, []byte(`"cache_control"`)), len(blocks), marks, w.CacheControl)
			if i > 0 {
				same := 0
				for same < len(prevBlocks) && same < len(blocks) && prevBlocks[same] == blocks[same] {
					same++
				}
				toolsSame := bytes.Equal(tools, cacheMark.ReplaceAll(prev.Tools, nil))
				systemSame := bytes.Equal(system, cacheMark.ReplaceAll(prev.System, nil))
				line += fmt.Sprintf("; vs previous: tools same %v, system same %v, %d/%d previous blocks are a byte-identical prefix", toolsSame, systemSame, same, len(prevBlocks))
				if variant.opts == nil {
					gap, run := 0, ""
					for _, b := range blocks[prevMark+1 : marks[0]+1] {
						kind := ""
						if strings.Contains(b, `{"type":"tool_use"`) {
							kind = "use"
						}
						if strings.Contains(b, `{"type":"tool_result"`) {
							kind = "result"
						}
						if kind == "" || kind != run {
							gap++
						}
						run = kind
					}
					line += fmt.Sprintf(", marker is %d positions after the previous one", gap)
					if gap > 20 {
						t.Errorf("request %d: marker is %d positions past the previous one, outside the lookback", i+1, gap)
					}
				}
				if !toolsSame || !systemSame || same != len(prevBlocks) {
					t.Errorf("request %d breaks the prefix of request %d", i+1, i)
					if same < len(prevBlocks) && same < len(blocks) {
						t.Logf("first difference:\n prev %s\n next %s", prevBlocks[same], blocks[same])
					}
				}
			}
			t.Log(line)
			if variant.opts == nil {
				if !bytes.Contains(w.System, []byte(`"cache_control"`)) || bytes.Contains(w.Tools, []byte(`"cache_control"`)) || len(marks) != 1 || marks[0] != len(blocks)-1 {
					t.Errorf("request %d: want one marker on the system prompt and one on the last block, got system %s, message markers %v of %d blocks", i+1, w.System[:60], marks, len(blocks))
				}
				prevMark = marks[0]
			} else if bytes.Contains(body, []byte(`"cache_control"`)) {
				t.Errorf("request %d carries a marker with caching off", i+1)
			}
			prev, prevBlocks = w, blocks
		}
		w, _, _ := flatten(t, bodies[0])
		sum := sha256.Sum256(append(cacheMark.ReplaceAll(w.Tools, nil), cacheMark.ReplaceAll(w.System, nil)...))
		t.Logf("tools+system sha256 %x", sum[:8])
		if dir := os.Getenv("CACHE_DUMP"); dir != "" && variant.opts == nil {
			for i, body := range bodies {
				os.WriteFile(fmt.Sprintf("%s/request-%d.json", dir, i+1), body, 0o644)
			}
		}
		if testing.Verbose() && variant.opts == nil {
			_, blocks, _ := flatten(t, bodies[len(bodies)-1])
			for _, b := range blocks {
				if len(b) > 150 {
					b = b[:150] + "..."
				}
				t.Log("  " + strings.ReplaceAll(b, "\n", " "))
			}
		}
	}
}
