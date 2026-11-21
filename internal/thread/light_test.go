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
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"slices"
	"strings"
	"testing"

	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/internal/modules/files"
	"neui.dev/internal/modules/js"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/modules/xls"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/stack"
	"neui.dev/provider/claude"
	"neui.dev/provider/deepseek"
	"neui.dev/provider/gemini"
	"neui.dev/provider/generic"
	"neui.dev/provider/gpt"
)

var scriptWords = regexp.MustCompile(`(?i)\bscripts?\b|island|js__|Output schema|Output definitions`)

func TestLightModeToolsAndSystem(t *testing.T) {
	for _, light := range []bool{false, true} {
		p := testkit.NewProvider(api.Model{}, testkit.Say("hi"))
		c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{
			common.OptionModule(chatmod.Module),
			common.OptionModule(timemod.Module),
			common.OptionModule(files.Module),
			common.OptionModule(js.Module),
			common.OptionModule(xls.Module(xls.Config{})),
			common.OptionLightMode(light),
		}})
		c.Send("hello")
		c.Wait()
		req := p.Request(t, 0)
		var descs, schemas, outputs int
		var found []string
		for _, tool := range req.Tools {
			schema, err := json.Marshal(tool.Schema)
			if err != nil {
				t.Fatal(err)
			}
			descs += len(tool.Name) + len(tool.Desc)
			schemas += len(schema)
			if _, output, ok := strings.Cut(tool.Desc, "\nOutput schema: "); ok {
				outputs += len(output)
			}
			for _, text := range []string{tool.Name, tool.Desc, string(schema)} {
				found = append(found, scriptWords.FindAllString(text, -1)...)
			}
		}
		found = append(found, scriptWords.FindAllString(req.System, -1)...)
		t.Logf("light %v: %d tools, %.1f KB declared: system %.1f KB, names and descriptions %.1f KB (output schemas %.1f KB), input schemas %.1f KB",
			light, len(req.Tools), float64(len(req.System)+descs+schemas)/1024, float64(len(req.System))/1024, float64(descs)/1024, float64(outputs)/1024, float64(schemas)/1024)
		if !light {
			if len(req.Tools) != 50 || len(found) == 0 {
				t.Fatalf("full mode offers %d tools and mentions %d script words, want 50 tools and the full texts", len(req.Tools), len(found))
			}
			continue
		}
		if len(req.Tools) != 27 {
			t.Fatalf("light mode offers %d tools, want 27", len(req.Tools))
		}
		if len(found) != 0 {
			t.Fatalf("light mode texts mention %q", found)
		}
	}
}

func TestHostModuleReadsLightMode(t *testing.T) {
	host := func(ctx context.Context) neui.AnyModule {
		m := neui.NewModule("crm", "")
		name := "full"
		if neui.LightMode(ctx) {
			name = "lite"
		}
		if _, err := m.Tool(name, "reports the mode", func(ctx context.Context, _ struct{}) (string, error) {
			return fmt.Sprint(neui.LightMode(ctx), neui.LightMode(neui.ToolThreadContext(ctx))), nil
		}); err != nil {
			t.Fatal(err)
		}
		return m
	}
	extra := func(ctx context.Context) neui.AnyModule {
		if neui.LightMode(ctx) {
			return nil
		}
		m := neui.NewModule("extra", "")
		if _, err := m.Tool("more", "does more", func(context.Context, struct{}) (string, error) { return "more", nil }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, light := range []bool{false, true} {
		name := "crm__full"
		if light {
			name = "crm__lite"
		}
		p := testkit.NewProvider(api.Model{}, testkit.Call(name, map[string]any{}), testkit.Say("done"))
		c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionModules([]common.ModuleFactory{host, extra}), common.OptionLightMode(light)}})
		c.Send("which mode")
		c.Wait()
		var names []string
		for _, tool := range p.Request(t, 0).Tools {
			if !strings.HasPrefix(tool.Name, "page__") {
				names = append(names, tool.Name)
			}
		}
		want := []string{"crm__full", "extra__more"}
		if light {
			want = []string{"crm__lite"}
		}
		if !slices.Equal(names, want) {
			t.Fatalf("light %v: the host modules offer %v, want %v", light, names, want)
		}
		r := testkit.Results(p.Request(t, 1))[0]
		if text, _ := r.Content[0].(api.ContentText); text.Text != fmt.Sprint(light, light) {
			t.Fatalf("light %v: the tool call saw %#v", light, r.Content)
		}
	}
}

func lightReplay(t *testing.T) api.Request {
	t.Helper()
	p := testkit.NewProvider(api.Model{},
		testkit.Call("js__run", map[string]any{"code": "return 1 + 1", "comment": "adds"}),
		testkit.Call("page__island_begin", map[string]any{"path": "board", "revision": "0", "code": "ui.draw()", "comment": "the board app"}),
		testkit.Say("done"),
		testkit.Say("light"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionModule(timemod.Module), common.OptionModule(js.Module)}})
	c.Send("run it")
	c.Wait()
	c.Switch(common.OptionModule(timemod.Module), common.OptionModule(js.Module), common.OptionLightMode(true))
	c.Send("go on")
	c.Wait()
	req := p.Request(t, 3)
	for _, tool := range req.Tools {
		if tool.Name == "js__run" || strings.HasPrefix(tool.Name, "page__island") {
			t.Fatalf("light mode offers %s", tool.Name)
		}
	}
	var calls []string
	for _, m := range req.Messages {
		for _, block := range m.Content {
			switch b := block.(type) {
			case api.ContentToolUse:
				calls = append(calls, "use "+b.Name)
			case api.ContentToolResult:
				calls = append(calls, "result "+b.ID)
			}
		}
	}
	if want := []string{"use js__run", "result call-js__run", "use page__island_begin", "result call-page__island_begin"}; !slices.Equal(calls, want) {
		t.Fatalf("the light request replays %v, want %v", calls, want)
	}
	return req
}

func TestStoredThreadReplaysAfterSwitchToLight(t *testing.T) {
	req := lightReplay(t)
	for _, a := range []struct {
		name string
		open func(url string) (api.Provider, error)
	}{
		{"claude", func(url string) (api.Provider, error) {
			return claude.New(claude.Conf{Model: claude.Haiku55}, claude.WithAPIKey("k"), claude.WithBaseURL(url))
		}},
		{"gpt", func(url string) (api.Provider, error) {
			return gpt.New(gpt.Conf{Model: gpt.GPT6Luna}, gpt.WithAPIKey("k"), gpt.WithBaseURL(url))
		}},
		{"deepseek", func(url string) (api.Provider, error) {
			return deepseek.New(deepseek.Conf{Model: deepseek.Flash}, deepseek.WithAPIKey("k"), deepseek.WithBaseURL(url))
		}},
		{"gemini", func(url string) (api.Provider, error) {
			return gemini.New(gemini.Conf{Model: gemini.FlashLite35}, gemini.WithAPIKey("k"), gemini.WithBaseURL(url))
		}},
		{"generic", func(url string) (api.Provider, error) {
			return generic.New(generic.Conf{Model: api.Model{Name: "qwen/qwen3.8-27b"}, BaseURL: url}, generic.WithAPIKey("k"))
		}},
	} {
		t.Run(a.name, func(t *testing.T) {
			var body []byte
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				body, _ = io.ReadAll(r.Body)
				w.WriteHeader(http.StatusBadRequest)
			}))
			defer srv.Close()
			provider, err := a.open(srv.URL)
			if err != nil {
				t.Fatal(err)
			}
			for range provider.Stream(context.Background(), req) {
			}
			if body == nil {
				t.Fatal("the adapter sent nothing")
			}
			var wire map[string]any
			if err := json.Unmarshal(body, &wire); err != nil {
				t.Fatal(err)
			}
			var tools []string
			var names func(v any)
			names = func(v any) {
				switch v := v.(type) {
				case map[string]any:
					if name, ok := v["name"].(string); ok {
						tools = append(tools, name)
					}
					for _, x := range v {
						names(x)
					}
				case []any:
					for _, x := range v {
						names(x)
					}
				}
			}
			names(wire["tools"])
			slices.Sort(tools)
			wire["tools"] = tools
			var shorten func(v any) any
			shorten = func(v any) any {
				switch v := v.(type) {
				case map[string]any:
					for k, x := range v {
						v[k] = shorten(x)
					}
				case []any:
					for i, x := range v {
						v[i] = shorten(x)
					}
				case string:
					if len(v) > 200 {
						return fmt.Sprintf("<%d bytes>", len(v))
					}
				}
				return v
			}
			got, err := json.MarshalIndent(shorten(wire), "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			for _, name := range []string{"js__run", "page__island_begin"} {
				if !strings.Contains(string(body), name) || slices.Contains(tools, name) {
					t.Fatalf("the %s body carries %s as a declared tool or not at all", a.name, name)
				}
			}
			testkit.Golden(t, append(got, '\n'))
		})
	}
}
