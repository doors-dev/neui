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

var scriptWords = regexp.MustCompile(`(?i)\bscripts?\b|js__|Output schema|Output definitions`)

var islandWords = regexp.MustCompile(`(?i)island`)

func TestTierToolsAndSystem(t *testing.T) {
	for _, tier := range []common.ModelTier{common.TierDeveloper, common.TierAgent, common.TierAssistant} {
		p := testkit.NewProvider(api.Model{}, testkit.Say("hi"))
		c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{
			common.OptionModule(chatmod.Module),
			common.OptionModule(timemod.Module),
			common.OptionModule(files.Module),
			common.OptionModule(js.Module),
			common.OptionModule(xls.Module(xls.Config{})),
			common.OptionTier(tier),
		}})
		c.Send("hello")
		c.Wait()
		req := p.Request(t, 0)
		var descs, schemas, outputs int
		var scripts, islands []string
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
				scripts = append(scripts, scriptWords.FindAllString(text, -1)...)
				islands = append(islands, islandWords.FindAllString(strings.ReplaceAll(text, `"island":{"type":"boolean"}`, ""), -1)...)
			}
		}
		scripts = append(scripts, scriptWords.FindAllString(req.System, -1)...)
		islands = append(islands, islandWords.FindAllString(strings.ReplaceAll(req.System, `"island":{"type":"boolean"}`, ""), -1)...)
		t.Logf("tier %d: %d tools, %.1f KB declared: system %.1f KB, names and descriptions %.1f KB (output schemas %.1f KB), input schemas %.1f KB",
			tier, len(req.Tools), float64(len(req.System)+descs+schemas)/1024, float64(len(req.System))/1024, float64(descs)/1024, float64(outputs)/1024, float64(schemas)/1024)
		names := map[string]bool{}
		for _, tool := range req.Tools {
			names[tool.Name] = true
		}
		switch tier {
		case common.TierDeveloper:
			if len(req.Tools) != 50 || len(scripts) == 0 || len(islands) == 0 {
				t.Fatalf("TierDeveloper offers %d tools and mentions %d script and %d island words, want 50 tools and the full texts", len(req.Tools), len(scripts), len(islands))
			}
		case common.TierAgent:
			if len(req.Tools) != 43 || len(scripts) == 0 || outputs == 0 || !names["js__run"] || !names["files__zip"] || !names["xls__format"] {
				t.Fatalf("TierAgent offers %d tools with %d bytes of output schemas and mentions %d script words, want 43 tools with js, the full files and xls sets and the script texts", len(req.Tools), outputs, len(scripts))
			}
			if len(islands) != 0 {
				t.Fatalf("TierAgent texts mention %q", islands)
			}
		case common.TierAssistant:
			if len(req.Tools) != 27 {
				t.Fatalf("TierAssistant offers %d tools, want 27", len(req.Tools))
			}
			if found := append(scripts, islands...); len(found) != 0 {
				t.Fatalf("TierAssistant texts mention %q", found)
			}
		}
	}
}

func TestHostModuleReadsTier(t *testing.T) {
	names := map[common.ModelTier]string{common.TierDeveloper: "developer", common.TierAgent: "agent", common.TierAssistant: "assistant"}
	host := func(ctx context.Context) neui.AnyModule {
		m := neui.NewModule("crm", "")
		if _, err := m.Tool(names[neui.Tier(ctx)], "reports the tier", func(ctx context.Context, _ struct{}) (string, error) {
			return fmt.Sprint(neui.Tier(ctx), neui.Tier(neui.ToolThreadContext(ctx))), nil
		}); err != nil {
			t.Fatal(err)
		}
		return m
	}
	extra := func(ctx context.Context) neui.AnyModule {
		if neui.Tier(ctx) == neui.TierAssistant {
			return nil
		}
		m := neui.NewModule("extra", "")
		if _, err := m.Tool("more", "does more", func(context.Context, struct{}) (string, error) { return "more", nil }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, tier := range []common.ModelTier{common.TierDeveloper, common.TierAgent, common.TierAssistant} {
		name := "crm__" + names[tier]
		p := testkit.NewProvider(api.Model{}, testkit.Call(name, map[string]any{}), testkit.Say("done"))
		c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionModules([]common.ModuleFactory{host, extra}), common.OptionTier(tier)}})
		c.Send("which tier")
		c.Wait()
		var offered []string
		for _, tool := range p.Request(t, 0).Tools {
			if !strings.HasPrefix(tool.Name, "page__") {
				offered = append(offered, tool.Name)
			}
		}
		want := []string{name, "extra__more"}
		if tier == common.TierAssistant {
			want = []string{name}
		}
		if !slices.Equal(offered, want) {
			t.Fatalf("tier %d: the host modules offer %v, want %v", tier, offered, want)
		}
		r := testkit.Results(p.Request(t, 1))[0]
		if text, _ := r.Content[0].(api.ContentText); text.Text != fmt.Sprint(tier, tier) {
			t.Fatalf("tier %d: the tool call saw %#v", tier, r.Content)
		}
	}
}

type describedRows struct {
	_    struct{} `description:"the rows; truncated: call again"`
	Rows []string `json:"rows"`
}

func TestTierShowsResultDescriptions(t *testing.T) {
	host := func(context.Context) neui.AnyModule {
		m := neui.NewModule("crm", "")
		if _, err := m.Tool("rows", "Read rows.", func(context.Context, struct{}) (describedRows, error) { return describedRows{}, nil }); err != nil {
			t.Fatal(err)
		}
		if _, err := m.Tool("text", "Read text.", func(context.Context, struct{}) (string, error) { return "", nil }); err != nil {
			t.Fatal(err)
		}
		return m
	}
	for _, tier := range []common.ModelTier{common.TierDeveloper, common.TierAgent, common.TierAssistant} {
		assistant := tier == common.TierAssistant
		p := testkit.NewProvider(api.Model{}, testkit.Say("hi"))
		c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionModules([]common.ModuleFactory{host}), common.OptionTier(tier)}})
		c.Send("hello")
		c.Wait()
		descs := map[string]string{}
		for _, tool := range p.Request(t, 0).Tools {
			descs[tool.Name] = tool.Desc
		}
		rows, text := "Read rows.\nOutput schema: {\"description\":\"the rows; truncated: call again\",", "Read text.\nOutput schema: "
		if assistant {
			rows, text = "Read rows.\nResult: the rows; truncated: call again", "Read text."
		}
		if !strings.HasPrefix(descs["crm__rows"], rows) || !strings.HasPrefix(descs["crm__text"], text) || assistant && (descs["crm__rows"] != rows || descs["crm__text"] != text) {
			t.Fatalf("tier %d: the tools read %q and %q, want %q and %q", tier, descs["crm__rows"], descs["crm__text"], rows, text)
		}
	}
}

func assistantReplay(t *testing.T) api.Request {
	t.Helper()
	p := testkit.NewProvider(api.Model{},
		testkit.Call("js__run", map[string]any{"code": "return 1 + 1", "comment": "adds"}),
		testkit.Call("page__island_begin", map[string]any{"path": "board", "revision": "0", "code": "ui.draw()", "comment": "the board app"}),
		testkit.Say("done"),
		testkit.Say("assistant"),
	)
	c := stack.Headless(t, stack.Host{Provider: p, Options: []common.Option{common.OptionModule(timemod.Module), common.OptionModule(js.Module)}})
	c.Send("run it")
	c.Wait()
	c.Switch(common.OptionModule(timemod.Module), common.OptionModule(js.Module), common.OptionTier(common.TierAssistant))
	c.Send("go on")
	c.Wait()
	req := p.Request(t, 3)
	for _, tool := range req.Tools {
		if tool.Name == "js__run" || strings.HasPrefix(tool.Name, "page__island") {
			t.Fatalf("TierAssistant offers %s", tool.Name)
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
		t.Fatalf("the TierAssistant request replays %v, want %v", calls, want)
	}
	return req
}

func TestStoredThreadReplaysAfterSwitchToAssistant(t *testing.T) {
	req := assistantReplay(t)
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
