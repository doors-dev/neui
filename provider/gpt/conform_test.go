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

package gpt

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
	"neui.dev/provider/lib"
)

const (
	docModels    = "https://developers.openai.com/api/docs/models"
	docReasoning = "https://developers.openai.com/api/docs/guides/reasoning"
	docLatest    = "https://developers.openai.com/api/docs/guides/latest-model"
	docVision    = "https://developers.openai.com/api/docs/guides/images-vision"
	docCreate    = "https://developers.openai.com/api/reference/resources/responses/methods/create"
	docErrors    = "https://developers.openai.com/api/docs/guides/error-codes"
)

func TestConform(t *testing.T) {
	conform.Run(t, conformAdapter())
}

func conformAdapter() conform.Adapter {
	return conform.Adapter{
		Name:   providerName,
		Models: []api.Model{GPT56Sol, GPT56Luna, GPT6Astra, GPT6Sol, GPT6Luna, GPT61Sol, GPT62Astra, GPT62Sol, GPT61Terra},
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
			"effort":      "/reasoning/effort",
			"off":         "/reasoning/effort",
			"choice":      "/tool_choice",
			"temperature": "/temperature",
			"top_p":       "/top_p",
			"max_tokens":  "/max_output_tokens",
		},
		Spec: conform.Spec{
			File:       "openai.json.gz",
			Request:    "/components/schemas/CreateResponse",
			Count:      "/components/schemas/TokenCountsBody",
			Event:      "/components/schemas/ResponseStreamEvent",
			Error:      "/components/schemas/ErrorResponse",
			LooseOneOf: true,
			Defaults: map[string]any{
				"/tools/*/strict": nil,
			},
			Accept: []conform.Accept{{
				At: "/components/schemas/EasyInputMessage",
				Also: map[string]any{
					"type":     "object",
					"required": []any{"role", "content"},
					"properties": map[string]any{
						"role":  map[string]any{"enum": []any{"assistant"}},
						"phase": map[string]any{"enum": []any{"commentary", "final_answer", nil}},
						"type":  map[string]any{"enum": []any{"message"}},
						"content": map[string]any{"type": "array", "items": map[string]any{
							"type":                 "object",
							"required":             []any{"type", "text"},
							"additionalProperties": false,
							"properties": map[string]any{
								"type": map[string]any{"enum": []any{"output_text"}},
								"text": map[string]any{"type": "string"},
							},
						}},
					},
				},
				Source: "live probe of /v1/responses/input_tokens on gpt-5.6-sol, 2026-11-06: an assistant message refuses input_text and takes output_text, with phase and with type message; the spec lists only input content there",
			}},
		},
		Render:   renderGPT,
		Messages: "/input",
		CrossCheck: func(t *testing.T, body []byte) {
			var req struct {
				Input []struct {
					Type   string `json:"type"`
					CallID string `json:"call_id"`
				} `json:"input"`
			}
			if err := json.Unmarshal(body, &req); err != nil {
				t.Fatal(err)
			}
			called, answered := map[string]bool{}, map[string]bool{}
			for _, it := range req.Input {
				switch it.Type {
				case "function_call":
					called[it.CallID] = true
				case "function_call_output":
					if !called[it.CallID] {
						t.Errorf("function_call_output %q comes before any function_call with its call_id", it.CallID)
					}
					answered[it.CallID] = true
				}
			}
			for id := range called {
				if !answered[id] {
					t.Errorf("function_call %q has no function_call_output", id)
				}
			}
		},
		Skipped: func(r api.ContentReasoning) bool {
			var item outputItem
			data, _ := lib.OwnMeta(r.Meta, providerName)
			return json.Unmarshal(data, &item) == nil && item.EncryptedContent == ""
		},
		Signed:   true,
		Sampling: true,
		Errors:   gptErrors(),
		Rules: []conform.Rule{
			{
				When:   func(p conform.Params) bool { return p.Sampling() && p.Effort != conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: docLatest,
			},
			{
				Model:  "gpt-6-astra",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: docReasoning,
			},
			{
				Model:  "gpt-6.1-sol",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: docReasoning,
			},
			{
				Model:  "gpt-6.2-astra",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: docReasoning,
			},
			{
				Model:  "gpt-6.2-sol",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: docReasoning,
			},
			{
				When:   func(p conform.Params) bool { return p.MaxTokens > 0 && p.MaxTokens < 16 },
				Want:   conform.RefuseNew,
				Source: docCreate,
			},
			{
				When:   func(p conform.Params) bool { return p.MaxTokens > maxOutputTokensCeiling },
				Want:   conform.RefuseNew,
				Source: docModels,
			},
			{
				When:   func(p conform.Params) bool { return p.Choice == api.ToolChoiceAny },
				Carry:  map[string]any{"/tool_choice": "required"},
				Source: docCreate,
			},
			{
				When:   func(p conform.Params) bool { return p.Choice == conform.ChoiceNamed },
				Carry:  map[string]any{"/tool_choice": map[string]any{"type": "function", "name": "lookup"}},
				Source: docCreate,
			},
			{
				Carry:  map[string]any{"/store": false, "/include": []string{"reasoning.encrypted_content"}},
				Source: docReasoning,
			},
			{Entry: "image-no-mime", Carry: map[string]any{"/input/0/content/1/type": "input_text"}, Source: docVision},
			{Entry: "tool-result-image-bad-mime", Carry: map[string]any{"/input/2/output/0/type": "input_text"}, Source: docVision},
			{Entry: "tool-error", Carry: map[string]any{"/input/2/output/0/text": "Error:\n"}, Source: docCreate},
			{Entry: "tool-discarded-arguments", Carry: map[string]any{"/input/1/arguments": "{}"}, Source: docCreate},
		},
		Live: conform.Live{
			Env:    "OPENAI_API_KEY",
			Model:  GPT56Luna,
			Params: conform.Params{MaxTokens: 1024, Effort: conform.EffortLow},
			Base:   defaultBaseURL,
			Count:  "/responses/input_tokens",
			Strip:  []string{"/stream", "/stream_options", "/store", "/include", "/max_output_tokens", "/service_tier", "/temperature", "/top_p", "/metadata", "/prompt_cache_key", "/prompt_cache_options"},
			Auth: func(r *http.Request, key string) {
				r.Header.Set("Authorization", "Bearer "+key)
			},
		},
	}
}

func renderGPT(m api.Model, s conform.Script) (string, bool) {
	var b strings.Builder
	seq := 0
	frame := func(v map[string]any) {
		v["sequence_number"] = seq
		seq++
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", v["type"], data)
	}
	response := func(status string) map[string]any {
		return map[string]any{
			"id": "resp_01", "object": "response", "created_at": 1, "model": m.Name, "status": status,
			"error": nil, "incomplete_details": nil, "instructions": nil, "metadata": map[string]any{},
			"output": []any{}, "parallel_tool_calls": true, "temperature": nil, "top_p": nil,
			"tool_choice": "auto", "tools": []any{}, "access_programs": nil,
		}
	}
	frame(map[string]any{"type": "response.created", "response": response("in_progress")})
	for i, blk := range s.Blocks {
		added := func(item map[string]any) {
			frame(map[string]any{"type": "response.output_item.added", "output_index": i, "item": item})
		}
		done := func(item map[string]any) {
			frame(map[string]any{"type": "response.output_item.done", "output_index": i, "item": item})
		}
		switch blk.Kind {
		case conform.KindText:
			id := fmt.Sprintf("msg_%d", i)
			part := map[string]any{"type": "output_text", "text": "", "annotations": []any{}, "logprobs": []any{}}
			added(map[string]any{"type": "message", "id": id, "status": "in_progress", "role": "assistant", "content": []any{}})
			frame(map[string]any{"type": "response.content_part.added", "item_id": id, "output_index": i, "content_index": 0, "part": part})
			frame(map[string]any{"type": "response.output_text.delta", "item_id": id, "output_index": i, "content_index": 0, "delta": blk.Text, "logprobs": []any{}})
			frame(map[string]any{"type": "response.output_text.done", "item_id": id, "output_index": i, "content_index": 0, "text": blk.Text, "logprobs": []any{}})
			full := map[string]any{"type": "output_text", "text": blk.Text, "annotations": []any{}, "logprobs": []any{}}
			frame(map[string]any{"type": "response.content_part.done", "item_id": id, "output_index": i, "content_index": 0, "part": full})
			done(map[string]any{"type": "message", "id": id, "status": "completed", "role": "assistant", "content": []any{full}})
		case conform.KindThinking, conform.KindSummary:
			id := fmt.Sprintf("rs_%d", i)
			added(map[string]any{"type": "reasoning", "id": id, "summary": []any{}})
			item := map[string]any{"type": "reasoning", "id": id, "summary": []any{}, "encrypted_content": blk.Signature}
			if blk.Kind == conform.KindThinking {
				frame(map[string]any{"type": "response.reasoning_text.delta", "item_id": id, "output_index": i, "content_index": 0, "delta": blk.Text})
				item["content"] = []any{map[string]any{"type": "reasoning_text", "text": blk.Text}}
			} else {
				frame(map[string]any{"type": "response.reasoning_summary_text.delta", "item_id": id, "output_index": i, "summary_index": 0, "delta": blk.Text})
				item["summary"] = []any{map[string]any{"type": "summary_text", "text": blk.Text}}
			}
			done(item)
		case conform.KindTool:
			id := fmt.Sprintf("fc_%d", i)
			added(map[string]any{"type": "function_call", "id": id, "call_id": blk.ID, "name": blk.Name, "arguments": "", "status": "in_progress"})
			frame(map[string]any{"type": "response.function_call_arguments.delta", "item_id": id, "output_index": i, "delta": blk.Args})
			frame(map[string]any{"type": "response.function_call_arguments.done", "item_id": id, "output_index": i, "arguments": blk.Args})
			done(map[string]any{"type": "function_call", "id": id, "call_id": blk.ID, "name": blk.Name, "arguments": blk.Args, "status": "completed"})
		case conform.KindOpaque:
			id := fmt.Sprintf("ws_%d", i)
			action := map[string]any{"type": "search", "query": "cats"}
			added(map[string]any{"type": "web_search_call", "id": id, "status": "in_progress", "action": action})
			done(map[string]any{"type": "web_search_call", "id": id, "status": "completed", "action": action})
		default:
			return "", false
		}
	}
	end := response("completed")
	end["usage"] = map[string]any{
		"input_tokens": conform.UsageInput, "input_tokens_details": map[string]any{"cached_tokens": 0, "cache_write_tokens": 0},
		"output_tokens": conform.UsageOutput, "output_tokens_details": map[string]any{"reasoning_tokens": 0},
		"total_tokens": conform.UsageInput + conform.UsageOutput,
	}
	switch s.Stop {
	case "":
		frame(map[string]any{"type": "response.completed", "response": end})
		return b.String(), true
	case api.CodeCapped:
		end["incomplete_details"] = map[string]any{"reason": "max_output_tokens"}
	case api.CodeRefused:
		end["incomplete_details"] = map[string]any{"reason": "content_filter"}
	default:
		return "", false
	}
	end["status"] = "incomplete"
	frame(map[string]any{"type": "response.incomplete", "response": end})
	return b.String(), true
}

func gptErrors() []conform.ErrorCase {
	body := func(kind, code, message string) string {
		return fmt.Sprintf(`{"error":{"message":%q,"type":%q,"param":null,"code":%q}}`, message, kind, code)
	}
	return []conform.ErrorCase{
		{Name: "invalid", Status: 400, Body: body("invalid_request_error", "invalid_value", "Invalid value for 'input'."), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "context-length", Status: 400, Body: body("invalid_request_error", "context_length_exceeded", "Your input exceeds the context window of this model."), Want: api.CodeContextLength, Schema: true, Source: docErrors},
		{Name: "invalid-api-key", Status: 401, Body: body("invalid_request_error", "invalid_api_key", "Incorrect API key provided."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "region", Status: 403, Body: body("invalid_request_error", "unsupported_country_region_territory", "Country, region, or territory not supported."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "model-not-found", Status: 404, Body: body("invalid_request_error", "model_not_found", "The model does not exist."), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "too-large", Status: 413, Body: "payload too large", Want: api.CodeTooLarge, Source: docErrors},
		{Name: "rate-limit", Status: 429, Body: body("requests", "rate_limit_exceeded", "Rate limit reached for requests."), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "slow-down", Status: 429, Body: body("requests", "slow_down", "Please reduce your rate of requests."), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "insufficient-quota", Status: 429, Body: body("insufficient_quota", "insufficient_quota", "You exceeded your current quota."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "credit-exhausted", Status: 429, Body: body("insufficient_quota", "credit_balance_exhausted", "Your credit balance is exhausted."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "org-spend-limit", Status: 429, Body: body("insufficient_quota", "organization_spend_limit_exceeded", "Your organization has reached its spend limit."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "project-spend-limit", Status: 429, Body: body("insufficient_quota", "project_spend_limit_exceeded", "Your project has reached its spend limit."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "org-usage-limit", Status: 429, Body: body("insufficient_quota", "organization_usage_limit_exceeded", "Your organization has reached its usage limit."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "server-error", Status: 500, Body: body("server_error", "server_error", "The server had an error."), Want: api.CodeNetwork, Schema: true, Source: docErrors},
		{Name: "overloaded", Status: 503, Body: body("server_error", "server_is_overloaded", "The server is overloaded."), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "in-stream-overloaded", Stream: "event: error\ndata: {\"type\":\"error\",\"code\":\"server_is_overloaded\",\"message\":\"The server is overloaded.\",\"param\":null,\"sequence_number\":0}\n\n", Want: api.CodeBusy, Source: docErrors},
		{Name: "response-failed", Stream: "event: response.failed\ndata: {\"type\":\"response.failed\",\"sequence_number\":0,\"response\":{\"id\":\"resp_01\",\"status\":\"failed\",\"error\":{\"code\":\"server_error\",\"message\":\"The server had an error.\"}}}\n\n", Want: api.CodeNetwork, Source: docErrors},
	}
}
