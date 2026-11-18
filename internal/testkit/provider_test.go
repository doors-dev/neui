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

package testkit

import (
	"context"
	"fmt"
	"reflect"
	"testing"
	"time"

	"neui.dev/api"
)

func drain(t *testing.T, ch <-chan api.Event) []api.Event {
	t.Helper()
	var events []api.Event
	guard := time.After(5 * time.Second)
	for {
		select {
		case e, ok := <-ch:
			if !ok {
				return events
			}
			events = append(events, e)
		case <-guard:
			t.Fatalf("the turn never closed its channel: %v", events)
		}
	}
}

func text(text string) api.Request {
	return api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{api.ContentText{Text: text}}}}}
}

func TestProviderSayThenRunsOut(t *testing.T) {
	p := NewProvider(api.Model{}, Say("hi"))
	if m := p.Model(); m.Name != "fake" || m.Context == 0 || m.RequestBytes == 0 {
		t.Fatalf("a zero model resolved to %+v, want the large fake default", m)
	}
	events := drain(t, p.Stream(context.Background(), text("one")))
	if len(events) != 5 || events[1] != (api.EventText{Index: 0, Text: "hi"}) || events[4] != (api.EventOk{Code: api.CodeTurn}) {
		t.Fatalf("Say streamed %#v", events)
	}
	events = drain(t, p.Stream(context.Background(), text("two")))
	if len(events) != 1 {
		t.Fatalf("a request past the script streamed %#v, want one error", events)
	}
	if e, ok := events[0].(api.EventErr); !ok || e.Code != api.CodeUnknown {
		t.Fatalf("a request past the script streamed %#v, want CodeUnknown", events[0])
	}
	if got := p.Request(t, 1); !reflect.DeepEqual(got, text("two")) {
		t.Fatalf("request 1 recorded as %#v", got)
	}
}

func TestProviderCallMarshalsArgs(t *testing.T) {
	p := NewProvider(api.Model{Name: "tight", Context: 100}, Call("probe__run", map[string]int{"seq": 3}))
	if p.Model().Context != 100 {
		t.Fatalf("a given model was replaced by %+v", p.Model())
	}
	events := drain(t, p.Stream(context.Background(), text("go")))
	tool, ok := events[1].(api.EventTool)
	if !ok || tool.Name != "probe__run" || tool.Args != `{"seq":3}` || tool.ID == "" {
		t.Fatalf("Call streamed %#v", events)
	}
}

func TestProviderEventsWithoutEndingIsCut(t *testing.T) {
	p := NewProvider(api.Model{}, Events(api.EventStart{ID: "cut"}, api.EventText{Index: 0, Text: "half"}))
	events := drain(t, p.Stream(context.Background(), text("go")))
	if len(events) != 2 {
		t.Fatalf("a cut turn streamed %#v, want the two events and a close", events)
	}
}

func TestProviderHangEndsOnCancel(t *testing.T) {
	p := NewProvider(api.Model{})
	p.Push(Hang(api.EventStart{ID: "hang"}))
	ctx, cancel := context.WithCancel(context.Background())
	ch := p.Stream(ctx, text("go"))
	if e := <-ch; e != (api.EventStart{ID: "hang"}) {
		t.Fatalf("Hang streamed %#v first", e)
	}
	cancel()
	if events := drain(t, ch); len(events) != 0 {
		t.Fatalf("a canceled hang streamed %#v", events)
	}
}

func TestProviderThenReadsResults(t *testing.T) {
	p := NewProvider(api.Model{}, Then(func(req api.Request) Turn {
		var out struct{ Seq int }
		Decode(t, Results(req)[0], &out)
		return Say(fmt.Sprint(out.Seq + 1))
	}))
	req := api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
		api.ContentToolResult{ID: "c", Content: api.UserContents{api.ContentText{Text: `{"seq":3}`}}},
	}}}}
	events := drain(t, p.Stream(context.Background(), req))
	if events[1] != (api.EventText{Index: 0, Text: "4"}) {
		t.Fatalf("Then streamed %#v", events)
	}
}

func TestProviderRoutesCompaction(t *testing.T) {
	p := NewProvider(api.Model{}, Say("round"))
	p.OnCompaction(Say("summary"))
	compaction := text("history " + compactionMarker + ".")
	if events := drain(t, p.Stream(context.Background(), compaction)); events[1] != (api.EventText{Index: 0, Text: "summary"}) {
		t.Fatalf("the compaction got %#v", events)
	}
	if events := drain(t, p.Stream(context.Background(), text("go"))); events[1] != (api.EventText{Index: 0, Text: "round"}) {
		t.Fatalf("the round got %#v", events)
	}
	if n := len(p.Requests()); n != 2 {
		t.Fatalf("%d requests recorded, want both", n)
	}
}
