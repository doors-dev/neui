package conform

import (
	"errors"
	"runtime"
	"strings"
	"testing"

	"neui.dev/api"
)

type fakeTB struct {
	testing.TB
	failed string
}

func (r *fakeTB) Helper() {}

func (r *fakeTB) Fatalf(format string, args ...any) {
	r.failed = format
	runtime.Goexit()
}

func grammarFails(events []api.Event, unframed bool) bool {
	r := &fakeTB{}
	done := make(chan struct{})
	go func() {
		defer close(done)
		walkGrammar(r, events, unframed)
	}()
	<-done
	return r.failed != ""
}

func TestGrammar(t *testing.T) {
	err := errors.New("x")
	for _, c := range []struct {
		name     string
		events   []api.Event
		unframed bool
		bad      bool
	}{
		{"ok", []api.Event{api.EventStart{}, api.EventText{Index: 0, Text: "a"}, api.EventClose{Index: 0}, api.EventUsage{}, api.EventOk{}}, false, false},
		{"cut short", []api.Event{api.EventStart{}, api.EventText{Index: 0, Text: "a"}}, false, false},
		{"interleaved", []api.Event{api.EventThinking{Index: 0}, api.EventTool{Index: 1}, api.EventThinking{Index: 0}, api.EventTool{Index: 1}}, false, false},
		{"meta opens", []api.Event{api.EventMeta{Index: 0}, api.EventText{Index: 0}}, false, false},
		{"second start", []api.Event{api.EventStart{}, api.EventStart{}}, false, true},
		{"after end", []api.Event{api.EventOk{}, api.EventText{Index: 0}}, false, true},
		{"err without cause", []api.Event{api.EventErr{Code: api.CodeBusy}}, false, true},
		{"hole", []api.Event{api.EventText{Index: 1}}, false, true},
		{"edit closed", []api.Event{api.EventText{Index: 0}, api.EventClose{Index: 0}, api.EventText{Index: 0}}, false, true},
		{"close unopened", []api.Event{api.EventText{Index: 0}, api.EventClose{Index: 1}}, false, true},
		{"kind change", []api.Event{api.EventText{Index: 0}, api.EventTool{Index: 0}}, false, true},
		{"meta on opaque", []api.Event{api.EventOpaque{Index: 0}, api.EventMeta{Index: 0}}, false, true},
		{"opaque after meta", []api.Event{api.EventMeta{Index: 0}, api.EventOpaque{Index: 0}}, false, true},
		{"close on unframed wire", []api.Event{api.EventText{Index: 0}, api.EventClose{Index: 0}}, true, true},
		{"err ends", []api.Event{api.EventText{Index: 0}, api.EventErr{Code: api.CodeBusy, Err: err}}, true, false},
	} {
		if got := grammarFails(c.events, c.unframed); got != c.bad {
			t.Errorf("%s: refused %v, want %v", c.name, got, c.bad)
		}
	}
}

func TestSpecsArePinned(t *testing.T) {
	for _, file := range []string{"anthropic.json.gz", "openai.json.gz"} {
		s, err := loadSpec(file, false)
		if err != nil {
			t.Fatal(err)
		}
		if s.sum() != s.pin.Sha256 {
			t.Errorf("%s: sha256 %s, SPECS pins %s", file, s.sum(), s.pin.Sha256)
		}
		if !strings.Contains(s.pin.Source, s.pin.Commit) {
			t.Errorf("%s: source %s is not at the pinned commit %s", file, s.pin.Source, s.pin.Commit)
		}
	}
}

func TestValidatorRefuses(t *testing.T) {
	ant, err := loadSpec("anthropic.json.gz", false)
	if err != nil {
		t.Fatal(err)
	}
	oai, err := loadSpec("openai.json.gz", true)
	if err != nil {
		t.Fatal(err)
	}
	const msgs = `"messages":[{"role":"user","content":[{"type":"text","text":"hi"}]}]`
	for _, c := range []struct {
		name string
		s    *schemas
		ptr  string
		body string
		ok   bool
	}{
		{"anthropic minimal", ant, "/components/schemas/CreateMessageParams", `{"model":"m","max_tokens":1,"stream":true,` + msgs + `}`, true},
		{"anthropic sampling at 2023-06-01", ant, "/components/schemas/CreateMessageParams", `{"model":"m","max_tokens":1,"temperature":0.5,"top_p":0.9,` + msgs + `}`, true},
		{"anthropic unknown field", ant, "/components/schemas/CreateMessageParams", `{"model":"m","max_tokens":1,"bogus":1,` + msgs + `}`, false},
		{"anthropic bad block", ant, "/components/schemas/CreateMessageParams", `{"model":"m","max_tokens":1,"messages":[{"role":"user","content":[{"type":"bogus"}]}]}`, false},
		{"anthropic bad effort", ant, "/components/schemas/CreateMessageParams", `{"model":"m","max_tokens":1,"output_config":{"effort":"bogus"},` + msgs + `}`, false},
		{"anthropic count refuses max_tokens", ant, "/components/schemas/CountMessageTokensParams", `{"model":"m","max_tokens":1,` + msgs + `}`, false},
		{"openai minimal", oai, "/components/schemas/CreateResponse", `{"model":"m","input":"hi"}`, true},
		{"openai bad effort", oai, "/components/schemas/CreateResponse", `{"model":"m","input":"hi","reasoning":{"effort":"bogus"}}`, false},
		{"openai max_output_tokens floor", oai, "/components/schemas/CreateResponse", `{"model":"m","input":"hi","max_output_tokens":1}`, false},
		{"chat bad role", oai, "/components/schemas/CreateChatCompletionRequest", `{"model":"m","messages":[{"role":"bogus","content":"hi"}]}`, false},
	} {
		err := c.s.validate(c.ptr, []byte(c.body), nil, nil)
		if (err == nil) != c.ok {
			t.Errorf("%s: valid %v, want %v: %v", c.name, err == nil, c.ok, err)
		}
	}
}

func TestPointers(t *testing.T) {
	body := []byte(`{"a":{"b":[{"c":1},{"c":2}]}}`)
	if v, ok := lookup(body, "/a/b/1/c"); !ok || !sameJSON(v, 2) {
		t.Fatalf("lookup = %v %v", v, ok)
	}
	if _, ok := lookup(body, "/a/x"); ok {
		t.Fatal("lookup found a missing key")
	}
	var v any = map[string]any{"a": map[string]any{"b": []any{map[string]any{"c": 1}, map[string]any{"d": 2}}}}
	v = deletePointer(v, splitPointer("/a/b/*/c"))
	fillPointer(v, splitPointer("/a/b/*/d"), 9)
	if !sameJSON(v, map[string]any{"a": map[string]any{"b": []any{map[string]any{"d": 9}, map[string]any{"d": 2}}}}) {
		t.Fatalf("got %v", v)
	}
}

func TestExpectPrecedence(t *testing.T) {
	a := Adapter{Rules: []Rule{
		{Want: Remote, Source: "https://a"},
		{Model: "m", When: Params.Sampling, Want: RefuseLocal, Source: "https://b"},
		{Entry: "e", Omit: []string{"/x"}, Source: "https://c"},
	}}
	if e := a.expect("m", "e", Params{Temperature: 0.5}); e.want != RefuseLocal || !e.omits("/x") {
		t.Fatalf("got %+v", e)
	}
	if e := a.expect("other", "f", Params{Temperature: 0.5}); e.want != Remote || e.omits("/x") {
		t.Fatalf("got %+v", e)
	}
	if n := len(Matrix(10)); n != 5*5*4*4 {
		t.Fatalf("matrix has %d cells", n)
	}
	if n := len(Matrix(0)); n != 5*5*4*2 {
		t.Fatalf("matrix without a ceiling has %d cells", n)
	}
}
