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
	"errors"
	"net/http"
	"strconv"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

var errTooLarge = errors.New("deepseek: request too large")

// APIError is a failure DeepSeek reported, as opposed to one the connection
// produced: a non 200 response, an error frame inside a stream that already
// returned 200, or a finish_reason that ended the turn without a reply to
// keep. Status is zero for the last two.
//
// Type and Code are the error envelope's, Code as a string whichever way the
// host sent it; for a finish_reason Code is the value as the server sent it.
// DeepSeek does not reliably answer failures with JSON, so Message is the
// trimmed body, cut to 512 characters, when the envelope gave none. DeepSeek
// documents no request id and no Retry-After; RetryAfter is read when a host
// sends one. The event's Code follows the status, except that a 400, 413 or
// 422 reporting the context limit is api.CodeContextLength.
type APIError = lib.APIError

// apiError builds the failure for a non 200. The body is parsed on a best
// effort basis into OpenAI's error envelope; whatever comes out is a hint, not
// the classification.
func apiError(status int, header http.Header, body []byte) *APIError {
	e := &APIError{Provider: providerName, Status: status, RetryAfter: lib.RetryAfter(header)}
	var envelope struct {
		Error struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		} `json:"error"`
	}
	if json.Unmarshal(body, &envelope) == nil {
		e.Message = envelope.Error.Message
		e.Type = envelope.Error.Type
		e.Code = codeString(envelope.Error.Code)
	}
	if e.Message == "" {
		e.Message = lib.ErrorMessage(body)
	}
	return e
}

func contextOverflow(e *APIError, body []byte) bool {
	switch e.Status {
	case http.StatusBadRequest, http.StatusRequestEntityTooLarge, http.StatusUnprocessableEntity:
	default:
		return false
	}
	if e.Code == "context_length_exceeded" || e.Type == "context_length_exceeded" {
		return true
	}
	if strings.Contains(strings.ToLower(e.Message), "maximum context length") {
		return true
	}
	return strings.Contains(strings.ToLower(string(body)), "maximum context length")
}

// codeString flattens the code field, which OpenAI compatible hosts send as
// either a string or a number.
func codeString(v any) string {
	switch v := v.(type) {
	case string:
		return v
	case float64:
		return strconv.FormatFloat(v, 'f', -1, 64)
	default:
		return ""
	}
}

// statusCode maps an HTTP status onto the small set worth branching on.
//
// 402 is insufficient balance. It is an account state rather than a bad
// request, and no edit to the body fixes it, which is what puts it with the
// credentials rather than with the request.
//
// 503 is the one 5xx DeepSeek names, documenting it as a server overloaded by
// traffic, so it asks for a backoff and becomes api.CodeBusy. Every other
// 5xx becomes api.CodeNetwork, which asks for the identical request to be
// sent again and is what a server that failed without saying why deserves;
// nothing here can tell a 500 that was momentary from one that will repeat, and
// api.CodeUnknown would hand a caller a decision it has no more
// information to make than this does.
//
// 408 is api.CodeNetwork even though DeepSeek does not document it. It
// comes from whatever sits in front of the API and says the request timed out
// before it was answered, which is the plainest case of sending the identical
// request again; 504 says the same thing and already lands there with the rest
// of the 5xx.
//
// 413 is api.CodeTooLarge rather than a bad request. Where the body was
// measured against Model.RequestBytes, the error names the bytes sent and the
// declared limit.
//
// A status outside all of these is api.CodeUnknown rather than a guess at
// the nearest 4xx. An unlisted status comes from a host this adapter does not
// know rather than from DeepSeek, and calling it a bad request would tell a
// caller to go and fix a body that may be perfect.
//
// A 400, 413 or 422 that reports the context limit is api.CodeContextLength
// instead: see contextOverflow, which the caller of this function consults
// first.
func statusCode(status int) api.ErrCode {
	switch status {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return api.CodeAccess
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return api.CodeBusy
	case http.StatusRequestTimeout:
		return api.CodeNetwork
	case http.StatusRequestEntityTooLarge:
		return api.CodeTooLarge
	case http.StatusBadRequest, http.StatusNotFound, http.StatusUnprocessableEntity:
		return api.CodeInvalid
	}
	if status >= 500 {
		return api.CodeNetwork
	}
	return api.CodeUnknown
}

// inbandError recognizes DeepSeek's error frame, an error object on the last
// chunk of a generation that failed after the 200, and the other shape that
// circulates for OpenAI compatible hosts. A frame carrying one must not be
// folded into content and must not end the turn as a completion.
func inbandError(c *chunk) (api.Event, bool) {
	e := &APIError{Provider: providerName}
	switch {
	case len(c.Error) > 0 && string(c.Error) != "null":
		var envelope struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		}
		if json.Unmarshal(c.Error, &envelope) == nil {
			e.Message = envelope.Message
			e.Type = envelope.Type
			e.Code = codeString(envelope.Code)
		}
		if e.Message == "" {
			e.Message = lib.ErrorMessage(c.Error)
		}
	case c.ErrorType != "" || c.ErrorMsg != "":
		e.Type, e.Message = c.ErrorType, c.ErrorMsg
	default:
		return nil, false
	}
	code := streamErrorCode(e.Code)
	if e.Code == "" {
		code = streamErrorCode(e.Type)
	}
	return api.EventErr{Code: code, Err: e}, true
}

// streamErrorCode classifies an in band failure by the code string.
//
// context_length_exceeded sits apart from the request errors beside it even
// though every one of them is a request the server would not run, because the
// fixes are opposite: compacting the conversation is the only thing that helps
// and it is the one condition here a caller can act on without a person.
//
// api_error is DeepSeek's own: the model failed to finish the reply, and the
// same request may be sent again. An unrecognized code is api.CodeUnknown and
// never a guess at transport: a code nobody here knows says nothing about
// whether the stream was cut, the request was wrong or the account is empty.
func streamErrorCode(code string) api.ErrCode {
	switch code {
	case "context_length_exceeded":
		return api.CodeContextLength
	case "invalid_request_error", "model_not_found", "invalid_parameter":
		return api.CodeInvalid
	case "invalid_api_key", "authentication_error":
		return api.CodeAccess
	case "rate_limit_exceeded", "rate_limit_error", "overloaded_error", "insufficient_system_resource":
		return api.CodeBusy
	case "server_error", "api_error":
		return api.CodeNetwork
	default:
		return api.CodeUnknown
	}
}
