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
	"net/http"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// APIError is a failure Anthropic reported about the request itself, as opposed
// to one the connection produced. It covers the three places the API puts
// them: the body of a non 200 response, an error frame on a stream that already
// returned 200, and a stop_reason that ended the turn as a failure. Status is
// zero for the last two, which carry neither a status code nor a request id.
//
// Type is the API's error type, such as "overloaded_error". Code is the
// error's details.error_code, such as "enforced_spend_limit_reached", or the
// stop_reason as the API sent it. RequestID is Anthropic's request id, to
// quote to their support.
type APIError = lib.APIError

// statusCode maps a non 200 response. The status is the discriminator, not the
// type string: the docs say 400 doubles as the catch all for unlisted 4xx and
// that the type values grow over time, so a body that fails to parse still
// classifies correctly.
//
// 413 is CodeTooLarge rather than CodeContextLength or CodeInvalid. It is the
// edge refusing the request by its byte size before the API has counted a token.
// Where this adapter measured the body against Model.RequestBytes, the error
// names the bytes sent and the declared limit. A conversation that genuinely does
// not fit the window comes back as a 400 instead, and contextOverflow is what
// tells it from the other 4xx.
//
// 500 and 502 are CodeNetwork rather than CodeBusy: the documented action for
// them is to retry the request as sent, which is what CodeNetwork says, while
// CodeBusy also says to back off first and is reserved for the statuses that
// mean out of capacity — 429, 503 and this API's own 529.
//
// A conversation over the window is a 400 whose only statement of that is prose,
// so it is not separated here: see contextOverflow, which the caller of this
// function consults before settling for CodeInvalid.
func statusCode(status int) api.ErrCode {
	switch status {
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return api.CodeAccess
	case http.StatusRequestTimeout, http.StatusGatewayTimeout:
		return api.CodeNetwork
	case http.StatusRequestEntityTooLarge:
		return api.CodeTooLarge
	case http.StatusTooManyRequests, http.StatusServiceUnavailable, 529:
		return api.CodeBusy
	case http.StatusInternalServerError, http.StatusBadGateway:
		return api.CodeNetwork
	}
	switch {
	case status >= 400 && status < 500:
		return api.CodeInvalid
	case status >= 500:
		return api.CodeNetwork
	}
	return api.CodeUnknown
}

// contextOverflow recognizes the one invalid_request_error that is a
// conversation too long for the model rather than a request written wrongly.
// This API offers nothing machine-readable for it: the status is the 400 every
// rejected body shares and the type is invalid_request_error, which every one of
// them carries too, so the sentence is the only discriminator there is. Leaving
// it unread is what kept CodeContextLength unreachable on this path, and with it
// the compaction that is the entire fix, while the caller was told instead to go
// and rewrite a request whose only fault was its length.
//
// The phrasing is "prompt is too long: 1210970 tokens > 1048576 maximum", and
// the match is the words that are about this condition and no other. A refusal
// about a media type, a tool schema, a sampling knob or a thinking block that
// was modified names the field it refused; none of them says the prompt is too
// long. A wording change defeats this, which is the price of a wire that states
// the condition in prose, and the failure it degrades to is the CodeInvalid of
// today. A false positive costs one summarization and then fails identically,
// which is why the match is a phrase rather than the word "long" or "tokens".
func contextOverflow(message string) bool {
	return strings.Contains(message, "prompt is too long")
}

func spendLimited(e *APIError) bool {
	switch e.Status {
	case http.StatusTooManyRequests:
		return e.Code == "enforced_spend_limit_reached"
	case http.StatusBadRequest:
		return strings.HasPrefix(e.Message, "You have reached your specified API usage limits") ||
			strings.HasPrefix(e.Message, "You have reached your specified workspace API usage limits")
	}
	return false
}

// inbandCode maps an error frame, which arrives with no status to branch on.
// api_error is the in band spelling of a 500 and takes the same code for the
// same reason: it says the attempt failed, not that the service is full.
// request_too_large is the in band spelling of the 413 and takes CodeTooLarge
// for the reason statusCode gives.
func inbandCode(kind string) api.ErrCode {
	switch kind {
	case "overloaded_error", "rate_limit_error":
		return api.CodeBusy
	case "timeout_error", "api_error":
		return api.CodeNetwork
	case "authentication_error", "permission_error", "billing_error":
		return api.CodeAccess
	case "request_too_large":
		return api.CodeTooLarge
	case "invalid_request_error", "not_found_error", "conflict_error":
		return api.CodeInvalid
	}
	return api.CodeUnknown
}

// readAPIError decodes a refused request, tolerating a body that is not JSON: a
// 413 is returned by the edge before the request reaches the API and need not
// use the documented envelope at all. The whole error is returned rather than
// the interface, because the message is what separates a conversation over the
// window from every other 400. See contextOverflow.
func readAPIError(h *lib.HTTPError) *APIError {
	e := &APIError{Provider: providerName, Status: h.Status, RequestID: h.Header.Get("request-id"), RetryAfter: lib.RetryAfter(h.Header)}
	var env errorEnvelope
	if err := json.Unmarshal(h.Body, &env); err != nil || env.Error == nil {
		e.Message = lib.ErrorMessage(h.Body)
		return e
	}
	e.Type = env.Error.Type
	e.Message = env.Error.Message
	if env.Error.Details != nil {
		e.Code = env.Error.Details.ErrorCode
	}
	if env.RequestID != "" {
		e.RequestID = env.RequestID
	}
	return e
}
