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

package lib

import (
	"encoding/json"
	"regexp"
	"strings"
	"testing"
)

var portableName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,64}$`)

func TestCallID(t *testing.T) {
	for _, id := range []string{"toolu_01Wxot3PvHLXx37Hi7y1fmiJ", "call_00_r21FzvpowNfoG50YAVMl3648", "call_7667973828208411451", "gemini_0123456789_call_1", "call_1accf4185c3848b89095f7eb1e67f528", strings.Repeat("a", 64), "a-b"} {
		if got := CallID(id); got != id {
			t.Errorf("CallID(%q) = %q, want it unchanged", id, got)
		}
	}
	seen := map[string]string{}
	for _, id := range []string{"call.1:x", "call:1.x", "", strings.Repeat("a", 93), strings.Repeat("a", 65), "日本", "1", "toolu_01<&>"} {
		got := CallID(id)
		if !portableName.MatchString(got) {
			t.Errorf("CallID(%q) = %q, outside [A-Za-z0-9_-]{1,64}", id, got)
		}
		if got != CallID(id) {
			t.Errorf("CallID(%q) is not stable", id)
		}
		if other, dup := seen[got]; dup {
			t.Errorf("CallID(%q) = CallID(%q) = %q", id, other, got)
		}
		seen[got] = id
	}
}

func TestCallName(t *testing.T) {
	for _, name := range []string{"lookup", "page__write", "get-weather", strings.Repeat("n", 64)} {
		if got := CallName(name); got != name {
			t.Errorf("CallName(%q) = %q, want it unchanged", name, got)
		}
	}
	for name, want := range map[string]string{"functions.lookup": "functions_lookup", "": "", strings.Repeat("n", 139): ""} {
		got := CallName(name)
		if !portableName.MatchString(got) || want != "" && got != want {
			t.Errorf("CallName(%q) = %q, want %q in [A-Za-z0-9_-]{1,64}", name, got, want)
		}
		if got != CallName(name) {
			t.Errorf("CallName(%q) is not stable", name)
		}
	}
}

func TestObjectInput(t *testing.T) {
	for in, want := range map[string]string{
		``:             `{}`,
		` `:            `{}`,
		`{"q":1}`:      `{"q":1}`,
		` {"q": 1} `:   ` {"q": 1} `,
		`[1]`:          `{"input":[1]}`,
		`"cat"`:        `{"input":"cat"}`,
		`null`:         `{"input":null}`,
		` 7 `:          `{"input":7}`,
		`{"q":`:        `{"input":"{\"q\":"}`,
		`not json <&>`: `{"input":"not json <&>"}`,
	} {
		got := ObjectInput(json.RawMessage(in))
		if string(got) != want {
			t.Errorf("ObjectInput(%q) = %s, want %s", in, got, want)
		}
		var obj map[string]any
		if json.Unmarshal(got, &obj) != nil {
			t.Errorf("ObjectInput(%q) = %s, not an object", in, got)
		}
	}
}

func TestMetaRoundTrips(t *testing.T) {
	for _, c := range []struct {
		data string
		want string
	}{
		{``, `{"provider":"deepseek"}`},
		{`"sig<&>"`, `{"provider":"deepseek","data":"sig<&>"}`},
		{`{"type":"reasoning", "id":"rs_1"}`, `{"provider":"deepseek","data":{"type":"reasoning", "id":"rs_1"}}`},
	} {
		meta := Meta("deepseek", json.RawMessage(c.data))
		if string(meta) != c.want {
			t.Errorf("Meta(%s) = %s, want %s", c.data, meta, c.want)
		}
		data, own := OwnMeta(meta, "deepseek")
		if !own || string(data) != c.data {
			t.Errorf("OwnMeta(%s) = %s, %v, want %s, true", meta, data, own, c.data)
		}
		if _, own := OwnMeta(meta, "gemini"); own {
			t.Errorf("OwnMeta(%s) claims it for gemini", meta)
		}
	}
}
