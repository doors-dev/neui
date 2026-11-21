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
	"encoding/json"

	"neui.dev/api"
)

func ProviderBlocks() api.Contents {
	return api.Contents{
		api.ContentReasoning{Model: "claude-haiku-5-5", Thoughts: api.Indexed[string]{0: "claude thinks"}, Meta: json.RawMessage(`{"provider":"claude","data":"EqQBCgIYAhIM1gbcDa9GJwZA2b3h+/=="}`)},
		api.ContentReasoning{Model: "claude-haiku-5-5", Meta: json.RawMessage(`{"provider":"claude","data":{"type":"redacted_thinking","data":"EmwKAhgBEgy+/=="}}`)},
		api.ContentOpaque{Model: "claude-haiku-5-5", Kind: "claude.server_tool_use", Data: json.RawMessage(`{"type":"server_tool_use","id":"srvtoolu_1","name":"web_search","input":{}}`), Deltas: []string{`{"query":`, `"cats"}`}},
		api.ContentReasoning{Model: "gpt-5.6-sol", Summary: api.Indexed[string]{0: "gpt summarises"}, Meta: json.RawMessage(`{"provider":"gpt","data":{"id":"rs_1","type":"reasoning","summary":[{"type":"summary_text","text":"gpt summarises"}],"encrypted_content":"gAAAAB+/=="}}`)},
		api.ContentReasoning{Model: "gpt-5.6-sol", Meta: json.RawMessage(`{"provider":"gpt","data":{"type":"reasoning","id":"rs_2","encrypted_content":"gAAAAC+/=="}}`)},
		api.ContentText{Text: "gpt preamble", Meta: json.RawMessage(`{"provider":"gpt","data":{"phase":"commentary"}}`)},
		api.ContentOpaque{Model: "gpt-5.6-sol", Kind: "gpt.web_search_call", Data: json.RawMessage(`{"id":"ws_1","type":"web_search_call","status":"completed","action":{"query":"cats"}}`)},
		api.ContentReasoning{Model: "deepseek-flash", Thoughts: api.Indexed[string]{0: "deepseek ", 1: "thinks"}, Meta: json.RawMessage(`{"provider":"deepseek"}`)},
		api.ContentReasoning{Model: "gemini-4-hydrogen", Meta: json.RawMessage(`{"provider":"gemini","data":{"type":"thought","signature":"EoMNCoANAWkU+/=="}}`)},
		api.ContentOpaque{Model: "gemini-4-hydrogen", Kind: "gemini.google_search_call", Data: json.RawMessage(`{"type":"google_search_call","id":"gs_1","arguments":{"queries":["cats"]},"signature":"c2ln"}`)},
		api.ContentReasoning{Model: "qwen3.8-27b", Thoughts: api.Indexed[string]{0: "compat thinks"}, Meta: json.RawMessage(`{"provider":"compat","data":"reasoning_content"}`)},
		api.ContentToolUse{ID: "call.1:x", Name: "functions.lookup", Input: json.RawMessage(`{"z":1,"a":[2]}`)},
	}
}
