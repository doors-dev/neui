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

package conform

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/internal/thread/response"
)

func Run(t *testing.T, a Adapter) {
	sch, err := loadSpec(a.Spec.File, a.Spec.LooseOneOf, a.Spec.Accept)
	if err != nil {
		t.Fatal(err)
	}
	limits := loadLimits(t, a.Name)
	srv := newServer(t)
	for _, c := range []struct {
		name string
		run  func(t *testing.T, a Adapter, sch *schemas, srv *server, limits limitsFile)
	}{
		{"Spec", checkSpec},
		{"Sources", checkSources},
		{"Limits", checkLimits},
		{"Encode", checkEncode},
		{"Corpus", checkCorpus},
		{"RequestBytes", checkRequestBytes},
		{"Decode", checkDecode},
		{"RoundTrip", checkRoundTrip},
		{"Errors", checkErrors},
		{"Streams", checkStreams},
	} {
		t.Run(c.name, func(t *testing.T) { c.run(t, a, sch, srv, limits) })
	}
}

var (
	scriptText     = Script{Name: "text", Blocks: []Block{{Kind: KindText, Text: "Hello there."}}}
	scriptTool     = Script{Name: "tool", Blocks: []Block{{Kind: KindText, Text: "Checking."}, {Kind: KindTool, ID: "call_7", Name: "lookup", Args: `{"q":"cat"}`}}}
	scriptThinking = Script{Name: "thinking", Blocks: []Block{{Kind: KindThinking, Text: "The user wants a lookup.", Signature: "c2lnbmF0dXJlLTE="}, {Kind: KindTool, ID: "call_8", Name: "lookup", Args: `{"q":"dog"}`}}}
	scriptSummary  = Script{Name: "summary", Blocks: []Block{{Kind: KindSummary, Text: "Thought about the time.", Signature: "c2lnbmF0dXJlLTI="}, {Kind: KindTool, ID: "call_9", Name: "clock", Args: `{}`}}}
	scriptOpaque   = Script{Name: "opaque", Blocks: []Block{{Kind: KindOpaque}, {Kind: KindText, Text: "Found it."}}}
)

func Scripts() []Script {
	return []Script{
		scriptText,
		scriptTool,
		scriptThinking,
		scriptSummary,
		scriptOpaque,
		{Name: "two-tools", Blocks: []Block{{Kind: KindTool, ID: "call_1", Name: "lookup", Args: `{"q":"a"}`}, {Kind: KindTool, ID: "call_2", Name: "clock", Args: `{}`}}},
		{Name: "capped", Blocks: []Block{{Kind: KindText, Text: "Partial"}}, Stop: api.CodeCapped},
		{Name: "refused", Stop: api.CodeRefused},
		{Name: "context", Blocks: []Block{{Kind: KindText, Text: "Partial"}}, Stop: api.CodeContextLength},
		{Name: "aborted", Blocks: []Block{{Kind: KindText, Text: "Partial"}}, Stop: api.CodeNetwork},
		{Name: "busy", Stop: api.CodeBusy},
	}
}

const (
	UsageInput  = 11
	UsageOutput = 7
)

func checkSpec(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	if got := sch.sum(); got != sch.pin.Sha256 {
		t.Fatalf("%s decompresses to sha256 %s, SPECS pins %s: specs are replaced only by fetchspec together with SPECS", a.Spec.File, got, sch.pin.Sha256)
	}
	if a.Spec.Version == "" {
		return
	}
	m := a.Models[0]
	srv.reset(reply{body: mustRender(t, a, m, scriptText)})
	collect(t, a.build(t, srv.srv.URL, m, Params{}), Corpus(m.Name)[0].Request)
	_, h := srv.last()
	if h == nil {
		t.Fatal("no request reached the server")
	}
	if got := h.Get(a.Spec.Version); got != sch.pin.APIVersion {
		t.Fatalf("adapter sends %s %q, the pinned spec %s describes %q: pin the spec of the version sent, or change the version", a.Spec.Version, got, a.Spec.File, sch.pin.APIVersion)
	}
}

func checkSources(t *testing.T, a Adapter, _ *schemas, _ *server, _ limitsFile) {
	for i, r := range a.Rules {
		if !strings.HasPrefix(r.Source, "https://") {
			t.Errorf("rule %d (model %q entry %q) has no doc URL as its source: %q", i, r.Model, r.Entry, r.Source)
		}
	}
	for _, e := range a.Errors {
		if !strings.HasPrefix(e.Source, "https://") {
			t.Errorf("error case %s has no doc URL as its source: %q", e.Name, e.Source)
		}
	}
	for _, x := range a.Spec.Accept {
		if x.Source == "" {
			t.Errorf("the spec deviation at %s gives no source for what the API accepts", x.At)
		}
	}
}

func mustRender(t *testing.T, a Adapter, m api.Model, s Script) string {
	t.Helper()
	sse, ok := a.Render(m, s)
	if !ok {
		t.Fatalf("adapter cannot render the %s script", s.Name)
	}
	return sse
}

func checkEncode(t *testing.T, a Adapter, sch *schemas, srv *server, limits limitsFile) {
	for _, m := range a.Models {
		probe := entry(t, m.Name, ProbeEntry)
		ok := mustRender(t, a, m, scriptText)
		t.Run(m.Name, func(t *testing.T) {
			for _, p := range Matrix(limits.Models[m.Name].MaxOutput, a.Sampling) {
				t.Run(p.String(), func(t *testing.T) {
					ex := a.expect(m.Name, ProbeEntry, p)
					if ex.bug != "" {
						t.Skip("bug: " + ex.bug)
					}
					srv.reset(reply{body: ok})
					provider, err := a.New(srv.srv.URL, m, p)
					if ex.want == RefuseNew {
						if err == nil {
							t.Fatalf("New accepted the settings, want it refused (%s)", strings.Join(ex.source, ", "))
						}
						return
					}
					if err != nil {
						t.Fatalf("New refused the settings and no rule says it does: %v", err)
					}
					events := collect(t, provider, p.request(probe))
					checkOutcome(t, a, sch, srv, ex, events, &p)
				})
			}
		})
	}
}

func (a Adapter) build(t *testing.T, base string, m api.Model, p Params) api.Provider {
	t.Helper()
	provider, err := a.New(base, m, p)
	if err != nil {
		t.Fatalf("New for %s with %s: %v", m.Name, p, err)
	}
	return provider
}

func entry(t *testing.T, model, name string) api.Request {
	t.Helper()
	for _, e := range Corpus(model) {
		if e.Name == name {
			return e.Request
		}
	}
	t.Fatalf("no corpus entry %s", name)
	return api.Request{}
}

func checkOutcome(t *testing.T, a Adapter, sch *schemas, srv *server, ex expect, events []api.Event, p *Params) []byte {
	t.Helper()
	if ex.want == RefuseLocal {
		if srv.hits() != 0 {
			t.Fatalf("request reached the API, want it refused before sending (%s)", strings.Join(ex.source, ", "))
		}
		if len(events) != 1 {
			t.Fatalf("got %v, want one EventErr", DescribeAll(events))
		}
		if e, is := events[0].(api.EventErr); !is || e.Code != api.CodeInvalid || e.Err == nil {
			t.Fatalf("got %s, want EventErr %s", Describe(events[0]), api.CodeInvalid)
		}
		return nil
	}
	body, _ := srv.last()
	if srv.hits() != 1 {
		t.Fatalf("request made %d calls, want it sent once (%s): %v", srv.hits(), ex.want, DescribeAll(events))
	}
	if err := sch.validate(a.Spec.Request, body, a.Spec.Private, a.Spec.Defaults); err != nil {
		t.Fatalf("body does not match %s in %s: %v\nbody: %s", a.Spec.Request, a.Spec.File, err, clip(body))
	}
	for ptr, want := range ex.carry {
		got, ok := lookup(body, ptr)
		if !ok || !sameJSON(got, want) {
			t.Fatalf("body carries %s = %v, want %v (%s)\nbody: %s", ptr, got, want, strings.Join(ex.source, ", "), clip(body))
		}
	}
	for _, ptr := range ex.omit {
		if got, ok := lookup(body, ptr); ok {
			t.Fatalf("body carries %s = %v, want it omitted (%s)\nbody: %s", ptr, got, strings.Join(ex.source, ", "), clip(body))
		}
	}
	if p != nil {
		for name, want := range p.wireValues() {
			ptr, mapped := a.Wire[name]
			if !mapped || ex.omits(ptr) {
				continue
			}
			got, ok := lookup(body, ptr)
			if !ok {
				t.Fatalf("%s was set and is not on the wire at %s, and no rule says it is omitted\nbody: %s", name, ptr, clip(body))
			}
			if want != nil && !sameJSON(got, want) {
				t.Fatalf("%s was set to %v and goes on the wire as %v at %s", name, want, got, ptr)
			}
		}
	}
	CheckGrammar(t, events)
	if len(events) == 0 {
		t.Fatal("no events")
	}
	if _, ok := events[len(events)-1].(api.EventOk); !ok {
		t.Fatalf("stream ended %s, want EventOk", Describe(events[len(events)-1]))
	}
	return body
}

func clip(b []byte) string {
	if len(b) > 2000 {
		return string(b[:2000]) + "…"
	}
	return string(b)
}

func checkCorpus(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	for i, m := range a.Models {
		ok := mustRender(t, a, m, scriptText)
		t.Run(m.Name, func(t *testing.T) {
			for _, e := range Corpus(m.Name) {
				t.Run(e.Name, func(t *testing.T) {
					ex := a.expect(m.Name, e.Name, Params{})
					if ex.bug != "" {
						t.Skip("bug: " + ex.bug)
					}
					srv.reset(reply{body: ok})
					events := collect(t, a.build(t, srv.srv.URL, m, Params{}), e.Request)
					body := checkOutcome(t, a, sch, srv, ex, events, nil)
					if i != 0 || body == nil {
						return
					}
					var pretty bytes.Buffer
					if err := json.Indent(&pretty, body, "", "  "); err != nil {
						t.Fatal(err)
					}
					pretty.WriteByte('\n')
					testkit.Golden(t, pretty.Bytes())
				})
			}
		})
	}
}

func checkRequestBytes(t *testing.T, a Adapter, _ *schemas, srv *server, _ limitsFile) {
	m := a.Models[0]
	req := entry(t, m.Name, ProbeEntry)
	ok := mustRender(t, a, m, scriptText)
	srv.reset(reply{body: ok})
	collect(t, a.build(t, srv.srv.URL, m, Params{}), req)
	body, _ := srv.last()
	if body == nil {
		t.Fatal("probe was not sent")
	}
	n := len(body)
	at := m
	at.RequestBytes = n
	srv.reset(reply{body: ok})
	events := collect(t, a.build(t, srv.srv.URL, at, Params{}), req)
	if srv.hits() != 1 {
		t.Fatalf("a %d byte body against RequestBytes %d was not sent: %v", n, n, DescribeAll(events))
	}
	under := m
	under.RequestBytes = n - 1
	srv.reset(reply{body: ok})
	events = collect(t, a.build(t, srv.srv.URL, under, Params{}), req)
	if srv.hits() != 0 {
		t.Fatalf("a %d byte body against RequestBytes %d was sent", n, n-1)
	}
	if len(events) != 1 {
		t.Fatalf("got %v, want one EventErr", DescribeAll(events))
	}
	if e, is := events[0].(api.EventErr); !is || e.Code != api.CodeTooLarge {
		t.Fatalf("got %s, want EventErr %s", Describe(events[0]), api.CodeTooLarge)
	}
	srv.reset(reply{status: http.StatusRequestEntityTooLarge, body: "payload too large"})
	events = collect(t, a.build(t, srv.srv.URL, at, Params{}), req)
	CheckGrammar(t, events)
	e, is := events[len(events)-1].(api.EventErr)
	if !is || e.Code != api.CodeTooLarge || e.Err == nil || !strings.Contains(e.Err.Error(), "sent "+strconv.Itoa(n)+" bytes, measured against the "+strconv.Itoa(n)+" declared") {
		t.Fatalf("a 413 on a body measured as fitting ended %s, want %s naming the %d bytes sent and the %d declared", Describe(events[len(events)-1]), api.CodeTooLarge, n, n)
	}
}

func checkDecode(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	m := a.Models[0]
	for _, s := range Scripts() {
		t.Run(s.Name, func(t *testing.T) {
			sse, ok := a.Render(m, s)
			if !ok {
				t.Skipf("the %s wire has no way to say this", a.Name)
			}
			for i, f := range frames(sse) {
				if string(f) == "[DONE]" {
					continue
				}
				if err := sch.validate(a.Spec.Event, f, a.Spec.EventPrivate, nil); err != nil {
					t.Fatalf("frame %d does not match %s: %v\nframe: %s", i, a.Spec.Event, err, f)
				}
			}
			testkit.Golden(t, []byte(sse))
			srv.reset(reply{body: sse})
			events := collect(t, a.build(t, srv.srv.URL, m, Params{}), entry(t, m.Name, "text"))
			content := checkScript(t, a, s, events)
			if len(content) != len(s.Blocks) {
				t.Fatalf("assembled %d blocks, script has %d: %v", len(content), len(s.Blocks), DescribeAll(events))
			}
		})
	}
}

func checkScript(t *testing.T, a Adapter, s Script, events []api.Event) []api.Content {
	t.Helper()
	if a.Unframed {
		CheckUnframed(t, events)
	} else {
		CheckGrammar(t, events)
	}
	if len(events) == 0 {
		t.Fatal("no events")
	}
	last := events[len(events)-1]
	if s.Stop == "" {
		if _, ok := last.(api.EventOk); !ok {
			t.Fatalf("ended %s, want EventOk\n%s", Describe(last), strings.Join(DescribeAll(events), "\n"))
		}
	} else if e, ok := last.(api.EventErr); !ok || e.Code != s.Stop {
		t.Fatalf("ended %s, want EventErr %s", Describe(last), s.Stop)
	}
	var usage *api.EventUsage
	b := response.NewBulder()
	for _, e := range events {
		if u, ok := e.(api.EventUsage); ok {
			usage = &u
		}
		if _, ok := b.On(e); !ok {
			t.Fatalf("builder refused %s", Describe(e))
		}
	}
	if usage == nil || usage.Input != UsageInput || usage.Output != UsageOutput {
		t.Fatalf("usage %+v, want input %d output %d", usage, UsageInput, UsageOutput)
	}
	content := b.Flush().Content.Ordered()
	for i, want := range s.Blocks {
		if i >= len(content) {
			break
		}
		switch c := content[i].(type) {
		case api.ContentText:
			if want.Kind != KindText || c.Text != want.Text {
				t.Fatalf("block %d is text %q, want %s %q", i, c.Text, want.Kind, want.Text)
			}
		case api.ContentReasoning:
			got := strings.Join(c.Thoughts.Ordered(), "")
			if want.Kind == KindSummary {
				got = strings.Join(c.Summary.Ordered(), "")
			}
			if (want.Kind != KindThinking && want.Kind != KindSummary) || got != want.Text {
				t.Fatalf("block %d is reasoning %q (thoughts %v summary %v), want %s %q", i, got, c.Thoughts, c.Summary, want.Kind, want.Text)
			}
			if a.Signed && want.Signature != "" && !bytes.Contains(c.Meta, []byte(want.Signature)) {
				t.Fatalf("block %d meta %s lost the signature %q", i, c.Meta, want.Signature)
			}
		case api.ContentToolUse:
			if want.Kind != KindTool || c.ID != want.ID || c.Name != want.Name || !sameJSON(json.RawMessage(c.Input), json.RawMessage(want.Args)) {
				t.Fatalf("block %d is tool %s %s %s, want %s %s %s", i, c.ID, c.Name, c.Input, want.ID, want.Name, want.Args)
			}
		case api.ContentOpaque:
			if want.Kind != KindOpaque || c.Kind == "" || len(c.Data) == 0 {
				t.Fatalf("block %d is opaque %q %s, want %s", i, c.Kind, c.Data, want.Kind)
			}
		default:
			t.Fatalf("block %d is %T", i, c)
		}
	}
	return content
}

func checkRoundTrip(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	m := a.Models[0]
	for _, s := range []Script{scriptThinking, scriptSummary, scriptOpaque} {
		t.Run(s.Name, func(t *testing.T) {
			sse, ok := a.Render(m, s)
			if !ok {
				t.Skipf("the %s wire has no way to say this", a.Name)
			}
			srv.reset(reply{body: sse})
			events := collect(t, a.build(t, srv.srv.URL, m, Params{}), entry(t, m.Name, "text"))
			content := checkScript(t, a, s, events)
			replyText := mustRender(t, a, m, scriptText)
			send := func(content []api.Content) []byte {
				t.Helper()
				req := api.Request{Tools: tools(), Messages: []api.Message{user(text("look up dog")), {Role: api.RoleAssistant, Content: content}}}
				var results []api.Content
				for _, c := range content {
					if u, is := c.(api.ContentToolUse); is {
						results = append(results, result(u.ID, api.ToolOk, text("a loyal canine")))
					}
				}
				if len(results) == 0 {
					results = append(results, text("thanks"))
				}
				req.Messages = append(req.Messages, api.Message{Role: api.RoleUser, Content: results})
				srv.reset(reply{body: replyText})
				events := collect(t, a.build(t, srv.srv.URL, m, Params{}), req)
				body, _ := srv.last()
				if body == nil {
					t.Fatalf("replay was not sent: %v", DescribeAll(events))
				}
				if err := sch.validate(a.Spec.Request, body, a.Spec.Private, a.Spec.Defaults); err != nil {
					t.Fatalf("replay body does not match %s: %v\nbody: %s", a.Spec.Request, err, clip(body))
				}
				return body
			}
			body := send(content)
			for i, c := range content {
				for _, want := range replayed(c, s.Blocks[i]) {
					if !bytes.Contains(body, want) {
						t.Fatalf("block %d did not replay byte for byte: %s is not in\n%s", i, want, clip(body))
					}
				}
			}
		})
	}
}

func replayed(c api.Content, b Block) [][]byte {
	switch c := c.(type) {
	case api.ContentReasoning:
		if b.Signature != "" && len(c.Meta) > 0 {
			return [][]byte{[]byte(b.Signature)}
		}
		quoted, _ := json.Marshal(b.Text)
		return [][]byte{quoted}
	case api.ContentOpaque:
		return [][]byte{c.Data}
	}
	return nil
}

func checkErrors(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	m := a.Models[0]
	for _, e := range a.Errors {
		t.Run(e.Name, func(t *testing.T) {
			if e.Schema {
				if err := sch.validate(a.Spec.Error, []byte(e.Body), nil, nil); err != nil {
					t.Fatalf("fixture body does not match %s: %v", a.Spec.Error, err)
				}
			}
			rep := reply{status: e.Status, header: e.Header, body: e.Body}
			if e.Stream != "" {
				rep = reply{header: e.Header, body: e.Stream}
			}
			srv.reset(rep)
			events := collect(t, a.build(t, srv.srv.URL, m, Params{}), entry(t, m.Name, "text"))
			CheckGrammar(t, events)
			if len(events) == 0 {
				t.Fatal("no events")
			}
			got, ok := events[len(events)-1].(api.EventErr)
			if !ok || got.Code != e.Want {
				t.Fatalf("ended %s, want EventErr %s (%s)", Describe(events[len(events)-1]), e.Want, e.Source)
			}
		})
	}
}

func checkStreams(t *testing.T, a Adapter, sch *schemas, srv *server, _ limitsFile) {
	files, _ := filepath.Glob(filepath.Join("testdata", "streams", "*.sse"))
	if len(files) == 0 {
		t.Skip("no recorded streams in testdata/streams; record them with go test -tags live -run Live -record")
	}
	m := a.Models[0]
	for _, file := range files {
		t.Run(filepath.Base(file), func(t *testing.T) {
			data, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for i, f := range frames(string(data)) {
				if string(f) == "[DONE]" {
					continue
				}
				if err := sch.validate(a.Spec.Event, f, a.Spec.EventPrivate, nil); err != nil {
					t.Fatalf("frame %d does not match %s: %v", i, a.Spec.Event, err)
				}
			}
			srv.reset(reply{body: string(data)})
			events := collect(t, a.build(t, srv.srv.URL, m, Params{}), entry(t, m.Name, "text"))
			if a.Unframed {
				CheckUnframed(t, events)
			} else {
				CheckGrammar(t, events)
			}
			switch events[len(events)-1].(type) {
			case api.EventOk, api.EventErr:
			default:
				t.Fatalf("recorded stream ended with no terminal event: %v", DescribeAll(events))
			}
		})
	}
}

type modelLimits struct {
	Context          int    `json:"context"`
	ImagePixels      int    `json:"imagePixels"`
	ImageBytes       int    `json:"imageBytes"`
	ImageEncoding    string `json:"imageEncoding"`
	ImagesPerRequest int    `json:"imagesPerRequest"`
	RequestBytes     int    `json:"requestBytes"`
	MaxOutput        int    `json:"maxOutput"`
	Note             string `json:"note"`
}

type limitsFile struct {
	AsOf    string                 `json:"asOf"`
	Sources map[string]string      `json:"sources"`
	Models  map[string]modelLimits `json:"models"`
}

func loadLimits(t *testing.T, name string) limitsFile {
	t.Helper()
	data, err := fs.ReadFile(testdata, "testdata/limits/"+name+".json")
	if err != nil {
		t.Fatal(err)
	}
	var l limitsFile
	if err := json.Unmarshal(data, &l); err != nil {
		t.Fatalf("limits/%s.json: %v", name, err)
	}
	return l
}

const dataURLPrefix = len("data:image/jpeg;base64,")

func checkLimits(t *testing.T, a Adapter, _ *schemas, _ *server, l limitsFile) {
	for _, field := range []string{"context", "imagePixels", "imageBytes", "imagesPerRequest", "requestBytes", "maxOutput"} {
		if !strings.HasPrefix(l.Sources[field], "https://") {
			t.Errorf("limits/%s.json has no doc URL for %s", a.Name, field)
		}
	}
	var declared []string
	for _, m := range a.Models {
		declared = append(declared, m.Name)
		doc, ok := l.Models[m.Name]
		if !ok {
			t.Errorf("%s is declared and limits/%s.json has no entry for it", m.Name, a.Name)
			continue
		}
		check := func(field string, got, want int) {
			if got != want {
				t.Errorf("%s %s = %d, the docs (as of %s, %s) say %d", m.Name, field, got, l.AsOf, l.Sources[field], want)
			}
		}
		check("Context", m.Context, doc.Context)
		check("ImagePixels", m.ImagePixels, doc.ImagePixels)
		check("ImagesPerRequest", m.ImagesPerRequest, doc.ImagesPerRequest)
		if m.RequestBytes <= 0 || m.RequestBytes > doc.RequestBytes {
			t.Errorf("%s RequestBytes = %d, the docs (as of %s, %s) say at most %d", m.Name, m.RequestBytes, l.AsOf, l.Sources["requestBytes"], doc.RequestBytes)
		}
		enc := base64.StdEncoding.EncodedLen
		switch doc.ImageEncoding {
		case "":
			check("ImageBytes", m.ImageBytes, doc.ImageBytes)
		case "base64":
			if enc(m.ImageBytes) > doc.ImageBytes || enc(m.ImageBytes+3) <= doc.ImageBytes {
				t.Errorf("%s ImageBytes = %d raw encodes to %d, the docs cap one image at %d base64 (%s)", m.Name, m.ImageBytes, enc(m.ImageBytes), doc.ImageBytes, l.Sources["imageBytes"])
			}
		case "data-url":
			if enc(m.ImageBytes)+dataURLPrefix > doc.ImageBytes || enc(m.ImageBytes+3)+dataURLPrefix <= doc.ImageBytes {
				t.Errorf("%s ImageBytes = %d raw is a %d character data URL, the docs cap it at %d (%s)", m.Name, m.ImageBytes, enc(m.ImageBytes)+dataURLPrefix, doc.ImageBytes, l.Sources["imageBytes"])
			}
		default:
			t.Errorf("%s: unknown image encoding %q", m.Name, doc.ImageEncoding)
		}
	}
	for name := range l.Models {
		if !slices.Contains(declared, name) {
			t.Errorf("limits/%s.json documents %s, which the adapter does not declare", a.Name, name)
		}
	}
}
