package gpt

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// describe renders one event as a line. The grammar is the contract and only a
// sequence assertion tests it, so the tests compare these lines in order rather
// than checking that some event of some kind showed up.
func describe(e api.Event) string {
	switch e := e.(type) {
	case api.EventStart:
		return fmt.Sprintf("start id=%s model=%s", e.ID, e.Model)
	case api.EventText:
		return fmt.Sprintf("text[%d] %q", e.Index, e.Text)
	case api.EventTool:
		return fmt.Sprintf("tool[%d] id=%s name=%s args=%q", e.Index, e.ID, e.Name, e.Args)
	case api.EventThinking:
		return fmt.Sprintf("thinking[%d] part=%d %q", e.Index, e.Part, e.Text)
	case api.EventSummary:
		return fmt.Sprintf("summary[%d] part=%d %q", e.Index, e.Part, e.Text)
	case api.EventMeta:
		return fmt.Sprintf("meta[%d] %s", e.Index, string(e.Data))
	case api.EventOpaque:
		return fmt.Sprintf("opaque[%d] kind=%s data=%s delta=%q", e.Index, e.Kind, string(e.Data), e.Delta)
	case api.EventClose:
		return fmt.Sprintf("close[%d]", e.Index)
	case api.EventUsage:
		return fmt.Sprintf("usage in=%d out=%d reasoning=%d read=%d write=%d",
			e.Input, e.Output, e.Reasoning, e.CacheRead, e.CacheWrite)
	case api.EventOk:
		return fmt.Sprintf("ok %s", e.Code)
	case api.EventErr:
		return fmt.Sprintf("err %s", e.Code)
	default:
		return fmt.Sprintf("?%T", e)
	}
}

func collect(ch <-chan api.Event) []api.Event {
	var out []api.Event
	for e := range ch {
		out = append(out, e)
	}
	return out
}

func describeAll(events []api.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = describe(e)
	}
	return out
}

func drain(ch <-chan api.Event) []string {
	return describeAll(collect(ch))
}

// checkGrammar enforces the grammar in protocol/event.go on a sequence, because
// a fixture that pins the lines an adapter produced says nothing about whether
// the shape is legal:
//
//	stream    := EventStart? body (EventOk | EventErr)?
//	body      := any interleaving of block* and EventUsage*
//	text      := (EventText | EventMeta)+ EventClose?
//	tool      := (EventTool | EventMeta)+ EventClose?
//	reasoning := (EventThinking | EventSummary | EventMeta)+ EventClose?
//	opaque    := EventOpaque+ EventClose?
//
// A block production is the events at one Index rather than a contiguous run,
// so the state here is per index and interleaving passes. EventMeta names no
// kind, so it opens a block and leaves the kind to whatever fills it, and it is
// refused only on an opaque block, whose Data is already entirely the adapter's.
//
// The terminal event is optional and its absence is what says a reply was cut
// off, so what is required is that nothing follows one. A block with no
// EventClose may have ended when the turn did, been cut short, or streamed on a
// wire that never frames a block end, and nothing tells the three apart, so no
// close is required of any block here.
func checkGrammar(t *testing.T, events []api.Event) {
	t.Helper()
	kinds := map[int]string{}
	closed := map[int]bool{}
	highest := -1
	ended := false
	for i, e := range events {
		if ended {
			t.Fatalf("event %d (%s) arrives after the stream already ended\n%s", i, describe(e), strings.Join(describeAll(events), "\n"))
		}
		switch e.(type) {
		case api.EventStart:
			if i != 0 {
				t.Fatalf("event %d: start is only valid first", i)
			}
			continue
		case api.EventOk, api.EventErr:
			ended = true
			continue
		case api.EventUsage:
			continue
		}
		idx, kind := blockOf(e)
		if closed[idx] {
			t.Fatalf("event %d (%s) edits block %d after it closed", i, describe(e), idx)
		}
		if idx > highest {
			if idx != highest+1 {
				t.Fatalf("event %d (%s): index %d leaves a hole after %d, and Index is a position in the message", i, describe(e), idx, highest)
			}
			highest = idx
		}
		switch {
		case kind == "close":
			if kinds[idx] == "" {
				t.Fatalf("event %d closes block %d, which no event ever opened", i, idx)
			}
			closed[idx] = true
		case kind == "meta":
			if kinds[idx] == "opaque" {
				t.Fatalf("event %d attaches meta to opaque block %d, whose Data is already the adapter's", i, idx)
			}
			if kinds[idx] == "" {
				kinds[idx] = "meta"
			}
		case kinds[idx] == "" || kinds[idx] == "meta":
			if kind == "opaque" && kinds[idx] == "meta" {
				t.Fatalf("event %d opens opaque block %d, which already carries meta", i, idx)
			}
			kinds[idx] = kind
		case kinds[idx] != kind:
			t.Fatalf("event %d fills %s block %d with a %s event", i, kinds[idx], idx, kind)
		}
	}
}

func blockOf(e api.Event) (int, string) {
	switch e := e.(type) {
	case api.EventText:
		return e.Index, "text"
	case api.EventTool:
		return e.Index, "tool"
	case api.EventThinking:
		return e.Index, "reasoning"
	case api.EventSummary:
		return e.Index, "reasoning"
	case api.EventMeta:
		return e.Index, "meta"
	case api.EventOpaque:
		return e.Index, "opaque"
	case api.EventClose:
		return e.Index, "close"
	}
	panic(fmt.Sprintf("no block for %T", e))
}

func equalLines(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("event count %d, want %d\n got: %s\nwant: %s",
			len(got), len(want), strings.Join(got, "\n      "), strings.Join(want, "\n      "))
	}
	for i := range got {
		if got[i] != want[i] {
			t.Fatalf("event %d:\n got: %s\nwant: %s\nfull:\n%s", i, got[i], want[i], strings.Join(got, "\n"))
		}
	}
}

// sseServer serves a fixed body as an event stream, one frame at a time.
func sseServer(t *testing.T, body string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("x-request-id", "req_test")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(body)); err != nil {
			return
		}
		w.(http.Flusher).Flush()
	}))
	t.Cleanup(srv.Close)
	return srv
}

// testModel is what every test streams with unless it is about the name itself.
const testModel = "gpt-5.6-sol"

func testProvider(t *testing.T, url string, opts ...Option) *Provider {
	t.Helper()
	return modelProvider(t, testModel, url, opts...)
}

// modelProvider is testProvider for a test that configures the model itself,
// since the model is the Provider's now and no request carries one.
func modelProvider(t *testing.T, model, url string, opts ...Option) *Provider {
	t.Helper()
	all := append([]Option{WithAPIKey("test-key"), WithBaseURL(url)}, opts...)
	p, err := New(Conf{Model: api.Model{Name: model}}, all...)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return p
}

func textRequest() api.Request {
	return api.Request{
		Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: "hi"}}}},
	}
}

// frame builds one SSE frame the way the service writes them, with the event:
// line the decoder ignores in favour of the JSON type.
func frame(typ, data string) string {
	return "event: " + typ + "\ndata: " + data + "\n\n"
}

const usageFrame = `{"type":"response.completed","sequence_number":30,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":{"input_tokens":2000,"input_tokens_details":{"cached_tokens":1500,"cache_write_tokens":200},"output_tokens":1186,"output_tokens_details":{"reasoning_tokens":1024},"total_tokens":3186}}}`

const createdFrame = `{"type":"response.created","sequence_number":0,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"in_progress","usage":null}}`

const inProgressFrame = `{"type":"response.in_progress","sequence_number":1,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"in_progress","usage":null}}`

func TestStreamSequences(t *testing.T) {
	tests := []struct {
		name string
		sse  string
		want []string
	}{
		{
			name: "plain text turn",
			sse: frame("response.created", createdFrame) +
				frame("response.in_progress", inProgressFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","status":"in_progress","role":"assistant","content":[]}}`) +
				frame("response.content_part.added", `{"type":"response.content_part.added","item_id":"msg_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":"","annotations":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":"Hello","obfuscation":"xxxx"}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","item_id":"msg_1","output_index":0,"content_index":0,"delta":" there"}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","item_id":"msg_1","output_index":0,"content_index":0,"text":"Hello there"}`) +
				frame("response.content_part.done", `{"type":"response.content_part.done","item_id":"msg_1","output_index":0,"content_index":0,"part":{"type":"output_text","text":"Hello there","annotations":[]}}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"msg_1","type":"message","status":"completed","role":"assistant","content":[{"type":"output_text","text":"Hello there","annotations":[]}]}}`) +
				frame("response.completed", usageFrame),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "Hello"`,
				`text[0] " there"`,
				"close[0]",
				"usage in=300 out=162 reasoning=1024 read=1500 write=200",
				"ok turn",
			},
		},
		{
			name: "keepalive comments produce no events",
			sse: ": keepalive\n\n" +
				frame("response.created", createdFrame) +
				": keepalive\n" +
				": still here\n\n" +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				": keepalive\n\n" +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"ok"}`) +
				": keepalive\n\n" +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"ok"}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "ok"`,
				"close[0]",
				"ok turn",
			},
		},
		{
			name: "one tool call",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"in_progress","call_id":"call_abc","name":"get_weather","arguments":""}}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"{\"city\":"}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","item_id":"fc_1","output_index":0,"delta":"\"Paris\"}"}`) +
				frame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","item_id":"fc_1","output_index":0,"arguments":"{\"city\":\"Paris\"}"}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_abc","name":"get_weather","arguments":"{\"city\":\"Paris\"}"}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`tool[0] id=call_abc name=get_weather args=""`,
				`tool[0] id= name= args="{\"city\":"`,
				`tool[0] id= name= args="\"Paris\"}"`,
				"close[0]",
				"ok turn",
			},
		},
		{
			name: "several tool calls with interleaved arguments",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_a","name":"alpha","arguments":""}}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"fc_2","type":"function_call","call_id":"call_b","name":"beta","arguments":""}}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"a\":1}"}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","output_index":1,"delta":"{\"b\":"}`) +
				frame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","output_index":0,"arguments":"{\"a\":1}"}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","output_index":1,"delta":"2}"}`) +
				frame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","output_index":1,"arguments":"{\"b\":2}"}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`tool[0] id=call_a name=alpha args=""`,
				`tool[1] id=call_b name=beta args=""`,
				`tool[0] id= name= args="{\"a\":1}"`,
				`tool[1] id= name= args="{\"b\":"`,
				"close[0]",
				`tool[1] id= name= args="2}"`,
				"close[1]",
				"ok turn",
			},
		},
		{
			name: "tool call that takes no arguments gets an empty object",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_z","name":"now","arguments":""}}`) +
				frame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","output_index":0,"arguments":""}`) +
				// The finished item always follows and repeats the arguments the
				// frame before it already declared complete. Nothing is read off
				// it, so the block cannot gain a second fragment or a second
				// close.
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"completed","call_id":"call_z","name":"now","arguments":"{}"}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`tool[0] id=call_z name=now args=""`,
				`tool[0] id= name= args="{}"`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// The service opens a call's arguments with an empty delta. It
			// appends nothing, so it is not a fragment: it proves no arguments
			// were sent, and the object synthesized for a call that takes none
			// still goes out.
			name: "a leading empty argument fragment is not a fragment",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_z","name":"now","arguments":""}}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","output_index":0,"delta":""}`) +
				frame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","output_index":0,"arguments":""}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`tool[0] id=call_z name=now args=""`,
				`tool[0] id= name= args="{}"`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// An empty text fragment appends nothing either, so it opens no
			// block and claims no index: the message that does produce text is
			// the first block of the turn.
			name: "an empty text fragment opens nothing",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":""}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":""}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_2","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"Paris."}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":1,"content_index":0,"text":"Paris."}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "Paris."`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// A call the service named with neither a call_id nor an item id
			// gets a minted handle. The response id is in it because
			// output_index counts within one turn: on that alone, the first call
			// of every turn in a thread would answer to the same string.
			name: "a call named with neither id gets a handle scoped to the turn",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"type":"function_call","name":"now","arguments":""}}`) +
				frame("response.function_call_arguments.done", `{"type":"response.function_call_arguments.done","output_index":0,"arguments":""}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`tool[0] id=resp_1_call_0 name=now args=""`,
				`tool[0] id= name= args="{}"`,
				"close[0]",
				"ok turn",
			},
		},
		{
			name: "tool arguments cut off by the token cap are passed through unrepaired, and the block never closes",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"fc_1","type":"function_call","call_id":"call_cut","name":"search","arguments":""}}`) +
				frame("response.function_call_arguments.delta", `{"type":"response.function_call_arguments.delta","output_index":0,"delta":"{\"query\": \"the hist"}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"fc_1","type":"function_call","status":"incomplete","call_id":"call_cut","name":"search","arguments":"{\"query\": \"the hist"}}`) +
				frame("response.incomplete", `{"type":"response.incomplete","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`tool[0] id=call_cut name=search args=""`,
				`tool[0] id= name= args="{\"query\": \"the hist"`,
				"err capped",
			},
		},
		{
			name: "a reasoning item whose summary streamed keeps its meta without encrypted content",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"summary_index":0,"delta":"Checking."}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking."}]}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`summary[0] part=0 "Checking."`,
				`meta[0] {"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking."}]}`,
				"close[0]",
				"ok turn",
			},
		},
		{
			name: "reasoning streams readable and closes on the item that seals it",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.reasoning_summary_part.added", `{"type":"response.reasoning_summary_part.added","item_id":"rs_1","output_index":0,"summary_index":0,"part":{"type":"summary_text","text":""}}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"summary_index":0,"delta":"Checking "}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"summary_index":0,"delta":"the map."}`) +
				frame("response.reasoning_summary_text.done", `{"type":"response.reasoning_summary_text.done","item_id":"rs_1","output_index":0,"summary_index":0,"text":"Checking the map."}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking the map."}],"encrypted_content":"gAAAAABsealed"}}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"Paris."}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":1,"content_index":0,"text":"Paris."}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`summary[0] part=0 "Checking "`,
				`summary[0] part=0 "the map."`,
				// The meta is one event carrying the whole item, which is what
				// goes back on a later turn, id included.
				`meta[0] {"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking the map."}],"encrypted_content":"gAAAAABsealed"}`,
				"close[0]",
				`text[1] "Paris."`,
				"close[1]",
				"ok turn",
			},
		},
		{
			// The service indexes the summary parts it streams and promises no
			// order between them, so two of them interleave. Each fragment
			// carries the part it belongs to and fills that part alone; nothing
			// is joined and no separator is invented at the alternation.
			name: "interleaved summary parts keep their own slots",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":1,"delta":"Then the "}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"Checking "}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":1,"delta":"ledger."}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","output_index":0,"summary_index":0,"delta":"the map."}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking the map."},{"type":"summary_text","text":"Then the ledger."}],"encrypted_content":"gAAAAABsealed"}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`summary[0] part=1 "Then the "`,
				`summary[0] part=0 "Checking "`,
				`summary[0] part=1 "ledger."`,
				`summary[0] part=0 "the map."`,
				`meta[0] {"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Checking the map."},{"type":"summary_text","text":"Then the ledger."}],"encrypted_content":"gAAAAABsealed"}`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// summary_index and content_index are separate namespaces, so part
			// zero of each is a different run and the two events are what keep
			// them apart. The block opens on whichever arrives first.
			name: "thinking and summary fill one block through separate counters",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.reasoning_text.delta", `{"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":0,"delta":"1071 = 2 x 462 + 147"}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","item_id":"rs_1","output_index":0,"summary_index":0,"delta":"Doing the division."}`) +
				frame("response.reasoning_text.delta", `{"type":"response.reasoning_text.delta","item_id":"rs_1","output_index":0,"content_index":1,"delta":"GCD is 21."}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Doing the division."}],"encrypted_content":"gAAAAABsealed"}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`thinking[0] part=0 "1071 = 2 x 462 + 147"`,
				`summary[0] part=0 "Doing the division."`,
				`thinking[0] part=1 "GCD is 21."`,
				`meta[0] {"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"Doing the division."}],"encrypted_content":"gAAAAABsealed"}`,
				"close[0]",
				"ok turn",
			},
		},
		{
			name: "a reasoning item with nothing readable is the sealed item alone",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAAABsealed"}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`thinking[0] part=0 ""`,
				`meta[0] {"id":"rs_1","type":"reasoning","summary":[],"encrypted_content":"gAAAAABsealed"}`,
				"close[0]",
				"ok turn",
			},
		},
		{
			name: "a reasoning item carrying neither a summary nor a seal is no block at all",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"Paris."}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":1,"content_index":0,"text":"Paris."}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "Paris."`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// A tool the service ran itself is an opaque block: it opens when
			// the item is announced, its progress events edit nothing, and the
			// finished item replaces what it opened with and ends it.
			name: "a hosted tool item is an opaque block",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"ws_1","type":"web_search_call","status":"in_progress"}}`) +
				frame("response.web_search_call.in_progress", `{"type":"response.web_search_call.in_progress","output_index":0,"item_id":"ws_1"}`) +
				frame("response.web_search_call.completed", `{"type":"response.web_search_call.completed","output_index":0,"item_id":"ws_1"}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"weather"}}}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":1,"content_index":0,"delta":"done"}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":1,"content_index":0,"text":"done"}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`opaque[0] kind=web_search_call data={"id":"ws_1","type":"web_search_call","status":"in_progress"} delta=""`,
				`opaque[0] kind=web_search_call data={"id":"ws_1","type":"web_search_call","status":"completed","action":{"type":"search","query":"weather"}} delta=""`,
				"close[0]",
				`text[1] "done"`,
				"close[1]",
				"ok turn",
			},
		},
		{
			// The interpreter streams the code it is writing the way a function
			// call streams its arguments, so the fragments are parts of the
			// block and the finished item carries the field whole.
			name: "a code interpreter call streams the code into the parts of its block",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"ci_1","type":"code_interpreter_call","status":"in_progress","code":null}}`) +
				frame("response.code_interpreter_call_code.delta", `{"type":"response.code_interpreter_call_code.delta","output_index":0,"item_id":"ci_1","delta":"print("}`) +
				frame("response.code_interpreter_call_code.delta", `{"type":"response.code_interpreter_call_code.delta","output_index":0,"item_id":"ci_1","delta":"1)"}`) +
				frame("response.code_interpreter_call.interpreting", `{"type":"response.code_interpreter_call.interpreting","output_index":0,"item_id":"ci_1"}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"ci_1","type":"code_interpreter_call","status":"completed","code":"print(1)","outputs":[{"type":"logs","logs":"1\n"}]}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`opaque[0] kind=code_interpreter_call data={"id":"ci_1","type":"code_interpreter_call","status":"in_progress","code":null} delta=""`,
				`opaque[0] kind= data= delta="print("`,
				`opaque[0] kind= data= delta="1)"`,
				`opaque[0] kind=code_interpreter_call data={"id":"ci_1","type":"code_interpreter_call","status":"completed","code":"print(1)","outputs":[{"type":"logs","logs":"1\n"}]} delta=""`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// The other two fields that stream, an MCP call's arguments and a
			// custom call's input, reach the same block the same way.
			name: "an mcp call and a custom tool call stream into their own blocks",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"mcp_1","type":"mcp_call","server_label":"docs","name":"search","arguments":""}}`) +
				frame("response.mcp_call_arguments.delta", `{"type":"response.mcp_call_arguments.delta","output_index":0,"item_id":"mcp_1","delta":"{\"q\":1}"}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"mcp_1","type":"mcp_call","server_label":"docs","name":"search","arguments":"{\"q\":1}","output":"ok"}}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"ctc_1","type":"custom_tool_call","call_id":"call_c","name":"patch","input":""}}`) +
				frame("response.custom_tool_call_input.delta", `{"type":"response.custom_tool_call_input.delta","output_index":1,"item_id":"ctc_1","delta":"diff"}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":1,"item":{"id":"ctc_1","type":"custom_tool_call","call_id":"call_c","name":"patch","input":"diff"}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`opaque[0] kind=mcp_call data={"id":"mcp_1","type":"mcp_call","server_label":"docs","name":"search","arguments":""} delta=""`,
				`opaque[0] kind= data= delta="{\"q\":1}"`,
				`opaque[0] kind=mcp_call data={"id":"mcp_1","type":"mcp_call","server_label":"docs","name":"search","arguments":"{\"q\":1}","output":"ok"} delta=""`,
				"close[0]",
				`opaque[1] kind=custom_tool_call data={"id":"ctc_1","type":"custom_tool_call","call_id":"call_c","name":"patch","input":""} delta=""`,
				`opaque[1] kind= data= delta="diff"`,
				`opaque[1] kind=custom_tool_call data={"id":"ctc_1","type":"custom_tool_call","call_id":"call_c","name":"patch","input":"diff"} delta=""`,
				"close[1]",
				"ok turn",
			},
		},
		{
			// An item whose block is whole when the item is says so on the item,
			// and status incomplete is the turn having cut it short. What it
			// carried is real and goes out; the missing close is what says the
			// rest of it never arrived.
			name: "an item the turn cut short carries what it has and never closes",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"ws_1","type":"web_search_call","status":"in_progress"}}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":0,"item":{"id":"ws_1","type":"web_search_call","status":"incomplete"}}`) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":1,"item":{"id":"rs_1","type":"reasoning","summary":[]}}`) +
				frame("response.reasoning_summary_text.delta", `{"type":"response.reasoning_summary_text.delta","output_index":1,"summary_index":0,"delta":"Checking "}`) +
				frame("response.output_item.done", `{"type":"response.output_item.done","output_index":1,"item":{"id":"rs_1","type":"reasoning","status":"incomplete","summary":[{"type":"summary_text","text":"Checking "}],"encrypted_content":"gAAAAABsealed"}}`) +
				frame("response.incomplete", `{"type":"response.incomplete","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"incomplete","incomplete_details":{"reason":"max_output_tokens"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`opaque[0] kind=web_search_call data={"id":"ws_1","type":"web_search_call","status":"in_progress"} delta=""`,
				`opaque[0] kind=web_search_call data={"id":"ws_1","type":"web_search_call","status":"incomplete"} delta=""`,
				`summary[1] part=0 "Checking "`,
				`meta[1] {"id":"rs_1","type":"reasoning","status":"incomplete","summary":[{"type":"summary_text","text":"Checking "}],"encrypted_content":"gAAAAABsealed"}`,
				"err capped",
			},
		},
		{
			// max_tokens is the spelling of a stale documented example and
			// max_output_tokens is the one the enum names. Both are the cap.
			name: "the stale spelling of the token cap ends the turn the same way",
			sse: frame("response.created", createdFrame) +
				frame("response.incomplete", `{"type":"response.incomplete","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"incomplete","incomplete_details":{"reason":"max_tokens"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err capped",
			},
		},
		{
			// A reason nobody here knows is not rounded to the nearest ending. It
			// arrives unclassified, with the spelling that arrived reachable
			// through Err, because a vocabulary that grew must not read as a turn
			// the model meant to give.
			name: "an incomplete reason nobody recognizes is unclassified rather than rounded",
			sse: frame("response.created", createdFrame) +
				frame("response.incomplete", `{"type":"response.incomplete","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"incomplete","incomplete_details":{"reason":"safety_review_pending"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err unknown",
			},
		},
		{
			// An incomplete response that names no reason is unclassified for the
			// same reason: the frame itself says the turn did not finish, so the
			// missing reason is one nobody can act on rather than an ordinary
			// completion.
			name: "an incomplete response that names no reason is unclassified",
			sse: frame("response.created", createdFrame) +
				frame("response.incomplete", `{"type":"response.incomplete","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"incomplete","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err unknown",
			},
		},
		{
			// The service reports a refusal as a completed response carrying a
			// refusal part, and the refusal text streams like any other. It still
			// ends the turn on the unusable side: the caller keeps none of it. The
			// text that already went out is not withdrawn.
			name: "a refusal part makes a completed response unusable",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.refusal.delta", `{"type":"response.refusal.delta","output_index":0,"content_index":0,"delta":"I cannot help with that."}`) +
				frame("response.refusal.done", `{"type":"response.refusal.done","output_index":0,"content_index":0,"refusal":"I cannot help with that."}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "I cannot help with that."`,
				"close[0]",
				"err refused",
			},
		},
		{
			// The phase opens the block it labels, so the meta and the text it
			// belongs to are one block rather than two, and the meta arrives
			// before the first fragment because that is when the wire says it.
			name: "assistant phase opens the text block it labels",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","phase":"commentary","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"Let me look."}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"Let me look."}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] ""`,
				`meta[0] {"model":"gpt-5.6-sol","phase":"commentary"}`,
				`text[0] "Let me look."`,
				"close[0]",
				"ok turn",
			},
		},
		{
			// The filter cutting in and the model declining are one code. The
			// caller does the same thing with either, and the line between them is
			// the service's own.
			name: "content filter is the same refusal",
			sse: frame("response.created", createdFrame) +
				frame("response.incomplete", `{"type":"response.incomplete","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"incomplete","incomplete_details":{"reason":"content_filter"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err refused",
			},
		},
		{
			name: "in band error, nested shape",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"partial"}`) +
				frame("error", `{"type":"error","sequence_number":9,"error":{"type":"server_error","code":"server_error","message":"An error occurred while processing your request.","param":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "partial"`,
				"err network",
			},
		},
		{
			name: "in band error, flat shape",
			sse: frame("response.created", createdFrame) +
				frame("error", `{"type":"error","code":"rate_limit_exceeded","message":"slow down","param":null,"sequence_number":1}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err busy",
			},
		},
		{
			name: "in band overload is busy",
			sse: frame("response.created", createdFrame) +
				frame("error", `{"type":"error","sequence_number":9,"error":{"message":"The model is overloaded.","type":"service_unavailable_error","param":null,"code":"server_is_overloaded"}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err busy",
			},
		},
		{
			name: "in band spend limit is access",
			sse: frame("response.created", createdFrame) +
				frame("error", `{"type":"error","sequence_number":9,"error":{"message":"Spend limit reached.","type":null,"param":null,"code":"project_spend_limit_exceeded"}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err access",
			},
		},
		{
			name: "in band error carrying only a type still classifies",
			sse: frame("response.created", createdFrame) +
				frame("error", `{"type":"error","sequence_number":9,"error":{"message":"The server had an error while processing your request","type":"server_error","param":null,"code":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err network",
			},
		},
		{
			name: "response.failed",
			sse: frame("response.created", createdFrame) +
				frame("response.failed", `{"type":"response.failed","sequence_number":5,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"failed","error":{"code":"invalid_prompt","message":"the prompt was rejected"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err invalid",
			},
		},
		{
			name: "response.failed carrying only a type still classifies",
			sse: frame("response.created", createdFrame) +
				frame("response.failed", `{"type":"response.failed","sequence_number":5,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"failed","error":{"type":"server_error","message":"the server had an error","code":null},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err network",
			},
		},
		{
			// This is the one adapter that learns a conversation did not fit from
			// a machine-readable code rather than from a sentence, which is why
			// the code is worth keeping apart from the token cap: raising the cap
			// makes this one worse.
			name: "an over-long conversation is named in the code",
			sse: frame("response.created", createdFrame) +
				frame("response.failed", `{"type":"response.failed","sequence_number":5,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"failed","error":{"code":"context_length_exceeded","message":"the input exceeds the context window"},"usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err context_length",
			},
		},
		{
			// A failure the service named nothing about classifies as nothing.
			// The sentence beside it frequently says what happened, and matching
			// it is how a caller ends up retrying what no retry fixes.
			name: "response.failed naming nothing classifies as nothing",
			sse: frame("response.created", createdFrame) +
				frame("response.failed", `{"type":"response.failed","sequence_number":5,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"failed","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err unknown",
			},
		},
		{
			// A turn that failed still spent whatever it generated before it
			// did, so the counters go out ahead of the failure that ends it.
			name: "response.failed reports what the turn spent",
			sse: frame("response.created", createdFrame) +
				frame("response.failed", `{"type":"response.failed","sequence_number":5,"response":{"id":"resp_1","model":"gpt-5.6-sol","status":"failed","error":{"code":"server_error","message":"the server had an error"},"usage":{"input_tokens":40,"output_tokens":9,"input_tokens_details":{"cached_tokens":10},"output_tokens_details":{"reasoning_tokens":4}}}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"usage in=30 out=5 reasoning=4 read=10 write=0",
				"err network",
			},
		},
		{
			// cancelled has no event type of its own, so it rides an ordinary
			// frame. Somebody who is not this caller stopped the turn, which no
			// code describes, so it ends unclassified rather than as one of the
			// endings it does not mean.
			name: "response cancelled has no event of its own",
			sse: frame("response.created", createdFrame) +
				frame("response.in_progress", `{"type":"response.in_progress","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"cancelled","usage":null}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err unknown",
			},
		},
		{
			// It rides a terminal frame too, and the status is read before the
			// frame type is. A cancellation announced on response.completed would
			// otherwise be reported as the turn the model meant to give.
			name: "a cancellation delivered on the completed frame is still a cancellation",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"half a sen"}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"cancelled","usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":3,"output_tokens_details":{"reasoning_tokens":0}}}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "half a sen"`,
				"usage in=10 out=3 reasoning=0 read=0 write=0",
				"err unknown",
			},
		},
		{
			name: "a stream that just stops leaves the reply incomplete",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"half a sen"}`),
			// No terminal event, and no close on the block that was still
			// filling. Both absences are the report.
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "half a sen"`,
			},
		},
		{
			name: "a frame that is not JSON ends the turn",
			sse: frame("response.created", createdFrame) +
				"event: response.output_text.delta\ndata: {not json\n\n",
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"err network",
			},
		},
		{
			// There is no [DONE] on this API, but the gateways WithBaseURL
			// exists for append the Chat Completions sentinel. The turn
			// completed and was billed, so a frame that does not parse after it
			// must not turn it into a retryable network failure.
			name: "junk after the terminal frame does not undo the turn",
			sse: frame("response.created", createdFrame) +
				frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
				frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"hello"}`) +
				frame("response.output_text.done", `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"hello"}`) +
				frame("response.completed", usageFrame) +
				"data: [DONE]\n\n",
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				`text[0] "hello"`,
				"close[0]",
				"usage in=300 out=162 reasoning=1024 read=1500 write=200",
				"ok turn",
			},
		},
		{
			name: "usage arrives before the ending even when the socket lingers",
			sse: frame("response.created", createdFrame) +
				frame("response.completed", usageFrame),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"usage in=300 out=162 reasoning=1024 read=1500 write=200",
				"ok turn",
			},
		},
		{
			name: "usage is not repeated when it has not moved",
			sse: frame("response.created", createdFrame) +
				frame("response.in_progress", `{"type":"response.in_progress","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"in_progress","usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":0,"output_tokens_details":{"reasoning_tokens":0}}}}`) +
				frame("response.queued", `{"type":"response.queued","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"queued","usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":0,"output_tokens_details":{"reasoning_tokens":0}}}}`) +
				frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":{"input_tokens":10,"input_tokens_details":{"cached_tokens":0,"cache_write_tokens":0},"output_tokens":4,"output_tokens_details":{"reasoning_tokens":0}}}}`),
			want: []string{
				"start id=resp_1 model=gpt-5.6-sol",
				"usage in=10 out=0 reasoning=0 read=0 write=0",
				"usage in=10 out=4 reasoning=0 read=0 write=0",
				"ok turn",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := sseServer(t, tc.sse)
			p := testProvider(t, srv.URL)
			got := collect(p.Stream(context.Background(), textRequest()))
			checkGrammar(t, got)
			equalLines(t, describeAll(got), tc.want)
		})
	}
}

// TestStreamFrameLargerThanScannerLimit covers the reason this decoder is not a
// bufio.Scanner: response.completed carries the whole response object and runs
// well past the 64 KiB default line limit on an ordinary long turn.
func TestStreamFrameLargerThanScannerLimit(t *testing.T) {
	big := strings.Repeat("lorem ipsum ", 40000) // ~480 KB
	delta, err := json.Marshal(map[string]any{
		"type":          "response.output_text.delta",
		"output_index":  0,
		"content_index": 0,
		"delta":         big,
	})
	if err != nil {
		t.Fatal(err)
	}
	sse := frame("response.created", createdFrame) +
		frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`) +
		frame("response.output_text.delta", string(delta)) +
		frame("response.output_text.done", `{"type":"response.output_text.done","output_index":0,"content_index":0,"text":"x"}`) +
		frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`)

	srv := sseServer(t, sse)
	p := testProvider(t, srv.URL)

	var text string
	var lines []string
	for e := range p.Stream(context.Background(), textRequest()) {
		if te, ok := e.(api.EventText); ok {
			text += te.Text
		}
		lines = append(lines, kindOf(e))
	}
	equalLines(t, lines, []string{"start", "text", "close", "ok"})
	if text != big {
		t.Fatalf("text truncated: got %d bytes, want %d", len(text), len(big))
	}
}

func kindOf(e api.Event) string {
	return strings.FieldsFunc(describe(e), func(r rune) bool { return r == '[' || r == ' ' })[0]
}

// TestStreamAbruptDisconnect covers a connection dying mid frame, which is
// indistinguishable from the model stopping mid sentence. The adapter does not
// guess between them: it says nothing, and the missing terminal event is what
// tells a consumer the reply is incomplete.
func TestStreamAbruptDisconnect(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, frame("response.created", createdFrame))
		fmt.Fprint(w, frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`))
		fmt.Fprint(w, frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"cut"}`))
		w.(http.Flusher).Flush()
		panic(http.ErrAbortHandler)
	}))
	defer srv.Close()

	p := testProvider(t, srv.URL)
	got := drain(p.Stream(context.Background(), textRequest()))
	equalLines(t, got, []string{
		"start id=resp_1 model=gpt-5.6-sol",
		`text[0] "cut"`,
	})
}

// TestStreamContextCancel covers a caller that stops the turn. Cancelling ends
// it with no terminal event and nothing after it: the caller walked away, and
// ctx.Err() is the ending they can read for themselves once the channel closes.
// A caller that keeps draining is not owed one either.
func TestStreamContextCancel(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, frame("response.created", createdFrame))
		fmt.Fprint(w, frame("response.output_item.added", `{"type":"response.output_item.added","output_index":0,"item":{"id":"msg_1","type":"message","role":"assistant","content":[]}}`))
		fmt.Fprint(w, frame("response.output_text.delta", `{"type":"response.output_text.delta","output_index":0,"content_index":0,"delta":"one"}`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	p := testProvider(t, srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := p.Stream(ctx, textRequest())

	var got []string
	for e := range ch {
		got = append(got, describe(e))
		if _, ok := e.(api.EventText); ok {
			cancel()
		}
	}
	equalLines(t, got, []string{
		"start id=resp_1 model=gpt-5.6-sol",
		`text[0] "one"`,
	})
	if ctx.Err() == nil {
		t.Fatal("the channel closed without the context being done")
	}
}

// TestStreamIdleTimeout covers the deadline every turn keeps, shortened here so
// the test does not wait for it. A turn that goes quiet is the adapter's problem
// to notice, because it holds the connection and can tell a queue wait from a
// dead socket.
func TestStreamIdleTimeout(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, frame("response.created", createdFrame))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	p := testProvider(t, srv.URL, WithTransport(lib.Options{Idle: 50 * time.Millisecond}))
	got := drain(p.Stream(context.Background(), textRequest()))
	equalLines(t, got, []string{
		"start id=resp_1 model=gpt-5.6-sol",
		"err network",
	})
}

// TestStreamIdleTimeoutBeforeHeaders covers the wait for the response headers,
// which is silence like any other. A gateway that accepts the POST and never
// answers is exactly what the idle deadline is wanted for, and nothing else
// bounds it: New leaves http.Client.Timeout unset on purpose.
func TestStreamIdleTimeoutBeforeHeaders(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	p := testProvider(t, srv.URL, WithTransport(lib.Options{Idle: 50 * time.Millisecond}))
	done := make(chan []string, 1)
	go func() { done <- drain(p.Stream(context.Background(), textRequest())) }()
	select {
	case got := <-done:
		equalLines(t, got, []string{"err network"})
	case <-time.After(5 * time.Second):
		t.Fatal("the turn never ended: the idle deadline did not cover the wait for the headers")
	}
}

// TestStreamKeepalivesDoNotHoldTheTurnOpen covers a peer that heartbeats after
// the terminal frame instead of closing, which is what an SSE multiplexing
// gateway does. The turn ended on that frame, so lines arriving after it are
// read by nobody and cannot keep the caller waiting.
func TestStreamKeepalivesDoNotHoldTheTurnOpen(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, frame("response.created", createdFrame))
		fmt.Fprint(w, frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`))
		w.(http.Flusher).Flush()

		tick := time.NewTicker(200 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-tick.C:
				fmt.Fprint(w, ": keepalive\n\n")
				w.(http.Flusher).Flush()
			case <-r.Context().Done():
				return
			}
		}
	}))
	defer srv.Close()

	// Cancelling is what frees the handler if the turn does not end, so this
	// test fails rather than hanging the package.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	p := testProvider(t, srv.URL)
	done := make(chan []string, 1)
	go func() { done <- drain(p.Stream(ctx, textRequest())) }()
	select {
	case got := <-done:
		equalLines(t, got, []string{
			"start id=resp_1 model=gpt-5.6-sol",
			"ok turn",
		})
	case <-time.After(5 * time.Second):
		cancel()
		<-done
		t.Fatal("the keepalives held the turn open past the terminal frame")
	}
}

// TestStreamEndsAtTerminalFrame covers a peer that holds the socket open after
// saying how the turn ended. The terminal event is the last one there is, so
// the turn ends on the frame rather than on the socket.
func TestStreamEndsAtTerminalFrame(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, frame("response.created", createdFrame))
		fmt.Fprint(w, frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`))
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	p := testProvider(t, srv.URL)
	done := make(chan []string, 1)
	go func() { done <- drain(p.Stream(context.Background(), textRequest())) }()
	select {
	case got := <-done:
		equalLines(t, got, []string{
			"start id=resp_1 model=gpt-5.6-sol",
			"ok turn",
		})
	case <-time.After(5 * time.Second):
		t.Fatal("the turn never ended")
	}
}

func TestHTTPErrorsBeforeTheStream(t *testing.T) {
	tests := []struct {
		name    string
		status  int
		headers map[string]string
		body    string
		want    api.ErrCode
		noRetry bool
	}{
		{
			name:   "bad key",
			status: 401,
			body:   `{"error":{"message":"Incorrect API key provided: sk-***","type":"invalid_request_error","param":null,"code":"invalid_api_key"}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "region",
			status: 403,
			body:   `{"error":{"message":"Country, region, or territory not supported","type":"request_forbidden","param":null,"code":null}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "bad request",
			status: 400,
			body:   `{"error":{"message":"Unsupported parameter: 'temperature' is not supported with this model.","type":"invalid_request_error","param":"temperature","code":"unsupported_parameter"}}`,
			want:   api.CodeInvalid,
		},
		{
			name:   "rate limit",
			status: 429,
			body:   `{"error":{"message":"Rate limit reached for gpt-5.6","type":"rate_limit_error","param":null,"code":"rate_limit_exceeded"}}`,
			want:   api.CodeBusy,
		},
		{
			name:   "no credit is not a rate limit",
			status: 429,
			body:   `{"error":{"message":"You exceeded your current quota","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "no credit balance is not a rate limit",
			status: 429,
			body:   `{"error":{"message":"Your organization has no credits left.","type":"insufficient_quota","param":null,"code":"credit_balance_exhausted"}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "an organization spend limit is not a rate limit",
			status: 429,
			body:   `{"error":{"message":"Spend limit reached.","type":null,"param":null,"code":"organization_spend_limit_exceeded"}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "a project spend limit is not a rate limit",
			status: 429,
			body:   `{"error":{"message":"Spend limit reached.","type":null,"param":null,"code":"project_spend_limit_exceeded"}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "an organization usage limit is not a rate limit",
			status: 429,
			body:   `{"error":{"message":"Usage limit reached.","type":null,"param":null,"code":"organization_usage_limit_exceeded"}}`,
			want:   api.CodeAccess,
		},
		{
			name:   "slow down is a rate limit",
			status: 429,
			body:   `{"error":{"message":"Please slow down.","type":"rate_limit_error","param":null,"code":"slow_down"}}`,
			want:   api.CodeBusy,
		},
		{
			// The flex tier answers 429 when it has no spare capacity, which is
			// the same code as a rate limit: both are answered by backing off and
			// sending the turn again as it stands. The row stays because the body
			// is a real one and it must not classify as anything else.
			name:   "flex has no capacity",
			status: 429,
			body:   `{"error":{"message":"Resource Unavailable","type":"invalid_request_error","param":null,"code":"resource_unavailable"}}`,
			want:   api.CodeBusy,
		},
		{
			name:   "server error",
			status: 500,
			body:   `{"error":{"message":"The server had an error while processing your request","type":"server_error","param":null,"code":null}}`,
			want:   api.CodeNetwork,
		},
		{
			// 503 is out of capacity rather than a failed attempt, so it asks
			// for a backoff and lands on the same code as a rate limit here as
			// it does in the sibling adapters.
			name:   "overloaded",
			status: 503,
			body:   `The engine is currently overloaded, please try again later`,
			want:   api.CodeBusy,
		},
		{
			// The header is retry advice and not a classification: the status
			// was recognized, so the code stands and the advice rides along on
			// the APIError.
			name:    "the backend says not to retry",
			status:  500,
			headers: map[string]string{"x-should-retry": "false"},
			body:    `{"error":{"message":"nope","type":"server_error","param":null,"code":null}}`,
			want:    api.CodeNetwork,
			noRetry: true,
		},
		{
			name:   "gateway html",
			status: 502,
			body:   `<html><body>502 Bad Gateway</body></html>`,
			want:   api.CodeNetwork,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				for k, v := range tc.headers {
					w.Header().Set(k, v)
				}
				w.Header().Set("x-request-id", "req_err")
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer srv.Close()

			p := testProvider(t, srv.URL)
			var events []api.Event
			for e := range p.Stream(context.Background(), textRequest()) {
				events = append(events, e)
			}
			if len(events) != 1 {
				t.Fatalf("got %d events, want exactly one EventErr: %v", len(events), events)
			}
			ee, ok := events[0].(api.EventErr)
			if !ok {
				t.Fatalf("got %T, want EventErr", events[0])
			}
			if ee.Code != tc.want {
				t.Fatalf("code %q, want %q (%v)", ee.Code, tc.want, ee.Err)
			}
			var api *APIError
			if !errors.As(ee.Err, &api) {
				t.Fatalf("error %v does not carry an *APIError", ee.Err)
			}
			if api.Status != tc.status {
				t.Fatalf("APIError.Status %d, want %d", api.Status, tc.status)
			}
			if api.RequestID != "req_err" {
				t.Fatalf("APIError.RequestID %q, want req_err", api.RequestID)
			}
			if api.NoRetry != tc.noRetry {
				t.Fatalf("APIError.NoRetry %v, want %v", api.NoRetry, tc.noRetry)
			}
		})
	}
}

// TestStreamTransportFailure covers a request that never reaches a server. It
// is CodeNetwork, the same as a stream that died mid frame and the same as a
// 500: the request went out as written and the answer is to send it again.
func TestStreamTransportFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))
	url := srv.URL
	srv.Close()

	p := testProvider(t, url)
	got := drain(p.Stream(context.Background(), textRequest()))
	equalLines(t, got, []string{"err network"})
}

// TestStreamRejectsUnsendableContent covers a request the adapter refuses to
// send at all: the failure still arrives as the one EventErr that ends the
// turn rather than as a return value nobody is reading for.
func TestStreamRejectsUnsendableContent(t *testing.T) {
	srv := sseServer(t, "")
	p := testProvider(t, srv.URL)
	req := api.Request{
		Messages: []api.Message{{Role: api.RoleAssistant, Content: api.Contents{
			api.ContentToolUse{ID: "call_a", Name: "search", Input: api.DiscardedArguments},
		}}},
	}
	var events []api.Event
	for e := range p.Stream(context.Background(), req) {
		events = append(events, e)
	}
	if len(events) != 1 {
		t.Fatalf("got %d events, want one: %v", len(events), events)
	}
	ee, ok := events[0].(api.EventErr)
	if !ok || ee.Code != api.CodeInvalid {
		t.Fatalf("got %#v, want EventErr{invalid}", events[0])
	}
	if !strings.Contains(ee.Err.Error(), "err_json") {
		t.Fatalf("error %v does not say why it refused", ee.Err)
	}
}

// TestStartCarriesTheConfiguredModel pins which of the two model names a turn is
// stamped with. An alias resolves to a snapshot with a name of its own, so the
// response object names a string nobody configured and no later request will
// send either; the stamp on every stored block of the turn comes from this
// event, and it has to be the name that can be compared against the next turn.
func TestStartCarriesTheConfiguredModel(t *testing.T) {
	srv := sseServer(t, frame("response.created", createdFrame)+
		frame("response.completed", `{"type":"response.completed","response":{"id":"resp_1","model":"gpt-5.6-sol","status":"completed","usage":null}}`))
	p := modelProvider(t, "gpt-5.6", srv.URL)
	got := drain(p.Stream(context.Background(), textRequest()))
	if got[0] != "start id=resp_1 model=gpt-5.6" {
		t.Fatalf("%s, want the model the provider is configured with", got[0])
	}
}

func TestProviderRefusesBytesMeasuredAsFitting(t *testing.T) {
	const declared = 1 << 20
	var sent int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		sent = len(body)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, `{"error":{"message":"payload too large","type":"invalid_request_error"}}`)
	}))
	t.Cleanup(srv.Close)
	p, err := New(Conf{Model: api.Model{Name: testModel, RequestBytes: declared}}, WithAPIKey("k"), WithBaseURL(srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	events := collect(p.Stream(context.Background(), textRequest()))
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: a 413 is the request refused by its size (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if !errors.Is(e.Err, api.ErrMeasuredTooLarge) {
		t.Fatalf("error %v does not carry api.ErrMeasuredTooLarge: a caller cannot tell it from a refusal this adapter made itself, and will retry it", e.Err)
	}
	if msg := e.Err.Error(); sent == 0 || !strings.Contains(msg, fmt.Sprint(sent)) || !strings.Contains(msg, fmt.Sprint(declared)) {
		t.Fatalf("error %q names neither the %d bytes sent nor the %d declared: nothing points at the misconfiguration", msg, sent, declared)
	}
	var carried *APIError
	if !errors.As(e.Err, &carried) || carried.Status != http.StatusRequestEntityTooLarge {
		t.Fatalf("error %v lost what the API answered", e.Err)
	}
}

// A refusal by size against a model that declares no limit is not marked as
// measured. api.ErrMeasuredTooLarge states that this adapter measured the body
// against Model.RequestBytes and sent it only because it fit, which is why a
// caller reading it stops instead of sending less; at a declared zero the check
// in request never ran, so nothing was measured and sending less is the remedy
// that works. The code still says the request was refused by its byte size.
func TestUnmeasuredRequestTooLargeIsNotMarked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		io.WriteString(w, `{"error":{"message":"payload too large","type":"invalid_request_error"}}`)
	}))
	t.Cleanup(srv.Close)
	p := testProvider(t, srv.URL)
	events := collect(p.Stream(context.Background(), textRequest()))
	if len(events) != 1 {
		t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
	}
	e, ok := events[0].(api.EventErr)
	if !ok {
		t.Fatalf("got %#v, want api.EventErr", events[0])
	}
	if e.Code != api.CodeTooLarge {
		t.Fatalf("code = %q, want %q: the refusal is still one about bytes (err %v)", e.Code, api.CodeTooLarge, e.Err)
	}
	if errors.Is(e.Err, api.ErrMeasuredTooLarge) {
		t.Fatalf("error %v carries api.ErrMeasuredTooLarge with nothing measured: the caller ends the turn where sending less would have worked", e.Err)
	}
	if msg := e.Err.Error(); strings.Contains(msg, "the 0 declared") {
		t.Fatalf("error %q reports a measurement against a limit the model never declared", msg)
	}
}

// An over-length conversation reaches the caller as CodeContextLength from the
// HTTP path, not only from inside a stream. This service names the condition in
// error.code, which is the one machine-readable statement of it any of these
// APIs makes, and a 400 classified on the status alone sends the caller off to
// fix a request whose only fault is length: the compaction that is the whole fix
// never runs. Every other 400 stays CodeInvalid.
func TestContextOverflowOnTheHTTPPath(t *testing.T) {
	tests := []struct {
		name string
		body string
		want api.ErrCode
	}{
		{
			name: "over length",
			body: `{"error":{"message":"Your input exceeds the context window of this model. Please adjust your input and try again.","type":"invalid_request_error","param":"input","code":"context_length_exceeded"}}`,
			want: api.CodeContextLength,
		},
		{
			name: "ordinary bad request",
			body: `{"error":{"message":"Unsupported parameter: 'temperature' is not supported with this model.","type":"invalid_request_error","param":"temperature","code":"unsupported_parameter"}}`,
			want: api.CodeInvalid,
		},
		{
			name: "no code at all",
			body: `{"error":{"message":"Invalid request","type":"invalid_request_error","param":null,"code":null}}`,
			want: api.CodeInvalid,
		},
		{
			name: "one string above its max length",
			body: `{"error":{"message":"Invalid 'input[0].content[0].text': string too long.","type":"invalid_request_error","param":"input[0].content[0].text","code":"string_above_max_length"}}`,
			want: api.CodeInvalid,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusBadRequest)
				io.WriteString(w, tc.body)
			}))
			t.Cleanup(srv.Close)
			p := testProvider(t, srv.URL)
			events := collect(p.Stream(context.Background(), textRequest()))
			if len(events) != 1 {
				t.Fatalf("got %d events, want one terminal error: %v", len(events), events)
			}
			e, ok := events[0].(api.EventErr)
			if !ok {
				t.Fatalf("got %#v, want api.EventErr", events[0])
			}
			if e.Code != tc.want {
				t.Fatalf("code = %q, want %q (err %v)", e.Code, tc.want, e.Err)
			}
		})
	}
}

func TestStringAboveMaxLengthIsInvalid(t *testing.T) {
	if got := errorVocabulary("string_above_max_length"); got != api.CodeInvalid {
		t.Fatalf("errorVocabulary(string_above_max_length) = %q, want %q", got, api.CodeInvalid)
	}
	if got := inBandCode("string_above_max_length", "invalid_request_error"); got != api.CodeInvalid {
		t.Fatalf("inBandCode(string_above_max_length) = %q, want %q", got, api.CodeInvalid)
	}
	if got := errorVocabulary("context_length_exceeded"); got != api.CodeContextLength {
		t.Fatalf("errorVocabulary(context_length_exceeded) = %q, want %q", got, api.CodeContextLength)
	}
}
