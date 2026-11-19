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
	"fmt"
	"strings"
	"testing"

	"neui.dev/api"
)

func CheckGrammar(t testing.TB, events []api.Event) {
	t.Helper()
	walkGrammar(t, events, false)
}

func CheckUnframed(t testing.TB, events []api.Event) {
	t.Helper()
	walkGrammar(t, events, true)
}

func walkGrammar(t testing.TB, events []api.Event, unframed bool) {
	t.Helper()
	fail := func(i int, format string, args ...any) {
		t.Helper()
		t.Fatalf("event %d (%s): %s\nsequence:\n  %s", i, Describe(events[i]), fmt.Sprintf(format, args...), strings.Join(DescribeAll(events), "\n  "))
	}
	kinds := map[int]string{}
	closed := map[int]bool{}
	highest := -1
	ended := false
	for i, e := range events {
		if ended {
			fail(i, "arrives after the stream already ended")
		}
		if i == 0 {
			switch e.(type) {
			case api.EventStart, api.EventErr:
			default:
				fail(i, "opens the stream, which only a start or an err may")
			}
		}
		switch e := e.(type) {
		case api.EventStart:
			if i != 0 {
				fail(i, "start is only valid first")
			}
			continue
		case api.EventErr:
			if e.Err == nil {
				fail(i, "carries no Err")
			}
			ended = true
			continue
		case api.EventOk:
			ended = true
			continue
		case api.EventUsage:
			continue
		}
		idx, kind, ok := blockOf(e)
		if !ok {
			fail(i, "unknown event type %T", e)
		}
		if closed[idx] {
			fail(i, "edits block %d after it closed", idx)
		}
		if idx > highest {
			if idx != highest+1 {
				fail(i, "index %d leaves a hole after %d, and Index is a position in the message", idx, highest)
			}
			highest = idx
		}
		switch {
		case kind == "close":
			if unframed {
				fail(i, "closes block %d, on a wire that never says a block ended", idx)
			}
			if kinds[idx] == "" {
				fail(i, "closes block %d, which no event ever opened", idx)
			}
			closed[idx] = true
		case kind == "meta":
			if kinds[idx] == "opaque" {
				fail(i, "attaches meta to opaque block %d, whose Data is already the adapter's", idx)
			}
			if kinds[idx] == "" {
				fail(i, "opens block %d with meta, and the first event of a block names its kind", idx)
			}
		case kinds[idx] == "":
			kinds[idx] = kind
		case kinds[idx] != kind:
			fail(i, "fills %s block %d with a %s event", kinds[idx], idx, kind)
		}
	}
}

func blockOf(e api.Event) (int, string, bool) {
	switch e := e.(type) {
	case api.EventText:
		return e.Index, "text", true
	case api.EventTool:
		return e.Index, "tool", true
	case api.EventThinking:
		return e.Index, "reasoning", true
	case api.EventSummary:
		return e.Index, "reasoning", true
	case api.EventMeta:
		return e.Index, "meta", true
	case api.EventOpaque:
		return e.Index, "opaque", true
	case api.EventClose:
		return e.Index, "close", true
	}
	return 0, "", false
}

func Describe(e api.Event) string {
	raw := func(r []byte) string {
		if len(r) == 0 {
			return "-"
		}
		return string(r)
	}
	switch e := e.(type) {
	case api.EventStart:
		return fmt.Sprintf("start %s %s", e.ID, e.Model)
	case api.EventText:
		return fmt.Sprintf("text %d %q", e.Index, e.Text)
	case api.EventTool:
		return fmt.Sprintf("tool %d %q %q %q", e.Index, e.ID, e.Name, e.Args)
	case api.EventThinking:
		return fmt.Sprintf("thinking %d part=%d %q", e.Index, e.Part, e.Text)
	case api.EventSummary:
		return fmt.Sprintf("summary %d part=%d %q", e.Index, e.Part, e.Text)
	case api.EventMeta:
		return fmt.Sprintf("meta %d %s", e.Index, raw(e.Data))
	case api.EventOpaque:
		return fmt.Sprintf("opaque %d %q %s %q", e.Index, e.Kind, raw(e.Data), e.Delta)
	case api.EventClose:
		return fmt.Sprintf("close %d", e.Index)
	case api.EventUsage:
		return fmt.Sprintf("usage in=%d out=%d rsn=%d cr=%d cw=%d", e.Input, e.Output, e.Reasoning, e.CacheRead, e.CacheWrite)
	case api.EventOk:
		return fmt.Sprintf("ok %s", e.Code)
	case api.EventErr:
		return fmt.Sprintf("err %s %v", e.Code, e.Err)
	}
	return fmt.Sprintf("unknown %T", e)
}

func DescribeAll(events []api.Event) []string {
	out := make([]string, len(events))
	for i, e := range events {
		out[i] = Describe(e)
	}
	return out
}
