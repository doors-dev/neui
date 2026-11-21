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

package gemini

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
	docErrors     = "https://aistudio.google.com/docs/api-errors"
	docThinking   = "https://aistudio.google.com/docs/thinking"
	docFunctions  = "https://aistudio.google.com/docs/function-calling"
	docCreate     = "https://aistudio.google.com/api-reference/interactions-api/create-interaction?api-version=v1"
	docStreaming  = "https://aistudio.google.com/docs/streaming"
	docLongDecode = "https://aistudio.google.com/docs/long-decode"
	docSafety     = "https://aistudio.google.com/docs/safety-settings"
	docDocuments  = "https://aistudio.google.com/docs/document-processing"
	docSignatures = "https://aistudio.google.com/docs/thought-signatures"
)

func TestConform(t *testing.T) {
	conform.Run(t, conformAdapter())
}

func conformAdapter() conform.Adapter {
	return conform.Adapter{
		Name:   providerName,
		Models: []api.Model{Hydrogen4, Neon4, Carbon4, Argon4, Flash38, FlashLite35},
		New: func(base string, m api.Model, p conform.Params) (api.Provider, error) {
			key := p.Key
			if key == "" {
				key = "test-key"
			}
			opts := []Option{WithAPIKey(key), WithBaseURL(base)}
			switch p.Effort {
			case "":
			case conform.EffortOff:
				opts = append(opts, WithEffort(EffortMinimal))
			default:
				opts = append(opts, WithEffort(Effort(p.Effort)))
			}
			if p.MaxTokens != 0 {
				opts = append(opts, WithMaxTokens(p.MaxTokens))
			}
			return New(Conf{Model: m}, opts...)
		},
		Wire: map[string]string{
			"effort":     "/generation_config/thinking_level",
			"off":        "/generation_config/thinking_level",
			"choice":     "/generation_config/tool_choice",
			"max_tokens": "/generation_config/max_output_tokens",
		},
		Spec: conform.Spec{
			File:    "gemini.json.gz",
			Request: "/components/schemas/CreateModelInteractionParams",
			Event:   "/components/schemas/InteractionSseEvent",
			Error:   "/paths/~1{api_version}~1interactions/post/responses/4XX/content/application~1json/schema",
			Version: "Api-Revision",
			Accept: []conform.Accept{
				{
					At:     "/components/schemas/ModelOption",
					Also:   map[string]any{"type": "string"},
					Source: "the spec marks the enum x-speakeasy-unknown-values: allow, and gemini-4-neon is served though missing from it (live 404 hint, 2026-11-19; https://aistudio.google.com/docs/models)",
				},
				{
					At: "/components/schemas/InteractionSseEventInteraction",
					Also: map[string]any{"type": "object", "properties": map[string]any{
						"status":       map[string]any{"type": "string"},
						"model":        map[string]any{"type": "string"},
						"usage":        map[string]any{"$ref": "#/components/schemas/Usage"},
						"object":       map[string]any{"type": "string"},
						"created":      map[string]any{"type": "string"},
						"updated":      map[string]any{"type": "string"},
						"service_tier": map[string]any{"type": "string"},
					}},
					Source: "live capture 2026-11-19 with store false: interaction.created carries only object and model, interaction.completed no id",
				},
				{
					At: "/components/schemas/InteractionStatusUpdate",
					Also: map[string]any{"type": "object", "required": []any{"event_type", "status"}, "properties": map[string]any{
						"event_type": map[string]any{"const": "interaction.status_update"},
						"status":     map[string]any{"type": "string"},
						"usage":      map[string]any{"$ref": "#/components/schemas/Usage"},
					}},
					Source: "live capture 2026-11-19 with store false: no interaction_id",
				},
				{
					At: "/components/schemas/FunctionResultStep/properties/result",
					Also: map[string]any{"type": "array", "items": map[string]any{"oneOf": []any{
						map[string]any{"$ref": "#/components/schemas/DocumentContent"},
						map[string]any{"$ref": "#/components/schemas/ImageContent"},
						map[string]any{"$ref": "#/components/schemas/TextContent"},
					}}},
					Source: "https://aistudio.google.com/docs/function-calling#multimodal and the 2026-11-05 changelog (https://aistudio.google.com/docs/changelog) add document blocks to function_result; live on v1 Hydrogen 2026-11-19 a PDF in a result was read",
				},
			},
		},
		Render:   renderGemini,
		Errors:   geminiErrors(),
		Signed:   true,
		Sampling: false,
		Unframed: false,
		Rules: []conform.Rule{
			{
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortMax },
				Want:   conform.RefuseNew,
				Source: docThinking,
			},
			{
				Model:  "gemini-4",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: "https://aistudio.google.com/docs/whats-new-gemini-4-hydrogen#migrate-from-gemini-3-8-flash",
			},
			{
				Model:  "gemini-3.8-flash",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Want:   conform.RefuseNew,
				Source: "https://aistudio.google.com/docs/whats-new-gemini-3.8",
			},
			{
				Model:  "gemini-3.5-flash-lite",
				When:   func(p conform.Params) bool { return p.Effort == conform.EffortOff },
				Carry:  map[string]any{"/generation_config/thinking_level": "minimal"},
				Source: docThinking,
			},
			{
				When:   func(p conform.Params) bool { return p.MaxTokens > maxOutput },
				Want:   conform.RefuseNew,
				Source: docLongDecode + "#output-limits",
			},
			{
				When:   func(p conform.Params) bool { return p.Choice == api.ToolChoiceAny },
				Carry:  map[string]any{"/generation_config/tool_choice": "any"},
				Source: docFunctions,
			},
			{
				When:   func(p conform.Params) bool { return p.Choice == conform.ChoiceNamed },
				Carry:  map[string]any{"/generation_config/tool_choice": map[string]any{"allowed_tools": map[string]any{"mode": "any", "tools": []any{"lookup"}}}},
				Source: docFunctions,
			},
			{
				Carry:  map[string]any{"/stream": true, "/store": false, "/generation_config/thinking_summaries": "auto"},
				Source: docCreate,
			},
			{Entry: "tools", Carry: map[string]any{"/input/1": map[string]any{"type": "thought", "signature": skipSignature}, "/input/4/name": "lookup"}, Source: docSignatures},
			{Entry: "tool-no-input", Carry: map[string]any{"/input/1/signature": skipSignature, "/input/2/arguments": map[string]any{}}, Source: docCreate},
			{Entry: "tool-discarded-arguments", Carry: map[string]any{"/input/2/arguments": map[string]any{}}, Source: docCreate},
			{Entry: "tool-error", Carry: map[string]any{"/input/3/is_error": true}, Source: docFunctions},
			{Entry: "tool-result-empty", Carry: map[string]any{"/input/3/result/0/text": lib.EmptyResult}, Source: docFunctions},
			{Entry: "tool-result-image", Carry: map[string]any{"/input/3/result/1/type": "image"}, Source: docFunctions + "#multimodal"},
			{Entry: "tool-result-image-bad-mime", Carry: map[string]any{"/input/3/result/0/type": "text"}, Source: docFunctions + "#multimodal"},
			{Entry: "tool-result-document", Carry: map[string]any{"/input/3/result/0/type": "document", "/input/3/result/0/mime_type": "application/pdf"}, Source: docFunctions + "#multimodal"},
			{Entry: "image-no-mime", Carry: map[string]any{"/input/0/content/1/type": "text"}, Source: "https://aistudio.google.com/docs/image-understanding"},
			{Entry: "image-no-bytes", Carry: map[string]any{"/input/0/content/1/type": "text"}, Source: "https://aistudio.google.com/docs/image-understanding"},
			{Entry: "documents", Carry: map[string]any{"/input/0/content/1/type": "document", "/input/0/content/2": map[string]any{"type": "text", "text": "plain words"}}, Source: docDocuments},
			{Entry: "document-csv", Carry: map[string]any{"/input/0/content/1/mime_type": "text/csv"}, Source: docDocuments},
			{Entry: "document-no-bytes", Carry: map[string]any{"/input/0/content/1/text": "[document omitted]"}, Source: docDocuments},
			{Entry: "whitespace-text", Carry: map[string]any{"/input/0/content/1/text": "  "}, Source: docCreate},
			{Entry: "tools-text-turns", Carry: map[string]any{"/input/1": map[string]any{"type": "model_output", "content": []any{map[string]any{"type": "text", "text": "[reasoning omitted]"}, map[string]any{"type": "text", "text": "hello"}}}}, Source: docSignatures},
			{Entry: "reasoning-unsigned", Carry: map[string]any{"/input/1": map[string]any{"type": "model_output", "content": []any{map[string]any{"type": "text", "text": "[reasoning omitted]"}, map[string]any{"type": "text", "text": "hello"}}}}, Source: docSignatures},
		},
		Live: conform.Live{
			Env:    "GEMINI_API_KEY",
			Model:  Hydrogen4,
			Params: conform.Params{MaxTokens: 1024, Effort: conform.EffortLow},
			Base:   defaultBaseURL,
			Auth: func(r *http.Request, key string) {
				r.Header.Set("x-goog-api-key", key)
				r.Header.Set("Api-Revision", defaultRevision)
			},
		},
	}
}

func renderGemini(m api.Model, s conform.Script) (string, bool) {
	status := "completed"
	switch s.Stop {
	case "":
	case api.CodeCapped:
		status = "incomplete"
	default:
		return "", false
	}
	var b strings.Builder
	frame := func(v map[string]any) {
		data, err := json.Marshal(v)
		if err != nil {
			panic(err)
		}
		fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", v["event_type"], data)
	}
	frame(map[string]any{"event_type": "interaction.created", "interaction": map[string]any{"object": "interaction", "model": m.Name}})
	frame(map[string]any{"event_type": "interaction.status_update", "status": "in_progress"})
	i := 0
	if len(s.Blocks) == 0 || s.Blocks[0].Kind != conform.KindSummary {
		frame(map[string]any{"event_type": "step.start", "index": i, "step": map[string]any{"type": "thought"}})
		frame(map[string]any{"event_type": "step.stop", "index": i})
		i++
	}
	for _, blk := range s.Blocks {
		start := func(st map[string]any) {
			frame(map[string]any{"event_type": "step.start", "index": i, "step": st})
		}
		delta := func(d map[string]any) {
			frame(map[string]any{"event_type": "step.delta", "index": i, "delta": d})
		}
		switch blk.Kind {
		case conform.KindText:
			start(map[string]any{"type": "model_output"})
			delta(map[string]any{"type": "text", "text": blk.Text})
		case conform.KindSummary:
			start(map[string]any{"type": "thought"})
			delta(map[string]any{"type": "thought_summary", "content": map[string]any{"type": "text", "text": blk.Text}})
			delta(map[string]any{"type": "thought_signature", "signature": blk.Signature})
		case conform.KindTool:
			start(map[string]any{"type": "function_call", "id": blk.ID, "name": blk.Name, "arguments": map[string]any{}})
			delta(map[string]any{"type": "arguments_delta", "arguments": blk.Args})
			if status == "completed" {
				status = "requires_action"
			}
		case conform.KindOpaque:
			start(map[string]any{"type": "google_search_call", "id": "gs_1", "arguments": map[string]any{}, "signature": ""})
			delta(map[string]any{"type": "google_search_call", "signature": "c2lnbmF0dXJlLTM=", "arguments": map[string]any{"queries": []any{"cats"}}})
		default:
			return "", false
		}
		frame(map[string]any{"event_type": "step.stop", "index": i})
		i++
	}
	frame(map[string]any{"event_type": "interaction.completed", "interaction": map[string]any{
		"status": status,
		"usage": map[string]any{
			"total_input_tokens": conform.UsageInput, "total_output_tokens": conform.UsageOutput,
			"total_thought_tokens": 0, "total_cached_tokens": 0, "total_tokens": conform.UsageInput + conform.UsageOutput,
		},
		"created": "2026-11-19T00:00:00Z", "updated": "2026-11-19T00:00:00Z", "service_tier": "standard",
		"object": "interaction", "model": m.Name,
	}})
	b.WriteString("event: done\ndata: [DONE]\n\n")
	return b.String(), true
}

func geminiErrors() []conform.ErrorCase {
	body := func(code, message string) string {
		return fmt.Sprintf(`{"error":{"message":%q,"code":%q}}`, message, code)
	}
	stream := func(frames ...string) string {
		var b strings.Builder
		for _, f := range frames {
			var head frameHead
			if err := json.Unmarshal([]byte(f), &head); err != nil {
				panic(err)
			}
			fmt.Fprintf(&b, "event: %s\ndata: %s\n\n", head.EventType, f)
		}
		return b.String()
	}
	const created = `{"interaction":{"object":"interaction","model":"gemini-4-hydrogen"},"event_type":"interaction.created"}`
	rpc := func(status int, rpcStatus, reason, message string) string {
		return fmt.Sprintf(`[{
  "error": {
    "code": %d,
    "message": %q,
    "status": %q,
    "details": [
      {
        "@type": "type.googleapis.com/google.rpc.ErrorInfo",
        "reason": %q,
        "domain": "googleapis.com",
        "metadata": {
          "service": "generativelanguage.googleapis.com"
        }
      },
      {
        "@type": "type.googleapis.com/google.rpc.LocalizedMessage",
        "locale": "en-US",
        "message": %q
      }
    ]
  }
}
]`, status, message, rpcStatus, reason, message)
	}
	return []conform.ErrorCase{
		{Name: "invalid", Status: 400, Body: body("invalid_request", "Unknown parameter 'frobnicate'."), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "thinking-level", Status: 400, Body: body("invalid_request", "Thinking level THINKING_LEVEL_XHIGH is not supported for this model. Please retry with other thinking level."), Want: api.CodeInvalid, Schema: true, Source: docThinking},
		{Name: "missing-signature", Status: 400, Body: "event: error\ndata: {\"error\":{\"message\":\"Request contains an invalid argument.\",\"code\":\"invalid_request\"},\"event_type\":\"error\"}\n\n", Want: api.CodeInvalid, Source: docSignatures},
		{Name: "bad-key", Status: 400, Body: rpc(400, "INVALID_ARGUMENT", "API_KEY_INVALID", "API key not valid. Please pass a valid API key."), Want: api.CodeAccess, Source: "https://aistudio.google.com/docs/troubleshooting"},
		{Name: "no-key", Status: 403, Body: rpc(403, "PERMISSION_DENIED", "ACCESS_DENIED", "Permission denied on the service."), Want: api.CodeAccess, Source: "https://aistudio.google.com/docs/troubleshooting"},
		{Name: "failed-precondition", Status: 400, Body: body("failed_precondition", "User location is not supported for the API use."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "authentication", Status: 401, Body: body("authentication", "Invalid API key."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "payment", Status: 402, Body: body("payment_required", "Billing is not enabled."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "not-found", Status: 404, Body: body("not_found", "Model 'gemini-9-nonexistent' not found. Did you mean 'gemini-pro-latest'? Please verify the model name against the supported list: https://ai.google.dev/gemini-api/docs/models"), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "context-length", Status: 400, Body: body("invalid_request", "The input token count (1210970) exceeds the maximum number of tokens allowed (1048576)."), Want: api.CodeContextLength, Schema: true, Source: "https://discuss.ai.google.dev/t/gemini-3-5-flash-the-input-token-count-n-exceeds-the-maximum-number-of-tokens-allowed-32768/170484"},
		{Name: "safety", Status: 400, Body: body("safety", "Input blocked: the prompt was flagged for harassment."), Want: api.CodeRefused, Schema: true, Source: docSafety + "#when-content-is-blocked"},
		{Name: "too-large", Status: 413, Body: "payload too large", Want: api.CodeTooLarge, Source: "https://aistudio.google.com/docs/file-input-methods#method-comparison"},
		{Name: "aborted", Status: 409, Body: body("aborted", "The operation was aborted."), Want: api.CodeNetwork, Schema: true, Source: docErrors},
		{Name: "already-exists", Status: 409, Body: body("already_exists", "The resource already exists."), Want: api.CodeInvalid, Schema: true, Source: docErrors},
		{Name: "rate-limit", Status: 429, Header: http.Header{"Retry-After": {"7"}}, Body: body("rate_limit_exceeded", "You have exceeded the rate limit."), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "too-many-requests", Status: 429, Body: body("too_many_requests", "You have made too many requests in a short period of time."), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "quota-exceeded", Status: 429, Body: body("quota_exceeded", "You have exceeded your daily quota."), Want: api.CodeAccess, Schema: true, Source: docErrors},
		{Name: "daily-quota", Status: 429, Header: http.Header{"Retry-After": {"49314"}}, Body: body("too_many_requests", "Rate limit exceeded for model gemini-4-hydrogen (limit: 20 requests per day on Free Tier). Please retry in 13h41m54s or upgrade your tier at https://ai.dev/rate-limit."), Want: api.CodeAccess, Schema: true, Source: "https://aistudio.google.com/docs/rate-limits"},
		{Name: "free-tier-locked", Status: 429, Header: http.Header{"Retry-After": {"46247"}}, Body: body("too_many_requests", "Rate limit exceeded for model gemini-4-neon (limit: 0 requests per day on Free Tier). Please upgrade your tier at https://ai.dev/rate-limit."), Want: api.CodeAccess, Schema: true, Source: "https://aistudio.google.com/docs/rate-limits"},
		{Name: "api-error", Status: 500, Body: body("api_error", "An internal error has occurred."), Want: api.CodeNetwork, Schema: true, Source: docErrors},
		{Name: "unavailable", Status: 503, Header: http.Header{"Retry-After": {"30"}}, Body: body("service_unavailable", "gemini-4-hydrogen is currently experiencing high demand, spikes in demand are usually temporary. Please try again later."), Want: api.CodeBusy, Schema: true, Source: docErrors},
		{Name: "timeout", Status: 504, Body: body("deadline_exceeded", "The request timed out."), Want: api.CodeNetwork, Schema: true, Source: docErrors},
		{Name: "unparseable", Status: 502, Body: "<html>bad gateway</html>", Want: api.CodeNetwork, Source: docErrors},
		{Name: "in-stream-unavailable", Stream: stream(created, `{"error":{"message":"The model is overloaded.","code":"service_unavailable"},"event_type":"error"}`), Want: api.CodeBusy, Source: docStreaming + "#event-error"},
		{Name: "in-stream-timeout", Stream: stream(created, `{"error":{"message":"Deadline expired before operation could complete.","code":"gateway_timeout"},"event_type":"error"}`), Want: api.CodeNetwork, Source: docStreaming + "#event-error"},
		{Name: "in-stream-safety", Stream: stream(created,
			`{"index":0,"step":{"type":"model_output"},"event_type":"step.start"}`,
			`{"index":0,"delta":{"text":"half","type":"text"},"event_type":"step.delta"}`,
			`{"error":{"message":"Output blocked for safety.","code":"safety"},"event_type":"error"}`), Want: api.CodeRefused, Source: docSafety + "#when-content-is-blocked"},
		{Name: "error-first", Stream: stream(`{"error":{"message":"You have exceeded the rate limit.","code":"rate_limit_exceeded"},"event_type":"error"}`), Want: api.CodeBusy, Source: docErrors},
	}
}
