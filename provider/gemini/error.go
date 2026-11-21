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
	"bytes"
	"encoding/json"
	"net/http"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// APIError is a failure Google reported about the request: the body of a non
// 200 response, an error event inside a stream, or an interaction that ended
// without a reply to keep. Status is zero for the last two.
//
// Code is the API's snake_case error code, such as "invalid_request", the
// reason of a key failure, such as "API_KEY_INVALID", or the interaction
// status a turn ended on, such as "incomplete". Type is the google.rpc status,
// such as "INVALID_ARGUMENT", which only key failures carry. RetryAfter is the
// Retry-After header's wait, sent on 429 and 503. The API sends no request id.
type APIError = lib.APIError

func readAPIError(h *lib.HTTPError) *APIError {
	e := &APIError{Provider: providerName, Status: h.Status, RetryAfter: lib.RetryAfter(h.Header)}
	body := bytes.TrimSpace(h.Body)
	var detail *errorDetail
	switch {
	case bytes.HasPrefix(body, []byte("event:")):
		if _, data, ok := bytes.Cut(body, []byte("\ndata:")); ok {
			data, _, _ = bytes.Cut(data, []byte("\n"))
			var b errorBody
			if json.Unmarshal(data, &b) == nil {
				detail = b.Error
			}
		}
	case bytes.HasPrefix(body, []byte("[")):
		var bs []errorBody
		if json.Unmarshal(body, &bs) == nil && len(bs) > 0 {
			detail = bs[0].Error
		}
	default:
		var b errorBody
		if json.Unmarshal(body, &b) == nil {
			detail = b.Error
		}
	}
	if detail == nil {
		e.Message = lib.ErrorMessage(h.Body)
		return e
	}
	e.Message = detail.Message
	if json.Unmarshal(detail.Code, &e.Code) == nil {
		return e
	}
	e.Type = detail.Status
	for _, d := range detail.Details {
		if strings.HasSuffix(d.Type, "ErrorInfo") {
			e.Code = d.Reason
			break
		}
	}
	return e
}

func (c *converter) httpError(h *lib.HTTPError) api.EventErr {
	e := readAPIError(h)
	code := statusCode(e)
	if code == api.CodeTooLarge {
		return api.EventErr{Code: code, Err: lib.MeasuredTooLarge(c.model, c.sent, c.declared, e)}
	}
	return api.EventErr{Code: code, Err: e}
}

func statusCode(e *APIError) api.ErrCode {
	switch e.Status {
	case http.StatusBadRequest:
		if strings.HasPrefix(e.Code, "API_KEY_") {
			return api.CodeAccess
		}
		if v := vocabulary(e.Code); v == api.CodeAccess || v == api.CodeRefused {
			return v
		}
		if contextOverflow(e.Message) {
			return api.CodeContextLength
		}
		if sizeRefusal(e.Message) {
			return api.CodeTooLarge
		}
		return api.CodeInvalid
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return api.CodeAccess
	case http.StatusNotFound, http.StatusRequestedRangeNotSatisfiable, http.StatusUnprocessableEntity:
		return api.CodeInvalid
	case http.StatusConflict:
		if vocabulary(e.Code) == api.CodeNetwork {
			return api.CodeNetwork
		}
		return api.CodeInvalid
	case http.StatusRequestTimeout:
		return api.CodeNetwork
	case http.StatusRequestEntityTooLarge:
		return api.CodeTooLarge
	case http.StatusTooManyRequests:
		if vocabulary(e.Code) == api.CodeAccess || dailyLimit(e.Message) {
			return api.CodeAccess
		}
		return api.CodeBusy
	case http.StatusNotImplemented:
		return api.CodeInvalid
	case http.StatusServiceUnavailable:
		return api.CodeBusy
	}
	if e.Status >= 500 {
		return api.CodeNetwork
	}
	return api.CodeUnknown
}

func (c *converter) inBandError(w *wireError) api.EventErr {
	e := &APIError{Provider: providerName}
	if w != nil {
		e.Code, e.Message = w.Code, w.Message
	}
	code := vocabulary(e.Code)
	switch {
	case code == api.CodeInvalid && contextOverflow(e.Message):
		code = api.CodeContextLength
	case code == api.CodeInvalid && sizeRefusal(e.Message):
		return api.EventErr{Code: api.CodeTooLarge, Err: lib.MeasuredTooLarge(c.model, c.sent, c.declared, e)}
	case code == api.CodeBusy && dailyLimit(e.Message):
		code = api.CodeAccess
	}
	return api.EventErr{Code: code, Err: e}
}

func vocabulary(code string) api.ErrCode {
	switch code {
	case "invalid_request", "parameter_unknown", "not_found", "model_not_found", "already_exists",
		"out_of_range", "unimplemented", "missing_thought_signature":
		return api.CodeInvalid
	case "failed_precondition", "authentication", "payment_required", "permission_denied", "quota_exceeded":
		return api.CodeAccess
	case "rate_limit_exceeded", "too_many_requests", "service_unavailable":
		return api.CodeBusy
	case "api_error", "deadline_exceeded", "gateway_timeout", "aborted":
		return api.CodeNetwork
	case "safety", "recitation", "language", "prohibited_content", "spii", "blocklist", "image_safety",
		"image_prohibited_content", "image_recitation", "image_other", "content_blocked", "jailbreak":
		return api.CodeRefused
	}
	return api.CodeUnknown
}

func contextOverflow(message string) bool {
	return strings.Contains(message, "input token count")
}

func sizeRefusal(message string) bool {
	return strings.Contains(message, "payload size exceeds the limit")
}

func dailyLimit(message string) bool {
	return strings.Contains(message, "per day")
}
