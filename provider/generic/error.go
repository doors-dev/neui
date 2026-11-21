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
	"errors"
	"net/http"
	"strconv"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

var errTooLarge = errors.New("generic: request too large")

// APIError is a failure the server reported: a non 200 response, an error
// frame inside a stream that already returned 200, or a finish_reason that
// ended the turn without a reply to keep. Status is zero for the last two.
//
// Type, Code and Param come from the error object, which servers send as
// OpenAI's {"error":{...}} or as its fields at the top level. Code is
// OpenRouter's error_type where it sends one; otherwise the code field when it
// is a string, the status an error inside the stream carries as a number, and
// for a finish_reason the value as the server sent it. Message is the
// server's text, a gateway's upstream message included, or the trimmed body
// cut to 512 characters when the object gave none. RequestID is the
// X-Request-Id or X-Generation-Id header, or the stream's id for a failure
// inside it; RetryAfter is read from Retry-After.
type APIError = lib.APIError

func readError(e *APIError, raw []byte) int {
	var body errorBody
	if json.Unmarshal(raw, &body) != nil {
		var s string
		if json.Unmarshal(raw, &s) == nil {
			e.Message = s
		}
		return 0
	}
	e.Message, e.Type = body.Message, body.Type
	json.Unmarshal(body.Param, &e.Param)
	var status int
	var code string
	switch {
	case json.Unmarshal(body.Code, &status) == nil && status != 0:
		if e.Status == 0 {
			e.Code = strconv.Itoa(status)
		}
	case json.Unmarshal(body.Code, &code) == nil:
		e.Code = code
	}
	if body.Metadata.ErrorType != "" {
		e.Code = body.Metadata.ErrorType
	}
	return status
}

func errorCode(status int, e *APIError) api.ErrCode {
	text := strings.ToLower(e.Message)
	switch status {
	case 0, http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
		if e.Code == "context_length_exceeded" || e.Type == "context_length_exceeded" || e.Type == "exceed_context_size_error" ||
			strings.Contains(text, "context length") || strings.Contains(text, "context size") || strings.Contains(text, "context window") {
			return api.CodeContextLength
		}
	}
	if e.Code == "content_policy_violation" || e.Code == "refusal" {
		return api.CodeRefused
	}
	if strings.Contains(text, "input is not supported") || strings.Contains(text, "jinja exception") {
		return api.CodeInvalid
	}
	switch status {
	case 0:
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return api.CodeAccess
	case http.StatusTooManyRequests:
		if e.Code == "insufficient_quota" || e.Type == "insufficient_quota" {
			return api.CodeAccess
		}
		return api.CodeBusy
	case http.StatusServiceUnavailable:
		return api.CodeBusy
	case http.StatusRequestTimeout:
		return api.CodeNetwork
	case http.StatusRequestEntityTooLarge:
		return api.CodeTooLarge
	case http.StatusBadRequest, http.StatusNotFound, http.StatusConflict, http.StatusUnprocessableEntity, http.StatusNotImplemented:
		return api.CodeInvalid
	default:
		if status >= 500 {
			return api.CodeNetwork
		}
		return api.CodeUnknown
	}
	for _, v := range []string{e.Code, e.Type} {
		switch v {
		case "invalid_request_error", "invalid_request", "invalid_prompt", "not_found_error", "not_found", "model_not_found", "not_supported_error":
			return api.CodeInvalid
		case "authentication_error", "authentication", "invalid_api_key", "permission_error", "permission_denied", "payment_required", "insufficient_quota":
			return api.CodeAccess
		case "rate_limit_exceeded", "rate_limit_error", "provider_overloaded", "overloaded_error", "unavailable_error":
			return api.CodeBusy
		case "server_error", "server", "api_error", "provider_unavailable", "timeout":
			return api.CodeNetwork
		}
	}
	return api.CodeUnknown
}
