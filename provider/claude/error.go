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
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"neui.dev/api"
)

// APIError is a failure Anthropic reported about the request itself, as opposed
// to one the connection produced. It covers both places the API puts them: the
// body of a non 200 response, and an error frame on a stream that already
// returned 200. Status is zero for the second, which carries neither a status
// code nor a request id.
//
// It arrives as the Err of an api.EventErr; read it with errors.As. Branch on
// the event's Code, and use these fields for logs and support requests.
type APIError struct {
	// Status is the HTTP status, zero for an in-stream error.
	Status int
	// Type is the API's error type, such as "overloaded_error" or
	// "invalid_request_error".
	Type string
	// Message is the API's explanation; for a body that was not the
	// documented JSON it is the trimmed raw body, cut to 512 characters. A
	// response that leaves it empty gets "unparseable <status> response". It
	// can be empty for an in-stream error.
	Message string
	// ErrorCode is the error's details.error_code, such as
	// "enforced_spend_limit_reached"; empty when the error carries none.
	ErrorCode string
	// RequestID is Anthropic's request id, to quote to their support; empty
	// for an in-stream error.
	RequestID string
	// RetryAfter is the Retry-After header's wait, zero when the header is
	// absent or not a whole number of seconds.
	RetryAfter time.Duration
}

// Error returns "claude: <status> <type>: <message> (request <id>)", leaving
// out an empty status, type or request id and writing "no message" for an
// empty message.
func (e *APIError) Error() string {
	var b []byte
	b = append(b, "claude: "...)
	if e.Status != 0 {
		b = strconv.AppendInt(b, int64(e.Status), 10)
		b = append(b, ' ')
	}
	if e.Type != "" {
		b = append(b, e.Type...)
		b = append(b, ": "...)
	}
	if e.Message != "" {
		b = append(b, e.Message...)
	} else {
		b = append(b, "no message"...)
	}
	if e.RequestID != "" {
		b = append(b, " (request "...)
		b = append(b, e.RequestID...)
		b = append(b, ')')
	}
	return string(b)
}

// statusCode maps a non 200 response. The status is the discriminator, not the
// type string: the docs say 400 doubles as the catch all for unlisted 4xx and
// that the type values grow over time, so a body that fails to parse still
// classifies correctly.
//
// 413 is CodeTooLarge rather than CodeContextLength or CodeInvalid. It is the
// edge refusing the request by its byte size before the API has counted a token,
// so compacting the conversation is not what fixes it. It is not an ordinary bad
// request either: where this adapter measured the body against
// Model.RequestBytes and sent it only because it fit, a refusal means that
// declared limit over-promises for this endpoint. The error carries
// api.ErrMeasuredTooLarge and both numbers to say so, and a caller reads it as
// terminal rather than retrying; a model that declares no limit was never
// measured, so that refusal goes back unmarked and the code is the same. A
// conversation that genuinely does not fit comes back as a 400 instead, and
// contextOverflow is what tells it from the other 4xx.
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
		return e.ErrorCode == "enforced_spend_limit_reached"
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

// measuredTooLarge marks a refusal by size as one no retry can fix. The marker
// states a precondition: the adapter had already measured this body against
// Model.RequestBytes and sent it because it fit, so the declared limit
// over-promises for this endpoint and a smaller request is not what was wrong.
// A caller reading it ends the turn rather than shedding and retrying.
//
// A model that declares no limit was never measured — request skips the check at
// zero — so there is nothing the marker could be true about, and claiming it
// would spend the one remedy that does work on an error that forbids it. Such a
// refusal goes back as the API's own, under CodeTooLarge, which is still what
// the refusal was about.
func measuredTooLarge(model string, sent int, declared int, err error) error {
	if declared <= 0 {
		return err
	}
	return fmt.Errorf("%w: sent %d bytes, measured against the %d declared for %s: %w", api.ErrMeasuredTooLarge, sent, declared, model, err)
}

// retryAfter reads the header, which is a whole number of seconds and not a
// timestamp. The anthropic-ratelimit-*-reset headers next to it are RFC 3339
// instants; confusing the two produces a retry decades out.
func retryAfter(h http.Header) time.Duration {
	v := h.Get("Retry-After")
	if v == "" {
		return 0
	}
	secs, err := strconv.Atoi(v)
	if err != nil || secs < 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func newAPIError(status int, h http.Header, env *errorEnvelope) *APIError {
	e := &APIError{Status: status, RetryAfter: retryAfter(h)}
	if h != nil {
		e.RequestID = h.Get("request-id")
	}
	if env != nil {
		if env.Error != nil {
			e.Type = env.Error.Type
			e.Message = env.Error.Message
			if env.Error.Details != nil {
				e.ErrorCode = env.Error.Details.ErrorCode
			}
		}
		if env.RequestID != "" {
			e.RequestID = env.RequestID
		}
	}
	if e.Message == "" {
		e.Message = fmt.Sprintf("unparseable %d response", status)
	}
	return e
}
