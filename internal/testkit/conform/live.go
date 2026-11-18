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
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"flag"
	"image"
	"image/color"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/thread/response"
)

var record = flag.Bool("record", false, "save the live streams to testdata/streams, scrubbed, as level one fixtures")

func RunLive(t *testing.T, a Adapter) {
	key := os.Getenv(a.Live.Env)
	if key == "" {
		t.Skipf("%s is not set", a.Live.Env)
	}
	sch, err := loadSpec(a.Spec.File, a.Spec.LooseOneOf, a.Spec.Accept)
	if err != nil {
		t.Fatal(err)
	}
	t.Run("Count", func(t *testing.T) { liveCount(t, a, sch, key) })
	t.Run("Generate", func(t *testing.T) { liveGenerate(t, a, key) })
}

func liveCount(t *testing.T, a Adapter, sch *schemas, key string) {
	if a.Live.Count == "" {
		t.Skipf("%s has no token counting endpoint", a.Name)
	}
	m := a.Live.Model
	srv := newServer(t)
	ok := mustRender(t, a, m, scriptText)
	probe := entry(t, m.Name, ProbeEntry)
	seen := map[[32]byte]bool{}
	for _, p := range Matrix(0, a.Sampling) {
		if ex := a.expect(m.Name, ProbeEntry, p); ex.want != Send || ex.bug != "" {
			continue
		}
		srv.reset(reply{body: ok})
		collect(t, a.build(t, srv.srv.URL, m, p), p.request(probe))
		body, _ := srv.last()
		if body == nil {
			continue
		}
		stripped, err := strip(body, a.Live.Strip)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(stripped)
		if seen[sum] {
			continue
		}
		seen[sum] = true
		if a.Spec.Count != "" {
			if err := sch.validate(a.Spec.Count, stripped, a.Spec.Private, a.Spec.Defaults); err != nil {
				t.Errorf("%s: stripped body does not match %s, so Live.Strip is wrong: %v", p, a.Spec.Count, err)
				continue
			}
		}
		req, err := http.NewRequest(http.MethodPost, a.Live.Base+a.Live.Count, bytes.NewReader(stripped))
		if err != nil {
			t.Fatal(err)
		}
		req.Header.Set("Content-Type", "application/json")
		a.Live.Auth(req, key)
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatal(err)
		}
		reply, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Errorf("%s: the API refused a body the rules expect it to accept, so a rule is missing: %d %s\nbody: %s", p, resp.StatusCode, reply, clip(stripped))
		}
	}
}

func strip(body []byte, ptrs []string) ([]byte, error) {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return nil, err
	}
	for _, p := range ptrs {
		v = deletePointer(v, splitPointer(p))
	}
	return json.Marshal(v)
}

func tinyPNG() []byte {
	img := image.NewRGBA(image.Rect(0, 0, 8, 8))
	for x := range 8 {
		for y := range 8 {
			img.Set(x, y, color.RGBA{R: 200, G: 40, B: 40, A: 255})
		}
	}
	var b bytes.Buffer
	png.Encode(&b, img)
	return b.Bytes()
}

func liveGenerate(t *testing.T, a Adapter, key string) {
	m := a.Live.Model
	base := a.Live.Base
	var name string
	if *record {
		base = recorder(t, a.Live.Base, &name)
	}
	cheap := a.Live.Params
	cheap.Key = key
	turn := func(t *testing.T, m api.Model, p Params, req api.Request) ([]api.Event, []api.Content) {
		t.Helper()
		name = strings.NewReplacer("/", "-", " ", "-").Replace(t.Name())
		events := collect(t, a.build(t, base, m, p), req)
		if a.Unframed {
			CheckUnframed(t, events)
		} else {
			CheckGrammar(t, events)
		}
		b := response.NewBulder()
		for _, e := range events {
			b.On(e)
		}
		return events, b.Flush().Content.Ordered()
	}
	ended := func(t *testing.T, events []api.Event) {
		t.Helper()
		switch e := events[len(events)-1].(type) {
		case api.EventOk:
		case api.EventErr:
			if e.Code != api.CodeCapped {
				t.Fatalf("turn failed: %s", Describe(e))
			}
		default:
			t.Fatalf("turn ended with no terminal event: %v", DescribeAll(events))
		}
	}
	t.Run("text", func(t *testing.T) {
		events, _ := turn(t, m, cheap, api.Request{Messages: []api.Message{user(text("Say hi in one word."))}})
		ended(t, events)
	})
	t.Run("multiturn", func(t *testing.T) {
		events, _ := turn(t, m, cheap, entry(t, m.Name, "multiturn"))
		ended(t, events)
	})
	t.Run("image", func(t *testing.T) {
		if m.ImagesPerRequest == 0 {
			t.Skip("the model declares no image input")
		}
		events, _ := turn(t, m, cheap, api.Request{Messages: []api.Message{user(text("What colour is this square? One word."), api.ContentImage{Data: tinyPNG(), Mime: "image/png"})}})
		ended(t, events)
	})
	t.Run("tools-text-turns", func(t *testing.T) {
		events, _ := turn(t, m, cheap, entry(t, m.Name, "tools-text-turns"))
		ended(t, events)
	})
	t.Run("tool-replay", func(t *testing.T) {
		req := api.Request{Tools: tools(), Messages: []api.Message{user(text("Call the lookup tool with q set to cat. Do not answer before calling it."))}}
		events, content := turn(t, m, cheap, req)
		ended(t, events)
		var results []api.Content
		for _, c := range content {
			if u, ok := c.(api.ContentToolUse); ok {
				results = append(results, result(u.ID, api.ToolOk, text("a small feline")))
			}
		}
		if len(results) == 0 {
			t.Skip("the model answered without calling the tool")
		}
		req.Messages = append(req.Messages, api.Message{Role: api.RoleAssistant, Content: content}, api.Message{Role: api.RoleUser, Content: results})
		events, _ = turn(t, m, cheap, req)
		ended(t, events)
	})
	t.Run("invalid", func(t *testing.T) {
		bad := api.Model{Name: "no-such-model-conform"}
		events, _ := turn(t, bad, cheap, entry(t, m.Name, "text"))
		e, ok := events[len(events)-1].(api.EventErr)
		if !ok || (e.Code != api.CodeInvalid && e.Code != api.CodeAccess) {
			t.Fatalf("an unknown model ended %s, want EventErr %s", Describe(events[len(events)-1]), api.CodeInvalid)
		}
	})
}

var scrubbed = regexp.MustCompile(`"(signature|encrypted_content)":"[^"]*"`)

func recorder(t *testing.T, base string, name *string) string {
	target, err := url.Parse(base)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join("testdata", "streams"), 0o755); err != nil {
		t.Fatal(err)
	}
	proxy := &httputil.ReverseProxy{
		Rewrite: func(r *httputil.ProxyRequest) {
			r.SetURL(target)
		},
		ModifyResponse: func(resp *http.Response) error {
			if !strings.HasPrefix(resp.Header.Get("Content-Type"), "text/event-stream") {
				return nil
			}
			data, err := io.ReadAll(resp.Body)
			resp.Body.Close()
			if err != nil {
				return err
			}
			resp.Body = io.NopCloser(bytes.NewReader(data))
			clean := scrubbed.ReplaceAll(data, []byte(`"$1":"c2NydWJiZWQ="`))
			return os.WriteFile(filepath.Join("testdata", "streams", *name+".sse"), clean, 0o644)
		},
	}
	srv := httptest.NewServer(proxy)
	t.Cleanup(srv.Close)
	return srv.URL
}
