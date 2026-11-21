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
	"bytes"
	"encoding/json"

	"neui.dev/internal/utils"
)

// CallID returns the id an adapter sends for a stored tool call and for the
// result answering it. An id of 1 to 64 letters, digits, underscores and
// hyphens, which every provider takes, goes as it is; any other id, the empty
// one included, maps to an id of that form, the same one every time.
func CallID(id string) string {
	if portable(id) {
		return id
	}
	return utils.SanitizeName(id + "_" + utils.NameHash(id))
}

// CallName returns the tool name an adapter sends for a stored tool call. A
// name of 1 to 64 letters, digits, underscores and hyphens goes as it is; any
// other name, the empty one included, maps to a name of that form, the same one
// every time.
func CallName(name string) string {
	if portable(name) {
		return name
	}
	return utils.SanitizeName(name)
}

func portable(s string) bool {
	if len(s) == 0 || len(s) > 64 {
		return false
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '-') {
			return false
		}
	}
	return true
}

// ObjectInput returns the arguments of a stored tool call for a wire that takes
// only a JSON object: {} for no input, the input itself when it is an object,
// {"input":<input>} for any other JSON value and {"input":"<text>"} for input
// that is not JSON. The wrapped forms are what the tool layer reads for a tool
// whose schema is not an object.
func ObjectInput(input json.RawMessage) json.RawMessage {
	trimmed := bytes.TrimSpace(input)
	switch {
	case len(trimmed) == 0:
		return json.RawMessage(`{}`)
	case !json.Valid(trimmed):
		raw, _ := MarshalRaw(struct {
			Input string `json:"input"`
		}{string(input)})
		return raw
	case trimmed[0] == '{':
		return input
	}
	return json.RawMessage(`{"input":` + string(trimmed) + `}`)
}
