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

package generic

import (
	"encoding/json"
	"strings"

	"neui.dev/provider/lib"
)

type reasoningDetails struct {
	entries []map[string]any
	at      map[string]int
}

func (d *reasoningDetails) add(value json.RawMessage) bool {
	var frags []map[string]json.RawMessage
	if json.Unmarshal(value, &frags) != nil || frags == nil {
		return false
	}
	for _, frag := range frags {
		index := frag["index"]
		apart := len(index) == 0 || string(index) == "null"
		at, seen := d.at[string(index)]
		if apart || !seen {
			at = len(d.entries)
			d.entries = append(d.entries, make(map[string]any))
			if !apart {
				if d.at == nil {
					d.at = make(map[string]int)
				}
				d.at[string(index)] = at
			}
		}
		entry := d.entries[at]
		for name, v := range frag {
			switch name {
			case "text", "summary", "data", "signature":
				if string(v) == "null" {
					if _, set := entry[name]; !set {
						entry[name] = v
					}
					continue
				}
				var s string
				if json.Unmarshal(v, &s) == nil {
					b, ok := entry[name].(*strings.Builder)
					if !ok {
						b = new(strings.Builder)
						entry[name] = b
					}
					b.WriteString(s)
					continue
				}
			}
			entry[name] = v
		}
	}
	return true
}

func (d *reasoningDetails) MarshalJSON() ([]byte, error) {
	list := make([]map[string]any, len(d.entries))
	for i, entry := range d.entries {
		list[i] = make(map[string]any, len(entry))
		for name, v := range entry {
			if b, ok := v.(*strings.Builder); ok {
				v = b.String()
			}
			list[i][name] = v
		}
	}
	return lib.MarshalRaw(list)
}
