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

package generic

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	stdimage "image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/response"
	"neui.dev/internal/testkit/conform"
)

func llamaEnv(t *testing.T) (string, string) {
	t.Helper()
	base, model := os.Getenv("LLAMACPP_URL"), os.Getenv("LLAMACPP_MODEL")
	if base == "" || model == "" {
		t.Skip("LLAMACPP_URL and LLAMACPP_MODEL are not set")
	}
	return base, model
}

func llamaAdapter(t *testing.T) conform.Adapter {
	base, model := llamaEnv(t)
	a := conformAdapter()
	a.New = func(base string, m api.Model, p conform.Params) (api.Provider, error) {
		var opts []Option
		switch p.Effort {
		case "":
		case conform.EffortOff:
			opts = append(opts, WithEffort(EffortNone))
		default:
			opts = append(opts, WithEffort(Effort(p.Effort)))
		}
		if p.Temperature != 0 {
			opts = append(opts, WithTemperature(p.Temperature))
		}
		if p.TopP != 0 {
			opts = append(opts, WithTopP(p.TopP))
		}
		if p.MaxTokens != 0 {
			opts = append(opts, WithMaxTokens(p.MaxTokens))
		}
		return New(Conf{Model: m, BaseURL: base}, opts...)
	}
	a.Live = conform.Live{
		Env:                   "LLAMACPP_URL",
		Model:                 api.Model{Name: model, Context: 262_144},
		Params:                conform.Params{MaxTokens: 4096, Effort: conform.EffortLow},
		Base:                  base,
		Auth:                  func(*http.Request, string) {},
		AnyModel:              true,
		DropsEarlierReasoning: true,
	}
	return a
}

func TestConformLiveLlama(t *testing.T) {
	conform.RunLive(t, llamaAdapter(t))
}

func TestLiveCrossLlama(t *testing.T) {
	conform.RunLiveCross(t, llamaAdapter(t))
}

type llamaTee struct {
	name   string
	bodies []string
}

func (l *llamaTee) RoundTrip(r *http.Request) (*http.Response, error) {
	data, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	l.bodies = append(l.bodies, string(data))
	r.Body = io.NopCloser(bytes.NewReader(data))
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil || l.name == "" || resp.StatusCode != http.StatusOK || !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
		return resp, err
	}
	if f := flag.Lookup("record"); f == nil || f.Value.String() != "true" {
		return resp, err
	}
	file, err := os.Create(filepath.Join("testdata", "streams", "llamacpp-"+l.name+".sse"))
	if err != nil {
		return nil, err
	}
	resp.Body = &llamaBody{resp.Body, file}
	return resp, nil
}

type llamaBody struct {
	io.ReadCloser
	file *os.File
}

func (b *llamaBody) Read(p []byte) (int, error) {
	n, err := b.ReadCloser.Read(p)
	b.file.Write(p[:n])
	return n, err
}

func (b *llamaBody) Close() error {
	b.file.Close()
	return b.ReadCloser.Close()
}

type llamaTurn struct {
	events  []api.Event
	content []api.Content
	usage   api.EventUsage
	last    api.Event
}

func (l llamaTurn) prompt() int {
	return l.usage.Input + l.usage.CacheRead + l.usage.CacheWrite
}

func (l llamaTurn) text() string {
	var b strings.Builder
	for _, c := range l.content {
		if c, ok := c.(api.ContentText); ok {
			b.WriteString(c.Text)
		}
	}
	return b.String()
}

func (l llamaTurn) calls() []api.ContentToolUse {
	var calls []api.ContentToolUse
	for _, c := range l.content {
		if c, ok := c.(api.ContentToolUse); ok {
			calls = append(calls, c)
		}
	}
	return calls
}

func llamaSend(t *testing.T, tee *llamaTee, name string, m api.Model, req api.Request, opts ...Option) llamaTurn {
	t.Helper()
	base, _ := llamaEnv(t)
	tee.name = name
	p, err := New(Conf{Model: m, BaseURL: base}, append([]Option{WithHTTPClient(&http.Client{Transport: tee})}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	var out llamaTurn
	b := response.NewBuilder()
	for e := range p.Stream(ctx, req) {
		out.events = append(out.events, e)
		if u, ok := e.(api.EventUsage); ok {
			out.usage = u
		}
		if _, ok := b.On(e); !ok {
			t.Fatalf("builder refused %s", conform.Describe(e))
		}
	}
	if len(out.events) == 0 {
		t.Fatalf("%s: the stream closed with no event: %v", name, ctx.Err())
	}
	conform.CheckUnframed(t, out.events)
	out.last = out.events[len(out.events)-1]
	out.content = b.Flush().Content.Ordered()
	t.Logf("%s: %s | %s | text %q", name, conform.Describe(out.usage), conform.Describe(out.last), out.text())
	return out
}

func TestLiveLlama(t *testing.T) {
	_, model := llamaEnv(t)
	m := api.Model{Name: model, Context: 262_144}
	tee := &llamaTee{}
	lookup := api.Tool{Name: "lookup", Desc: "Returns the code of an item.", Schema: map[string]any{"type": "object", "properties": map[string]any{"item": map[string]any{"type": "string"}}, "required": []string{"item"}}}
	weather := api.Tool{Name: "get_weather", Desc: "Current weather for a city.", Schema: map[string]any{"type": "object", "properties": map[string]any{"city": map[string]any{"type": "string"}}, "required": []string{"city"}}}
	ok := func(t *testing.T, turn llamaTurn) {
		t.Helper()
		if e, isOk := turn.last.(api.EventOk); !isOk || e.Code != api.CodeTurn {
			t.Fatalf("the turn ended %s, want ok turn", conform.Describe(turn.last))
		}
	}
	failed := func(t *testing.T, turn llamaTurn, code api.ErrCode) {
		t.Helper()
		e, isErr := turn.last.(api.EventErr)
		if !isErr || e.Code != code {
			t.Fatalf("the turn ended %s, want err %s", conform.Describe(turn.last), code)
		}
		var apiErr *APIError
		if errors.As(e.Err, &apiErr) {
			t.Logf("APIError status %d type %q code %q message %q", apiErr.Status, apiErr.Type, apiErr.Code, apiErr.Message)
		}
	}

	t.Run("cache", func(t *testing.T) {
		var system strings.Builder
		fmt.Fprintf(&system, "Ledger run %d.\n", time.Now().UnixNano())
		for i := range 120 {
			fmt.Fprintf(&system, "Record %04d: the ledger keeps entry %d of %d in its original order.\n", i, i*7, 120*7)
		}
		req := api.Request{System: system.String(), Messages: []api.Message{user(text("Reply with the single word ok."))}}
		first := llamaSend(t, tee, "cache-1", m, req, WithEffort(EffortNone), WithMaxTokens(16))
		ok(t, first)
		second := llamaSend(t, tee, "cache-2", m, req, WithEffort(EffortNone), WithMaxTokens(16))
		ok(t, second)
		if second.usage.CacheRead == 0 || second.prompt() != first.prompt() {
			t.Fatalf("the identical second request: %s after %s, want the same prompt read from the cache", conform.Describe(second.usage), conform.Describe(first.usage))
		}
	})

	t.Run("reasoning", func(t *testing.T) {
		msgs := []api.Message{user(text("What is the code of item alpha? Use the lookup tool, then reply with the code only."))}
		first := llamaSend(t, tee, "tools", m, api.Request{Tools: []api.Tool{lookup}, Messages: msgs}, WithMaxTokens(4096))
		ok(t, first)
		calls := first.calls()
		if len(calls) == 0 {
			t.Fatalf("no tool call: %v", conform.DescribeAll(first.events))
		}
		var results api.Contents
		for _, c := range calls {
			results = append(results, api.ContentToolResult{ID: c.ID, Content: api.UserContents{text("alpha: 58")}})
		}
		with := append(append([]api.Message(nil), msgs...), assistant(first.content...), user(results...))
		var stripped []api.Content
		found := false
		for _, c := range first.content {
			if _, isReasoning := c.(api.ContentReasoning); isReasoning {
				found = true
				continue
			}
			stripped = append(stripped, c)
		}
		if !found {
			t.Fatal("the first turn carries no reasoning")
		}
		without := append(append([]api.Message(nil), msgs...), assistant(stripped...), user(results...))
		second := llamaSend(t, tee, "result-rc", m, api.Request{Tools: []api.Tool{lookup}, Messages: with}, WithMaxTokens(4096))
		ok(t, second)
		sentWith := tee.bodies[len(tee.bodies)-1]
		third := llamaSend(t, tee, "", m, api.Request{Tools: []api.Tool{lookup}, Messages: without}, WithMaxTokens(4096))
		ok(t, third)
		if !strings.Contains(sentWith, `"reasoning_content":"`) || strings.Contains(sentWith, `"reasoning":"`) {
			t.Fatalf("the replay does not carry the reasoning in reasoning_content alone:\n%s", sentWith)
		}
		t.Logf("prompt with own reasoning %d (%s), without %d (%s)", second.prompt(), conform.Describe(second.usage), third.prompt(), conform.Describe(third.usage))
		if second.prompt() <= third.prompt() {
			t.Errorf("own reasoning was not counted: %d prompt tokens with it, %d without", second.prompt(), third.prompt())
		}
		if !strings.Contains(second.text(), "58") {
			t.Errorf("the answer does not give the code: %q", second.text())
		}
	})

	t.Run("choice", func(t *testing.T) {
		msgs := []api.Message{user(text("Say hi in one word."))}
		for _, choice := range []api.ToolChoice{api.ToolChoiceAny, api.ToolChoice("get_weather"), api.ToolChoiceNone, api.ToolChoiceAuto} {
			turn := llamaSend(t, tee, "", m, api.Request{Tools: []api.Tool{lookup, weather}, ToolChoice: choice, Messages: msgs}, WithMaxTokens(300))
			var names []string
			for _, c := range turn.calls() {
				names = append(names, c.Name+string(c.Input))
			}
			t.Logf("choice %s: calls %v, ended %s", choice, names, conform.Describe(turn.last))
			if choice == api.ToolChoiceNone && len(names) > 0 {
				t.Errorf("choice none: the model called %v", names)
			}
		}
	})

	t.Run("stop", func(t *testing.T) {
		turn := llamaSend(t, tee, "stop-off", m, api.Request{Messages: []api.Message{user(text("Count from 1 to 10, comma separated, nothing else."))}}, WithEffort(EffortNone), WithStopSequences("7"))
		ok(t, turn)
		if strings.Contains(turn.text(), "7") || !strings.Contains(turn.text(), "6") {
			t.Errorf("the text did not stop before 7: %q", turn.text())
		}
		thinking := llamaSend(t, tee, "stop-thinking", m, api.Request{Messages: []api.Message{user(text("Count from 1 to 10, comma separated, nothing else."))}}, WithStopSequences("7"))
		t.Logf("thinking on: %d blocks, ended %s", len(thinking.content), conform.Describe(thinking.last))
	})

	t.Run("cap", func(t *testing.T) {
		turn := llamaSend(t, tee, "cap", m, api.Request{Messages: []api.Message{user(text("Write a four-line poem about tea."))}}, WithMaxTokens(24))
		failed(t, turn, api.CodeCapped)
	})

	t.Run("empty", func(t *testing.T) {
		msgs := []api.Message{
			{Role: api.RoleUser},
			{Role: api.RoleAssistant},
			user(text("Reply with the single word ok.")),
		}
		turn := llamaSend(t, tee, "empty", m, api.Request{Messages: msgs}, WithEffort(EffortNone), WithMaxTokens(32))
		ok(t, turn)
		if sent := tee.bodies[len(tee.bodies)-1]; strings.Count(sent, `"content":"[empty]"`) != 2 {
			t.Fatalf("the body does not carry two [empty] messages:\n%s", sent)
		}
	})

	t.Run("overflow", func(t *testing.T) {
		long := strings.Repeat(" token", 270_000)
		turn := llamaSend(t, tee, "", m, api.Request{Messages: []api.Message{user(text("Count the words:" + long))}}, WithEffort(EffortNone), WithMaxTokens(16))
		failed(t, turn, api.CodeContextLength)
	})

	t.Run("image-declared", func(t *testing.T) {
		declared := api.Model{Name: model, Context: 262_144, Inputs: api.Modalities{api.ModalityImage}}
		msgs := []api.Message{
			user(text("Look at the photo of item alpha and name its colour in one word.")),
			assistant(api.ContentToolUse{ID: "call_photo", Name: "photo", Input: json.RawMessage(`{"item":"alpha"}`)}),
			user(api.ContentToolResult{ID: "call_photo", Content: api.UserContents{text("photo of alpha:"), api.ContentImage{Data: tinyPNG(), Mime: "image/png"}}}),
		}
		photo := api.Tool{Name: "photo", Desc: "Returns the photo of an item.", Schema: lookup.Schema}
		turn := llamaSend(t, tee, "", declared, api.Request{Tools: []api.Tool{photo}, Messages: msgs}, WithEffort(EffortNone), WithMaxTokens(16))
		if sent := tee.bodies[len(tee.bodies)-1]; !strings.Contains(sent, `"role":"tool"`) || !strings.Contains(sent, `"image_url"`) {
			t.Fatalf("the body does not carry the image in the tool message:\n%.600s", sent)
		}
		failed(t, turn, api.CodeInvalid)
	})
}

func tinyPNG() []byte {
	img := stdimage.NewRGBA(stdimage.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}
