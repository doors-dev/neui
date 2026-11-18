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

package claude

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

const (
	docThinking = "https://platform.claude.com/docs/en/build-with-claude/thinking"
	docErrors   = "https://platform.claude.com/docs/en/api/errors"
	docVision   = "https://platform.claude.com/docs/en/build-with-claude/vision"
	docPDF      = "https://platform.claude.com/docs/en/build-with-claude/pdf-support"
	docModels   = "https://platform.claude.com/docs/en/about-claude/models/overview"
)

func TestConform(t *testing.T) {
	conform.Run(t, conformAdapter())
}

func conformAdapter() conform.Adapter {
	ceiling := map[string]int{}
	for _, m := range []api.Model{Fable51, Opus55, Sonnet55, Haiku55, Sonnet5} {
		ceiling[m.Name] = 128_000
	}
	ceiling[Haiku45.Name] = 64_000
	overCeiling := func(model string) func(conform.Params) bool {
		return func(p conform.Params) bool { return p.MaxTokens == ceiling[model]+1 }
	}
	rules := []conform.Rule{{
		Model:  "haiku-4-5",
		When:   func(p conform.Params) bool { return p.Effort != "" && p.Effort != conform.EffortOff },
		Want:   conform.RefuseNew,
		Source: docModels,
	}}
	for _, m := range []string{"opus-5-5", "sonnet-5-5", "fable-5-1"} {
		rules = append(rules,
			conform.Rule{
				Model:  m,
				When:   conform.Params.Forced,
				Want:   conform.RefuseLocal,
				Source: docErrors + "#forced-tool-use-not-supported",
			},
			conform.Rule{
				Model:  m,
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: docThinking + "#configuring-thinking",
			},
		)
	}
	for name := range ceiling {
		rules = append(rules, conform.Rule{
			Model:  name,
			When:   overCeiling(name),
			Want:   conform.RefuseNew,
			Source: docThinking + "#long-requests",
		})
	}
	rules = append(rules,
		conform.Rule{
			When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
			Carry:  map[string]any{"/thinking/type": "disabled"},
			Source: docThinking,
		},
		conform.Rule{
			When:   func(p conform.Params) bool { return p.Choice == conform.ChoiceNamed },
			Carry:  map[string]any{"/tool_choice": map[string]any{"type": "tool", "name": "lookup"}},
			Source: "https://platform.claude.com/docs/en/agents-and-tools/tool-use/implement-tool-use#forcing-tool-use",
		},
		conform.Rule{
			When:   func(p conform.Params) bool { return p.Choice == api.ToolChoiceAny },
			Carry:  map[string]any{"/tool_choice": map[string]any{"type": "any"}},
			Source: "https://platform.claude.com/docs/en/agents-and-tools/tool-use/implement-tool-use#forcing-tool-use",
		},
		conform.Rule{Entry: "image-no-mime", Carry: map[string]any{"/messages/0/content/1/type": "text"}, Source: docVision},
		conform.Rule{Entry: "document-csv", Carry: map[string]any{"/messages/0/content/1/source": map[string]any{"type": "text", "media_type": "text/plain", "data": "a,b\n1,2\n"}}, Source: docPDF},
		conform.Rule{Entry: "tool-result-image-bad-mime", Carry: map[string]any{"/messages/2/content/0/content/0/type": "text"}, Source: docVision},
		conform.Rule{Entry: "image-no-bytes", Carry: map[string]any{"/messages/0/content/1/type": "text"}, Source: docVision},
		conform.Rule{Entry: "whitespace-text", Carry: map[string]any{"/messages/0/content/1/type": "text"}, Source: docErrors},
		conform.Rule{Entry: "tool-error", Carry: map[string]any{"/messages/2/content/0/is_error": true}, Source: "https://platform.claude.com/docs/en/agents-and-tools/tool-use/implement-tool-use"},
		conform.Rule{Entry: "tool-discarded-arguments", Carry: map[string]any{"/messages/1/content/0/input": map[string]any{}}, Source: "https://platform.claude.com/docs/en/agents-and-tools/tool-use/implement-tool-use"},
	)
	return conform.Adapter{
		Name:   providerName,
		Models: []api.Model{Fable51, Opus55, Sonnet55, Haiku55, Sonnet5, Haiku45},
		New: func(base string, m api.Model, p conform.Params) (api.Provider, error) {
			key := p.Key
			if key == "" {
				key = "test-key"
			}
			opts := []Option{WithAPIKey(key), WithBaseURL(base)}
			switch p.Effort {
			case "":
			case conform.EffortOff:
				opts = append(opts, WithThinking(Thinking{Type: ThinkingDisabled}))
			default:
				opts = append(opts, WithEffort(Effort(p.Effort)))
			}
			if p.MaxTokens != 0 {
				opts = append(opts, WithMaxTokens(p.MaxTokens))
			}
			return New(Conf{Model: m}, opts...)
		},
		Wire: map[string]string{
			"effort":     "/output_config/effort",
			"off":        "/thinking/type",
			"choice":     "/tool_choice",
			"max_tokens": "/max_tokens",
		},
		Spec: conform.Spec{
			File:    "anthropic.json.gz",
			Request: "/components/schemas/CreateMessageParams",
			Count:   "/components/schemas/CountMessageTokensParams",
			Event:   "/components/schemas/MessageStreamEvent",
			Error:   "/components/schemas/ErrorResponse",
			Version: "anthropic-version",
		},
		Render: renderClaude,
		Errors: claudeErrors(),
		Rules:  rules,
		Signed: true,
		Live: conform.Live{
			Env:    "ANTHROPIC_API_KEY",
			Model:  Haiku45,
			Params: conform.Params{MaxTokens: 64},
			Base:   defaultBaseURL,
			Count:  "/v1/messages/count_tokens",
			Strip:  []string{"/max_tokens", "/stream", "/stop_sequences", "/temperature", "/top_p", "/top_k"},
			Auth: func(r *http.Request, key string) {
				r.Header.Set("x-api-key", key)
				r.Header.Set("anthropic-version", defaultVersion)
			},
		},
	}
}

func renderClaude(m api.Model, s conform.Script) (string, bool) {
	var b strings.Builder
	frame := func(event string, v any) {
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", event, data)
	}
	frame("message_start", map[string]any{"type": "message_start", "message": map[string]any{
		"id": "msg_01", "type": "message", "role": "assistant", "content": []any{}, "model": m.Name,
		"stop_reason": nil, "stop_sequence": nil, "stop_details": nil, "diagnostics": nil, "container": nil,
		"usage": map[string]any{
			"input_tokens": conform.UsageInput, "output_tokens": 1,
			"cache_creation": nil, "cache_creation_input_tokens": nil, "cache_read_input_tokens": nil,
			"inference_geo": nil, "output_tokens_details": nil, "server_tool_use": nil, "service_tier": nil, "speed": nil,
		},
	}})
	stop := "end_turn"
	for i, blk := range s.Blocks {
		start := func(block any) {
			frame("content_block_start", map[string]any{"type": "content_block_start", "index": i, "content_block": block})
		}
		delta := func(d any) {
			frame("content_block_delta", map[string]any{"type": "content_block_delta", "index": i, "delta": d})
		}
		switch blk.Kind {
		case conform.KindText:
			start(map[string]any{"type": "text", "text": "", "citations": nil})
			delta(map[string]any{"type": "text_delta", "text": blk.Text})
		case conform.KindThinking:
			start(map[string]any{"type": "thinking", "thinking": "", "signature": ""})
			delta(map[string]any{"type": "thinking_delta", "thinking": blk.Text})
			delta(map[string]any{"type": "signature_delta", "signature": blk.Signature})
		case conform.KindTool:
			stop = "tool_use"
			start(map[string]any{"type": "tool_use", "id": blk.ID, "name": blk.Name, "input": map[string]any{}, "caller": map[string]any{"type": "direct"}})
			delta(map[string]any{"type": "input_json_delta", "partial_json": blk.Args})
		case conform.KindOpaque:
			start(map[string]any{"type": "web_search_tool_result", "tool_use_id": "srvtoolu_01", "caller": map[string]any{"type": "direct"}, "content": []any{
				map[string]any{"type": "web_search_result", "url": "https://example.com/", "title": "Example", "encrypted_content": "ZW5j", "page_age": nil},
			}})
		default:
			return "", false
		}
		frame("content_block_stop", map[string]any{"type": "content_block_stop", "index": i})
	}
	switch s.Stop {
	case "":
	case api.CodeCapped:
		stop = "max_tokens"
	case api.CodeRefused:
		stop = "refusal"
	case api.CodeContextLength:
		stop = "model_context_window_exceeded"
	default:
		return "", false
	}
	frame("message_delta", map[string]any{"type": "message_delta",
		"delta": map[string]any{"stop_reason": stop, "stop_sequence": nil, "stop_details": nil, "container": nil},
		"usage": map[string]any{
			"output_tokens": conform.UsageOutput, "input_tokens": nil,
			"cache_creation_input_tokens": nil, "cache_read_input_tokens": nil, "output_tokens_details": nil, "server_tool_use": nil,
		},
	})
	frame("message_stop", map[string]any{"type": "message_stop"})
	return b.String(), true
}

func claudeErrors() []conform.ErrorCase {
	body := func(kind, message string) string {
		return fmt.Sprintf(`{"type":"error","error":{"type":%q,"message":%q},"request_id":"req_011"}`, kind, message)
	}
	return []conform.ErrorCase{
		{Name: "invalid", Status: 400, Body: body("invalid_request_error", "messages: field required"), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "prompt-too-long", Status: 400, Body: body("invalid_request_error", "prompt is too long: 1210970 tokens > 1000000 maximum"), Want: api.CodeContextLength, Schema: true, Source: docErrors},
		{Name: "workspace-spend-limit", Status: 400, Body: body("invalid_request_error", "You have reached your specified workspace API usage limits. You will regain access on 2026-12-01 at 00:00 UTC."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "authentication", Status: 401, Body: body("authentication_error", "invalid x-api-key"), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "billing", Status: 402, Body: body("billing_error", "credit balance is too low"), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "permission", Status: 403, Body: body("permission_error", "no access to this model"), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "not-found", Status: 404, Body: body("not_found_error", "model: claude-nope"), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "request-too-large", Status: 413, Body: body("request_too_large", "Request exceeds the maximum allowed number of bytes."), Want: api.CodeTooLarge, Source: docErrors},
		{Name: "rate-limit", Status: 429, Header: http.Header{"Retry-After": {"7"}}, Body: body("rate_limit_error", "Number of request tokens has exceeded your per-minute rate limit"), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "spend-cap", Status: 429, Body: `{"type":"error","error":{"type":"rate_limit_error","message":"Your organization has reached its spend limit.","details":{"error_code":"enforced_spend_limit_reached"}},"request_id":"req_011"}`, Want: api.CodeAccess, Source: docErrors},
		{Name: "api-error", Status: 500, Body: body("api_error", "Internal server error"), Want: api.CodeNetwork, Schema: true, Source: docErrors},
		{Name: "timeout", Status: 504, Body: body("timeout_error", "Request timed out"), Want: api.CodeNetwork, Schema: true, Source: docErrors},
		{Name: "overloaded", Status: 529, Body: body("overloaded_error", "Overloaded"), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "unparseable", Status: 502, Body: "<html>bad gateway</html>", Want: api.CodeNetwork, Source: docErrors},
		{Name: "in-stream-overloaded", Stream: "event: message_start\ndata: {\"type\":\"message_start\",\"message\":{\"id\":\"msg_01\",\"model\":\"m\",\"usage\":{\"input_tokens\":1,\"output_tokens\":1}}}\n\nevent: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\",\"message\":\"Overloaded\"}}\n\n", Want: api.CodeBusy, Source: docErrors + "#errors-during-streaming"},
		{Name: "in-stream-too-large", Stream: "event: error\ndata: {\"type\":\"error\",\"error\":{\"type\":\"request_too_large\",\"message\":\"too large\"}}\n\n", Want: api.CodeTooLarge, Source: docErrors + "#errors-during-streaming"},
	}
}
