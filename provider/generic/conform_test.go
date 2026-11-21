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
	"fmt"
	"net/http"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

const (
	docCreate      = "https://developers.openai.com/api/reference/resources/chat/subresources/completions/methods/create"
	docErrorCodes  = "https://developers.openai.com/api/docs/guides/error-codes"
	docLlama       = "https://github.com/ggml-org/llama.cpp/blob/master/tools/server/README.md"
	docLlamaSource = "https://github.com/ggml-org/llama.cpp/blob/master/tools/server/server-common.cpp"
	docORErrors    = "https://openrouter.ai/docs/api/reference/errors-and-debugging"
	docORReason    = "https://openrouter.ai/docs/guides/best-practices/reasoning-tokens"

	upstreamToolValidation = "Upstream error from Groq: Tool call validation failed: tool call validation failed: parameters for tool page__read did not match schema: errors: [missing properties: 'id']"
)

func TestConform(t *testing.T) {
	conform.Run(t, conformAdapter())
}

func conformAdapter() conform.Adapter {
	return conform.Adapter{
		Name:       providerName,
		Models:     []api.Model{{Name: "qwen3.8-27b"}},
		Undeclared: true,
		New: func(base string, m api.Model, p conform.Params) (api.Provider, error) {
			key := p.Key
			if key == "" {
				key = "test-key"
			}
			opts := []Option{WithAPIKey(key)}
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
			return New(Conf{Model: m, BaseURL: base}, opts...)
		},
		Wire: map[string]string{
			"effort":      "/reasoning_effort",
			"off":         "/reasoning_effort",
			"choice":      "/tool_choice",
			"temperature": "/temperature",
			"top_p":       "/top_p",
			"max_tokens":  "/max_tokens",
		},
		Spec: conform.Spec{
			File:    "openai.json.gz",
			Request: "/components/schemas/CreateChatCompletionRequest",
			Event:   "/components/schemas/CreateChatCompletionStreamResponse",
			Error:   "/components/schemas/ErrorResponse",
			Private: []string{"/messages/*/reasoning_content", "/messages/*/reasoning"},
			EventPrivate: []string{
				"/choices/*/delta/reasoning", "/choices/*/delta/reasoning_content", "/choices/*/delta/reasoning_details",
				"/choices/*/finish_reason", "/choices/*/native_finish_reason",
			},
			LooseOneOf: true,
			Accept: []conform.Accept{{
				At:     "/components/schemas/ChatCompletionRequestToolMessageContentPart",
				Also:   map[string]any{"$ref": "#/components/schemas/ChatCompletionRequestMessageContentPartImage"},
				Source: "live capture openrouter/qwen-tool-result-image, 2026-11-20: OpenRouter takes an image_url part in a tool message and qwen/qwen3.8-27b reads it; the spec is OpenAI's, which allows text alone there",
			}},
		},
		Render:     renderGeneric,
		Messages:   "/messages",
		CrossCheck: conform.CheckChatPairs,
		Skipped:    func(r api.ContentReasoning) bool { return r.Model != "qwen3.8-27b" },
		Errors:     genericErrors(),
		Unframed:   true,
		Sampling:   true,
		Rules: []conform.Rule{
			{When: func(p conform.Params) bool { return p.Effort == conform.EffortOff }, Carry: map[string]any{"/reasoning_effort": "none"}, Source: docLlama},
			{When: func(p conform.Params) bool { return p.Effort == "" }, Omit: []string{"/reasoning_effort"}, Source: docCreate},
			{When: func(p conform.Params) bool { return p.Choice == api.ToolChoiceAny }, Carry: map[string]any{"/tool_choice": "required"}, Source: docCreate},
			{When: func(p conform.Params) bool { return p.Choice == conform.ChoiceNamed }, Carry: map[string]any{"/tool_choice": map[string]any{"type": "function", "function": map[string]any{"name": "lookup"}}}, Source: docCreate},
			{Carry: map[string]any{"/stream": true, "/stream_options": map[string]any{"include_usage": true}}, Source: docCreate},
			{Entry: conform.ProbeEntry, Carry: map[string]any{"/tools/1/function/parameters": map[string]any{"type": "object", "properties": map[string]any{}}}, Source: docCreate},
			{Entry: "tools-text-turns", Omit: []string{"/messages/1/reasoning_content", "/messages/1/reasoning"}, Source: docORReason},
			{Entry: "reasoning-unsigned", Omit: []string{"/messages/1/reasoning_content", "/messages/1/reasoning"}, Source: docORReason},
			{Entry: "tool-error", Carry: map[string]any{"/messages/2/role": "tool", "/messages/2/content": "Error: lookup is down"}, Source: docCreate},
			{Entry: "documents", Carry: map[string]any{"/messages/0/content": "summarize these\n\n[document omitted]\n\nplain words"}, Source: docCreate},
		},
		Live: conform.Live{
			Env:    "OPENROUTER_API_KEY",
			Model:  api.Model{Name: "qwen/qwen3.8-27b", Context: 1_024_000, ImagesPerRequest: 8},
			Params: conform.Params{MaxTokens: 1024, Effort: conform.EffortLow},
			Base:   "https://openrouter.ai/api/v1",
			Auth: func(r *http.Request, key string) {
				r.Header.Set("Authorization", "Bearer "+key)
			},
		},
	}
}

func renderGeneric(m api.Model, s conform.Script) (string, bool) {
	var b strings.Builder
	frame := func(v map[string]any) {
		v["id"], v["object"], v["created"], v["model"] = "chatcmpl-01", "chat.completion.chunk", 1, m.Name
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&b, "data: %s\n\n", data)
	}
	delta := func(d map[string]any, finish any) {
		frame(map[string]any{"choices": []any{map[string]any{"index": 0, "delta": d, "finish_reason": finish}}})
	}
	usage := func() {
		frame(map[string]any{"choices": []any{}, "usage": map[string]any{
			"prompt_tokens": conform.UsageInput, "completion_tokens": conform.UsageOutput, "total_tokens": conform.UsageInput + conform.UsageOutput,
		}})
	}
	delta(map[string]any{"role": "assistant", "content": ""}, nil)
	finish := "stop"
	calls := 0
	for _, blk := range s.Blocks {
		switch blk.Kind {
		case conform.KindText:
			delta(map[string]any{"content": blk.Text}, nil)
		case conform.KindThinking:
			delta(map[string]any{"reasoning": blk.Text}, nil)
		case conform.KindTool:
			finish = "tool_calls"
			delta(map[string]any{"tool_calls": []any{map[string]any{"index": calls, "id": blk.ID, "type": "function", "function": map[string]any{"name": blk.Name, "arguments": ""}}}}, nil)
			delta(map[string]any{"tool_calls": []any{map[string]any{"index": calls, "function": map[string]any{"arguments": blk.Args}}}}, nil)
			calls++
		default:
			return "", false
		}
	}
	failure := func(code int, kind, message string) (string, bool) {
		usage()
		frame(map[string]any{
			"error":   map[string]any{"code": code, "message": message, "metadata": map[string]any{"error_type": kind}},
			"choices": []any{map[string]any{"index": 0, "delta": map[string]any{"content": ""}, "finish_reason": "error"}},
		})
		return b.String(), true
	}
	switch s.Stop {
	case "":
	case api.CodeCapped:
		finish = "length"
	case api.CodeRefused:
		finish = "content_filter"
	case api.CodeContextLength:
		return failure(400, "context_length_exceeded", "This endpoint's maximum context length is 131072 tokens.")
	case api.CodeNetwork:
		return failure(502, "provider_unavailable", "Provider disconnected mid-stream")
	case api.CodeBusy:
		return failure(429, "rate_limit_exceeded", "Rate limit exceeded")
	default:
		return "", false
	}
	delta(map[string]any{}, finish)
	usage()
	b.WriteString("data: [DONE]\n\n")
	return b.String(), true
}

func genericErrors() []conform.ErrorCase {
	router := func(code int, message string) string {
		return fmt.Sprintf(`{"error":{"message":%q,"code":%d}}`, message, code)
	}
	llama := func(code int, kind, message string) string {
		return fmt.Sprintf(`{"error":{"code":%d,"message":%q,"type":%q}}`, code, message, kind)
	}
	inStream := func(code int, kind, message string) string {
		return fmt.Sprintf(`data: {"id":"gen-1","object":"chat.completion.chunk","created":1,"model":"m","provider":"P","error":{"code":%d,"message":%q,"metadata":{"error_type":%q}},"choices":[{"index":0,"delta":{"content":""},"finish_reason":"error"}]}`+"\n\n", code, message, kind)
	}
	return []conform.ErrorCase{
		{Name: "openrouter-bad-model", Status: 400, Body: `{"error":{"message":"no-such/model-conform is not a valid model ID","code":400},"user_id":"user_x"}`, Want: api.CodeInvalid, Source: docORErrors},
		{Name: "openrouter-context", Status: 400, Body: `{"error":{"message":"This endpoint's maximum context length is 131072 tokens. However, you requested about 187521 tokens (187505 of text input, 16 in the output). Please reduce the length of the input, or use the context-compression plugin to compress your prompt automatically.","code":400,"metadata":{"provider_name":null}}}`, Want: api.CodeContextLength, Source: docORErrors},
		{Name: "openrouter-bad-key", Status: 401, Body: router(401, "User not found."), Want: api.CodeAccess, Source: docORErrors},
		{Name: "openrouter-credits", Status: 402, Body: router(402, "Insufficient credits"), Want: api.CodeAccess, Source: docORErrors},
		{Name: "openrouter-moderation", Status: 403, Body: router(403, "Input was flagged"), Want: api.CodeAccess, Source: docORErrors},
		{Name: "openrouter-timeout", Status: 408, Body: router(408, "Request timed out"), Want: api.CodeNetwork, Source: docORErrors},
		{Name: "openrouter-rate-limit", Status: 429, Header: http.Header{"Retry-After": {"3"}}, Body: router(429, "Rate limit exceeded"), Want: api.CodeBusy, Source: docORErrors},
		{Name: "openrouter-model-down", Status: 502, Body: router(502, "Provider returned error"), Want: api.CodeNetwork, Source: docORErrors},
		{Name: "openrouter-no-provider", Status: 503, Body: router(503, "No endpoints found"), Want: api.CodeBusy, Source: docORErrors},
		{Name: "openrouter-mid-stream-rate-limit", Stream: inStream(429, "rate_limit_exceeded", "Rate limit exceeded"), Want: api.CodeBusy, Source: docORErrors},
		{Name: "openrouter-mid-stream-refusal", Stream: inStream(403, "refusal", "The model refused to respond"), Want: api.CodeRefused, Source: docORErrors},
		{Name: "openrouter-mid-stream-provider", Stream: inStream(502, "provider_unavailable", "Provider disconnected mid-stream"), Want: api.CodeNetwork, Source: docORErrors},
		{Name: "openrouter-upstream-tool-validation", Status: 502, Body: `{"error":{"message":"` + upstreamToolValidation + `","code":502,"metadata":{"provider_name":"Groq"}}}`, Want: api.CodeNetwork, Source: docORErrors},
		{Name: "openrouter-mid-stream-upstream", Stream: `data: {"id":"gen-1","object":"chat.completion.chunk","created":1,"model":"openai/gpt-oss-20b","provider":"Groq","system_fingerprint":"fp_aef561cf8e","choices":[],"error":{"code":502,"message":"` + upstreamToolValidation + `","metadata":{"error_type":"provider_unavailable"}}}` + "\n\n", Want: api.CodeNetwork, Source: docORErrors},
		{Name: "llama-context", Status: 400, Body: `{"error":{"code":400,"message":"request (40000 tokens) exceeds the available context size (32768 tokens), try increasing it","type":"exceed_context_size_error","n_prompt_tokens":40000,"n_ctx":32768}}`, Want: api.CodeContextLength, Source: docLlamaSource},
		{Name: "llama-bad-key", Status: 401, Body: llama(401, "authentication_error", "Invalid API Key"), Want: api.CodeAccess, Source: docLlama},
		{Name: "llama-image-to-text-model", Status: 500, Body: llama(500, "server_error", "image input is not supported - hint: if this is unexpected, you may need to provide the mmproj"), Want: api.CodeInvalid, Source: docLlamaSource},
		{Name: "llama-loading", Status: 503, Body: llama(503, "unavailable_error", "Loading model"), Want: api.CodeBusy, Source: docLlama},
		{Name: "llama-not-supported", Status: 501, Body: llama(501, "not_supported_error", "This server does not support this feature."), Want: api.CodeInvalid, Source: docLlama},
		{Name: "llama-grammar", Status: 400, Body: llama(400, "invalid_request_error", "Failed to parse grammar"), Want: api.CodeInvalid, Source: docLlama},
		{Name: "openai-context", Status: 400, Body: `{"error":{"message":"This model's maximum context length is 32768 tokens.","type":"invalid_request_error","param":"messages","code":"context_length_exceeded"}}`, Want: api.CodeContextLength, Schema: true, Source: docErrorCodes},
		{Name: "openai-server", Status: 500, Body: `{"error":{"message":"The server had an error while processing your request.","type":"server_error","param":null,"code":null}}`, Want: api.CodeNetwork, Schema: true, Source: docErrorCodes},
		{Name: "openai-quota", Status: 429, Body: `{"error":{"message":"You exceeded your current quota, please check your plan and billing details.","type":"insufficient_quota","param":null,"code":"insufficient_quota"}}`, Want: api.CodeAccess, Schema: true, Source: docErrorCodes},
		{Name: "too-large", Status: 413, Body: "payload too large", Want: api.CodeTooLarge, Source: docErrorCodes},
		{Name: "in-stream-typed", Stream: "data: {\"error\":{\"message\":\"Rate limit reached.\",\"type\":\"rate_limit_error\",\"code\":\"rate_limit_exceeded\"}}\n\n", Want: api.CodeBusy, Source: docErrorCodes},
	}
}
