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

package thread_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	chatmod "neui.dev/internal/modules/chat"
	"neui.dev/internal/modules/files"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/modules/xls"
	"neui.dev/internal/testkit"
	"neui.dev/internal/testkit/conform"
	"neui.dev/internal/testkit/stack"
	"neui.dev/provider/generic"
)

type llamaWire struct {
	mu     sync.Mutex
	dir    string
	bodies []string
}

func (w *llamaWire) RoundTrip(r *http.Request) (*http.Response, error) {
	data, err := io.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, err
	}
	r.Body = io.NopCloser(bytes.NewReader(data))
	w.mu.Lock()
	w.bodies = append(w.bodies, string(data))
	n := len(w.bodies)
	w.mu.Unlock()
	resp, err := http.DefaultTransport.RoundTrip(r)
	if err != nil || w.dir == "" {
		return resp, err
	}
	if err := os.WriteFile(filepath.Join(w.dir, fmt.Sprintf("request-%d.json", n)), data, 0o644); err != nil {
		return nil, err
	}
	file, err := os.Create(filepath.Join(w.dir, fmt.Sprintf("response-%d.sse", n)))
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

type llamaRecorder struct {
	api.Provider
	mu       sync.Mutex
	requests []api.Request
	turns    [][]api.Event
}

func (r *llamaRecorder) Stream(ctx context.Context, req api.Request) <-chan api.Event {
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.turns = append(r.turns, nil)
	i := len(r.turns) - 1
	r.mu.Unlock()
	in := r.Provider.Stream(ctx, req)
	out := make(chan api.Event)
	go func() {
		defer close(out)
		for e := range in {
			r.mu.Lock()
			r.turns[i] = append(r.turns[i], e)
			r.mu.Unlock()
			select {
			case out <- e:
			case <-ctx.Done():
			}
		}
	}()
	return out
}

func llamaProvider(t *testing.T, m api.Model, wire *llamaWire, opts ...generic.Option) *llamaRecorder {
	t.Helper()
	base, model := os.Getenv("LLAMACPP_URL"), os.Getenv("LLAMACPP_MODEL")
	if base == "" || model == "" {
		t.Skip("LLAMACPP_URL and LLAMACPP_MODEL are not set")
	}
	m.Name = model
	p, err := generic.New(generic.Conf{Model: m, BaseURL: base}, append([]generic.Option{generic.WithHTTPClient(&http.Client{Transport: wire})}, opts...)...)
	if err != nil {
		t.Fatal(err)
	}
	return &llamaRecorder{Provider: p}
}

func llamaReport(t *testing.T, rec *llamaRecorder, wire *llamaWire, events []api.ChatEvent) {
	t.Helper()
	for i, turn := range rec.turns {
		var calls, texts []string
		var usage api.EventUsage
		var thinking int
		for _, e := range turn {
			switch e := e.(type) {
			case api.EventTool:
				if e.Name != "" || e.Args != "" {
					calls = append(calls, e.Name+e.Args)
				}
			case api.EventText:
				texts = append(texts, e.Text)
			case api.EventThinking:
				thinking += len(e.Text)
			case api.EventUsage:
				usage = e
			}
		}
		var last string
		if len(turn) > 0 {
			last = conform.Describe(turn[len(turn)-1])
		}
		t.Logf("request %d: %d bytes, %d messages, %d tools | %s | thinking %d chars | calls %s | text %q | %s",
			i+1, len(wire.bodies[i]), len(rec.requests[i].Messages), len(rec.requests[i].Tools), conform.Describe(usage), thinking, strings.Join(calls, " "), strings.Join(texts, ""), last)
	}
	for _, e := range events {
		if end, ok := e.(api.ChatEventRoundEnd); ok {
			t.Logf("round end: state %v error %q usage %s", end.State, end.Error, conform.Describe(end.Usage))
		}
	}
}

func TestLiveLlamaLightFirstTurn(t *testing.T) {
	wire := &llamaWire{dir: os.Getenv("LLAMACPP_DUMP")}
	rec := llamaProvider(t, api.Model{Context: 262_144}, wire)
	c := stack.Headless(t, stack.Host{Provider: testkit.NewProvider(api.Model{}), Guard: 10 * time.Minute, Options: []common.Option{
		common.OptionModule(chatmod.Module),
		common.OptionModule(timemod.Module),
		common.OptionModule(files.Module),
		common.OptionModule(xls.Module(xls.Config{})),
		common.OptionLightMode(true),
		common.OptionSystem([]string{"Back office of Kettle & Burr, a small coffee gear shop. " +
			"The user works on the pages next to the chat: prefer acting there, so they see it happen. " +
			"Deletions happen only when the user confirms them on the page."}),
		common.OptionProvider(rec),
	}})
	c.Send("Create a spreadsheet orders.xlsx with the columns Product and Quantity and one row: Cold brew jar, 3. Then tell me what time it is now.")
	events := c.Wait()
	llamaReport(t, rec, wire, events)
	if len(rec.requests) == 0 {
		t.Fatal("no request was made")
	}
	var names []string
	for _, tool := range rec.requests[0].Tools {
		names = append(names, tool.Name)
	}
	t.Logf("first request: %d tools %v, system %d bytes", len(names), names, len(rec.requests[0].System))
}

func TestLiveLlamaToolImage(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	host := func(context.Context) neui.AnyModule {
		m := neui.NewModule("shop", "")
		if _, err := m.Tool("photo", "Returns the product photo of an item.", func(context.Context, struct {
			Item string `json:"item"`
		}) (api.Image, error) {
			return api.Image{Data: b.Bytes(), Mime: api.ImagePNG}, nil
		}); err != nil {
			t.Fatal(err)
		}
		return m
	}
	wire := &llamaWire{dir: os.Getenv("LLAMACPP_DUMP")}
	rec := llamaProvider(t, api.Model{Context: 262_144}, wire, generic.WithMaxTokens(2048))
	c := stack.Headless(t, stack.Host{Provider: testkit.NewProvider(api.Model{}), Guard: 4 * time.Minute, Options: []common.Option{
		common.OptionModules([]common.ModuleFactory{host}),
		common.OptionLightMode(true),
		common.OptionProvider(rec),
	}})
	c.Send("Look at the product photo of the cold brew jar with shop__photo and tell me its colour in one word.")
	events := c.Wait()
	llamaReport(t, rec, wire, events)
	if len(wire.bodies) < 2 {
		t.Fatalf("%d requests, want the photo call and its answer", len(wire.bodies))
	}
	second := wire.bodies[1]
	var body struct {
		Messages []json.RawMessage `json:"messages"`
	}
	json.Unmarshal([]byte(second), &body)
	for _, m := range body.Messages {
		if strings.Contains(string(m), `"role":"tool"`) {
			t.Logf("tool message on the wire: %s", m)
		}
	}
	if strings.Contains(second, "image_url") || !strings.Contains(second, "noimage") {
		t.Fatalf("the photo did not become a note before the wire")
	}
	for _, e := range events {
		if end, ok := e.(api.ChatEventRoundEnd); ok && end.State != api.RoundOk {
			t.Fatalf("the round ended %v %q", end.State, end.Error)
		}
	}
}
