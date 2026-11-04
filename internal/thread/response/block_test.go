package response

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"neui.dev/api"
)

func TestToolArgumentsThatNeverParseAssembleDiscarded(t *testing.T) {
	b := NewBulder()
	events := []api.Event{
		api.EventStart{ID: "r1", Model: "m"},
		api.EventTool{Index: 0, ID: "call_a", Name: "search", Args: `{"query": "San Franc`},
		api.EventTool{Index: 1, ID: "call_b", Name: "list", Args: `{}`},
		api.EventTool{Index: 2, ID: "call_c", Name: "now"},
		api.EventClose{Index: 0},
		api.EventClose{Index: 1},
		api.EventClose{Index: 2},
		api.EventOk{Code: api.CodeTurn},
	}
	for _, e := range events {
		if _, ok := b.On(e); !ok {
			t.Fatalf("%T was rejected", e)
		}
	}
	resp := b.Flush()
	if code, ok := resp.Ok(); !ok || code != api.CodeTurn {
		t.Fatalf("response code %v", resp.code)
	}
	fragment, ok := resp.Content[0].(api.ContentToolUse)
	if !ok {
		t.Fatalf("block 0 assembled as %#v", resp.Content[0])
	}
	if !bytes.Equal(fragment.Input, api.DiscardedArguments) {
		t.Fatalf("a fragment of JSON assembled Input %q, want the discarded marker", fragment.Input)
	}
	empty, ok := resp.Content[1].(api.ContentToolUse)
	if !ok {
		t.Fatalf("block 1 assembled as %#v", resp.Content[1])
	}
	if string(empty.Input) != `{}` {
		t.Fatalf("a call that takes no arguments assembled Input %q, want %q", empty.Input, `{}`)
	}
	none, ok := resp.Content[2].(api.ContentToolUse)
	if !ok {
		t.Fatalf("block 2 assembled as %#v", resp.Content[2])
	}
	if none.Input != nil {
		t.Fatalf("a call sent no arguments at all assembled Input %q, want nil", none.Input)
	}
}

func TestLiveStateDoesNotWarnAboutUnfinishedArguments(t *testing.T) {
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(restore) })

	b := NewBulder()
	for _, e := range []api.Event{
		api.EventStart{ID: "r1", Model: "m"},
		api.EventTool{Index: 0, ID: "call_a", Name: "write", Args: `{"path": "/home/alex/secret`},
	} {
		if _, ok := b.On(e); !ok {
			t.Fatalf("%T was rejected", e)
		}
	}
	if use, ok := b.State()[0].(api.ContentToolUse); !ok || use.Name != "write" {
		t.Fatalf("a live snapshot of a streaming call: %#v", b.State()[0])
	}
	if logged.Len() != 0 {
		t.Fatalf("a page load during a streaming tool call logged %q: the arguments are not final yet", logged.String())
	}

	if _, ok := b.On(api.EventTool{Index: 0, Args: `.txt"}`}); !ok {
		t.Fatal("the rest of the arguments was rejected")
	}
	if _, ok := b.On(api.EventClose{Index: 0}); !ok {
		t.Fatal("close was rejected")
	}
	if _, ok := b.On(api.EventOk{Code: api.CodeTurn}); !ok {
		t.Fatal("ok was rejected")
	}
	resp := b.Flush()
	use, ok := resp.Content[0].(api.ContentToolUse)
	if !ok || string(use.Input) != `{"path": "/home/alex/secret.txt"}` {
		t.Fatalf("the completed call assembled %#v", resp.Content[0])
	}
	if logged.Len() != 0 {
		t.Fatalf("a call that completed with valid JSON logged %q", logged.String())
	}
}

func TestAbortWarnsAboutGenuinelyDiscardedArguments(t *testing.T) {
	var logged bytes.Buffer
	restore := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(restore) })

	b := NewBulder()
	for _, e := range []api.Event{
		api.EventStart{ID: "r1", Model: "m"},
		api.EventTool{Index: 0, ID: "call_a", Name: "write", Args: `{"path": "half`},
	} {
		if _, ok := b.On(e); !ok {
			t.Fatalf("%T was rejected", e)
		}
	}
	_, content := b.Abort()
	if use, ok := content[0].(api.ContentToolUse); !ok || !bytes.Equal(use.Input, api.DiscardedArguments) {
		t.Fatalf("the aborted call assembled %#v", content[0])
	}
	if !strings.Contains(logged.String(), "arguments discarded") {
		t.Fatalf("a user stop discarded the arguments for real and logged %q", logged.String())
	}
}
