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

package deepseek

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
	docCreate   = "https://api-docs.deepseek.com/api/create-chat-completion"
	docThinking = "https://api-docs.deepseek.com/guides/thinking_mode"
	docErrors   = "https://api-docs.deepseek.com/quick_start/error_codes"
	docWebTools = "https://api-docs.deepseek.com/guides/web_tools"
)

func TestConform(t *testing.T) {
	conform.Run(t, conformAdapter())
}

func conformAdapter() conform.Adapter {
	thinking := conform.Params.Thinking
	off := func(p conform.Params) bool { return !p.Thinking() }
	return conform.Adapter{
		Name:   providerName,
		Models: []api.Model{Flash, V4Pro},
		New: func(base string, m api.Model, p conform.Params) (api.Provider, error) {
			key := p.Key
			if key == "" {
				key = "test-key"
			}
			opts := []Option{WithAPIKey(key), WithBaseURL(base)}
			switch p.Effort {
			case "":
			case conform.EffortOff:
				opts = append(opts, WithEffort(EffortNone))
			default:
				opts = append(opts, WithEffort(Effort(p.Effort)))
			}
			if p.Temperature != 0 {
				opts = append(opts, WithTemperature(p.Temperature))
			}
			if p.TopP != 0 {
				opts = append(opts, WithTopP(p.TopP))
			}
			if p.MaxTokens != 0 {
				opts = append(opts, WithMaxTokens(p.MaxTokens))
			}
			return New(Conf{Model: m}, opts...)
		},
		Wire: map[string]string{
			"effort":      "/reasoning_effort",
			"off":         "/thinking/type",
			"choice":      "/tool_choice",
			"temperature": "/temperature",
			"top_p":       "/top_p",
			"max_tokens":  "/max_tokens",
		},
		Spec: conform.Spec{
			File:         "openai.json.gz",
			Request:      "/components/schemas/CreateChatCompletionRequest",
			Event:        "/components/schemas/CreateChatCompletionStreamResponse",
			Error:        "/components/schemas/ErrorResponse",
			Private:      []string{"/thinking", "/user_id", "/messages/*/reasoning_content"},
			EventPrivate: []string{"/choices/*/delta/reasoning_content", "/choices/*/finish_reason", "/usage/prompt_cache_hit_tokens", "/usage/prompt_cache_miss_tokens"},
			LooseOneOf:   true,
			Accept: []conform.Accept{{
				At:     "/components/schemas/ChatCompletionRequestToolMessageContentPart",
				Also:   map[string]any{"$ref": "#/components/schemas/ChatCompletionRequestMessageContentPartImage"},
				Source: "https://api-docs.deepseek.com/guides/vision refuses an image only in system and assistant messages, and Flash takes image_url parts in tool messages as in user ones; the spec is OpenAI's, which allows text alone there",
			}},
		},
		Render:   renderDeepSeek,
		Messages: "/messages",
		CrossCheck: func(t *testing.T, body []byte) {
			conform.CheckChatPairs(t, body)
			var req struct {
				Messages []map[string]json.RawMessage `json:"messages"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatal(err)
			}
			for i, m := range req.Messages {
				_, rc := m["reasoning_content"]
				if assistant := string(m["role"]) == `"assistant"`; rc != assistant {
					t.Errorf("message %d with role %s carries reasoning_content: %v", i, m["role"], rc)
				}
			}
		},
		Errors:   deepseekErrors(),
		Unframed: true,
		Sampling: true,
		Rules: []conform.Rule{
			{When: func(p conform.Params) bool { return p.Thinking() && p.Forced() }, Want: conform.RefuseLocal, Source: docCreate},
			{When: thinking, Carry: map[string]any{"/thinking": map[string]any{"type": "enabled"}}, Source: docThinking},
			{When: off, Carry: map[string]any{"/thinking": map[string]any{"type": "disabled"}}, Omit: []string{"/reasoning_effort"}, Source: docThinking},
			{Omit: []string{"/thinking/reasoning_effort"}, Source: docCreate},
			{When: func(p conform.Params) bool { return p.Thinking() && p.Temperature != 0 }, Want: conform.RefuseNew, Source: docThinking},
			{Model: "flash", When: func(p conform.Params) bool { return !p.Thinking() && p.TopP != 0 }, Want: conform.RefuseNew, Source: docCreate},
			{Model: "v4-pro", When: func(p conform.Params) bool { return p.Thinking() && p.TopP != 0 }, Want: conform.RefuseNew, Source: docCreate},
			{When: func(p conform.Params) bool { return p.MaxTokens > maxOutputTokens }, Want: conform.RefuseNew, Source: docCreate},
			{When: func(p conform.Params) bool { return p.Choice == api.ToolChoiceAny }, Carry: map[string]any{"/tool_choice": "required"}, Source: docCreate},
			{When: func(p conform.Params) bool { return p.Choice == conform.ChoiceNamed }, Carry: map[string]any{"/tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "lookup"}}}, Source: docCreate},
			{Carry: map[string]any{"/stream_options": map[string]any{"include_usage": true}}, Source: docCreate},
			{Entry: "tools-text-turns", Carry: map[string]any{"/messages/1/reasoning_content": ""}, Source: docThinking},
			{Entry: "reasoning-unsigned", Carry: map[string]any{"/messages/1/reasoning_content": ""}, Source: docThinking},
			{Entry: "tool-error", Carry: map[string]any{"/messages/2/role": "tool", "/messages/2/content": "Error: lookup is down"}, Source: docCreate},
			{Entry: "documents", Carry: map[string]any{"/messages/0/content": "summarize these\n\n[document omitted]\n\nplain words"}, Source: docCreate},
			{Entry: "document-csv", Carry: map[string]any{"/messages/0/content": "summarize\n\na,b\n1,2\n"}, Source: docCreate},
			{Entry: "tool-result-document", Carry: map[string]any{"/messages/2/content": "[document omitted]"}, Source: docCreate},
		},
		Live: conform.Live{
			Env:    "DEEPSEEK_API_KEY",
			Model:  Flash,
			Params: conform.Params{MaxTokens: 1024, Effort: conform.EffortLow},
			Base:   defaultBaseURL,
			Auth: func(r *http.Request, key string) {
				r.Header.Set("Authorization", "Bearer "+key)
			},
		},
	}
}

func renderDeepSeek(m api.Model, s conform.Script) (string, bool) {
	var b strings.Builder
	chunk := func(delta map[string]any, finish any, usage any) {
		data, err := json.Marshal(map[string]any{
			"id": "chatcmpl-01", "object": "chat.completion.chunk", "created": 1, "model": m.Name,
			"choices": []any{map[string]any{"index": 0, "delta": delta, "finish_reason": finish, "logprobs": nil}},
			"usage":   usage,
		})
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&b, "data: %s\n\n", data)
	}
	chunk(map[string]any{"role": "assistant", "content": ""}, nil, nil)
	finish := "stop"
	calls := 0
	for _, blk := range s.Blocks {
		switch blk.Kind {
		case conform.KindText:
			chunk(map[string]any{"content": blk.Text}, nil, nil)
		case conform.KindThinking:
			chunk(map[string]any{"reasoning_content": blk.Text}, nil, nil)
		case conform.KindTool:
			finish = "tool_calls"
			chunk(map[string]any{"tool_calls": []any{map[string]any{"index": calls, "id": blk.ID, "type": "function", "function": map[string]any{"name": blk.Name, "arguments": ""}}}}, nil, nil)
			chunk(map[string]any{"tool_calls": []any{map[string]any{"index": calls, "function": map[string]any{"arguments": blk.Args}}}}, nil, nil)
			calls++
		default:
			return "", false
		}
	}
	switch s.Stop {
	case "":
	case api.CodeCapped:
		finish = "length"
	case api.CodeRefused:
		finish = "content_filter"
	case api.CodeNetwork:
		finish = "aborted"
	case api.CodeBusy:
		finish = "insufficient_system_resource"
	default:
		return "", false
	}
	chunk(map[string]any{}, finish, map[string]any{
		"prompt_tokens": conform.UsageInput, "completion_tokens": conform.UsageOutput, "total_tokens": conform.UsageInput + conform.UsageOutput,
		"prompt_cache_hit_tokens": 0, "prompt_cache_miss_tokens": conform.UsageInput,
	})
	b.WriteString("data: [DONE]\n\n")
	return b.String(), true
}

func deepseekErrors() []conform.ErrorCase {
	body := func(kind, message string) string {
		return fmt.Sprintf(`{"error":{"message":%q,"type":%q,"param":null,"code":%q}}`, message, kind, kind)
	}
	return []conform.ErrorCase{
		{Name: "invalid-format", Status: 400, Body: body("invalid_request_error", "Invalid request body format."), Want: api.CodeInvalid, Source: docErrors},
		{Name: "context-length", Status: 400, Body: body("invalid_request_error", "This model's maximum context length is 1048576 tokens. However, you requested 1200000 tokens (1200000 in the messages, 0 in the completion). Please reduce the length of the messages or completion."), Want: api.CodeContextLength, Source: docErrors},
		{Name: "authentication", Status: 401, Body: body("authentication_error", "Authentication Fails, Your api key is invalid."), Want: api.CodeAccess, Source: docErrors},
		{Name: "insufficient-balance", Status: 402, Body: body("invalid_request_error", "Insufficient Balance"), Want: api.CodeAccess, Source: docErrors},
		{Name: "invalid-parameters", Status: 422, Body: body("invalid_request_error", "Invalid parameters."), Want: api.CodeInvalid, Source: docErrors},
		{Name: "rate-limit", Status: 429, Body: body("rate_limit_error", "Rate limit reached."), Want: api.CodeBusy, Source: docErrors},
		{Name: "server-error", Status: 500, Body: body("server_error", "Server error."), Want: api.CodeNetwork, Source: docErrors},
		{Name: "overloaded", Status: 503, Body: body("server_error", "Server overloaded."), Want: api.CodeBusy, Source: docErrors},
		{Name: "too-large", Status: 413, Body: "payload too large", Want: api.CodeTooLarge, Source: docErrors},
		{Name: "generation-failure", Stream: "data: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1785000000,\"model\":\"deepseek-v4-pro\",\"choices\":[{\"index\":0,\"delta\":{\"content\":\"Partial\"},\"finish_reason\":null}]}\n\ndata: {\"id\":\"chatcmpl-1\",\"object\":\"chat.completion.chunk\",\"created\":1785000000,\"model\":\"deepseek-v4-pro\",\"choices\":[],\"usage\":{\"prompt_tokens\":11,\"completion_tokens\":7,\"total_tokens\":18},\"error\":{\"message\":\"The model failed to generate a complete response. Please retry.\",\"type\":\"api_error\"}}\n\n", Want: api.CodeNetwork, Source: docWebTools + "#generation-failures"},
		{Name: "in-stream-error", Stream: "data: {\"error\":{\"message\":\"Rate limit reached.\",\"type\":\"rate_limit_error\",\"code\":\"rate_limit_exceeded\"}}\n\n", Want: api.CodeBusy, Source: docErrors},
	}
}
