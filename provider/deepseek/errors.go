package deepseek

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"neui.dev/api"
)

// errNoKey is a configuration failure rather than a rejected credential, but it
// lands in the same bucket: there is nothing to retry and the fix is a key.
var errNoKey = errors.New("deepseek: no API key, set DEEPSEEK_API_KEY or pass WithAPIKey")

var errTooLarge = errors.New("deepseek: request too large")

// RequestError is a turn this adapter refused to send. Content is a bare string
// on every role of this wire and a tool result message is a string and nothing
// else, so images, documents and several block placements have no encoding at
// all here. Dropping them and sending the rest would change what the model is
// told, so the turn fails here instead of producing an answer to a different
// question.
type RequestError struct {
	Reason string
}

func (e *RequestError) Error() string {
	return "deepseek: " + e.Reason
}

func reqErrf(format string, args ...any) *RequestError {
	return &RequestError{Reason: fmt.Sprintf(format, args...)}
}

// FinishError is a finish_reason that ended the turn without a reply worth
// keeping. Reason is the wire value verbatim, which for a value this adapter
// does not recognize is the only thing that identifies what the server said,
// and for a recognized one is still what a log needs to tell content_filter
// from length after the code has flattened both into a branch.
type FinishError struct {
	Reason string
}

func (e *FinishError) Error() string {
	return "deepseek: finish_reason " + e.Reason
}

// APIError is a non 200 response. Body is kept verbatim because DeepSeek does
// not reliably answer with JSON on failure: octet-stream, plain text and empty
// bodies are all reported, so the status is the only field worth branching on
// and the rest is for logs.
//
// RetryAfter is zero unless the response carried a parseable Retry-After.
// DeepSeek documents no such header and one measurement found none, so nothing
// here depends on it being present.
type APIError struct {
	Status     int
	Type       string
	Code       string
	Message    string
	Body       string
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	msg := e.Message
	if msg == "" {
		msg = strings.TrimSpace(e.Body)
	}
	if msg == "" {
		msg = http.StatusText(e.Status)
	}
	if e.Code != "" {
		return fmt.Sprintf("deepseek: http %d (%s): %s", e.Status, e.Code, msg)
	}
	return fmt.Sprintf("deepseek: http %d: %s", e.Status, msg)
}

// apiError builds the failure for a non 200. The body is parsed on a best
// effort basis into OpenAI's error envelope; whatever comes out is a hint, not
// the classification.
func apiError(status int, header http.Header, body []byte) *APIError {
	e := &APIError{Status: status, Body: string(body), RetryAfter: retryAfter(header)}
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
	return e
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
// refusal goes back as the server's own, under CodeTooLarge, which is still what
// the refusal was about.
func measuredTooLarge(model string, sent int, declared int, err error) error {
	if declared <= 0 {
		return err
	}
	return fmt.Errorf("%w: sent %d bytes, measured against the %d declared for %s: %w", api.ErrMeasuredTooLarge, sent, declared, model, err)
}

// contextOverflow recognizes the one 400 that is a conversation too long for the
// model rather than a request written wrongly. There is nothing
// machine-readable to read it by: the envelope carries invalid_request_error,
// which every other rejected body carries too, so the message is the only
// discriminator the server offers and leaving it unread is what kept
// CodeContextLength unreachable on this path. The cost of that was measured on
// real traffic — a request of 1,210,970 tokens came back a 400, classified as a
// request to rewrite, and the compaction that was the whole fix never ran.
//
// DeepSeek words it the way OpenAI does: "This model's maximum context length
// is 1048576 tokens. However, you requested 1210970 tokens (1210970 in the
// messages, 0 in the completion). Please reduce the length of the messages or
// completion." Both halves of that are required here, because either on its own
// is a phrase an unrelated refusal could carry, while together they are a
// sentence about this condition and nothing else: no refusal about a malformed
// body, an unknown model, a bad tool schema or an image this model cannot read
// says what the limit is and what was asked for, since those name the field they
// refused. A false positive costs one summarization and then fails identically,
// which is why the match is this narrow rather than a search for the word
// context.
//
// The body is not reliably JSON on failure, so the raw body is matched when no
// message parsed out of it.
func contextOverflow(e *APIError) bool {
	if e.Status != http.StatusBadRequest {
		return false
	}
	msg := e.Message
	if msg == "" {
		msg = e.Body
	}
	return strings.Contains(msg, "maximum context length") && strings.Contains(msg, "you requested")
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
// 413 is api.CodeTooLarge rather than a bad request. The body was measured
// against Model.RequestBytes and sent only because it fit, so a refusal by size
// says that declared limit over-promises for this endpoint; the error carries
// api.ErrMeasuredTooLarge and both numbers, and a caller reads it as terminal
// rather than sending less and trying again.
//
// A status outside all of these is api.CodeUnknown rather than a guess at
// the nearest 4xx. An unlisted status comes from a host this adapter does not
// know rather than from DeepSeek, and calling it a bad request would tell a
// caller to go and fix a body that may be perfect.
//
// 400 is api.CodeInvalid here, and the one 400 that is not is told apart by
// its message: see contextOverflow, which the caller of this function consults.
// Nothing else in a refused body separates a conversation that does not fit from
// a request that is simply wrong.
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

// retryAfter reads RFC 9110's two spellings. A value read as seconds when it
// was a date would produce a sleep measured in decades, so both are tried and
// an unparseable header is treated as absent.
func retryAfter(header http.Header) time.Duration {
	v := strings.TrimSpace(header.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if n, err := strconv.Atoi(v); err == nil {
		if n < 0 {
			return 0
		}
		return time.Duration(n) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := time.Until(t); d > 0 {
			return d
		}
	}
	return 0
}

// StreamError is a failure reported inside an already successful response.
// DeepSeek documents no such frame, so this only ever comes from a host in
// front of it, and the code it carries is whatever that host used.
type StreamError struct {
	Code    string
	Message string
	Raw     string
}

func (e *StreamError) Error() string {
	if e.Code != "" {
		return fmt.Sprintf("deepseek: stream error (%s): %s", e.Code, e.Message)
	}
	return "deepseek: stream error: " + e.Message
}

// inbandError recognizes the two error frame shapes that circulate for OpenAI
// compatible hosts. Neither is confirmed for api.deepseek.com; the point is
// that a frame carrying one must not be folded into content and must not end
// the turn as a completion.
func inbandError(c *chunk) (api.Event, bool) {
	var e *StreamError
	switch {
	case len(c.Error) > 0 && string(c.Error) != "null":
		var envelope struct {
			Message string `json:"message"`
			Type    string `json:"type"`
			Code    any    `json:"code"`
		}
		e = &StreamError{Raw: string(c.Error)}
		if json.Unmarshal(c.Error, &envelope) == nil {
			e.Message = envelope.Message
			e.Code = codeString(envelope.Code)
			if e.Code == "" {
				e.Code = envelope.Type
			}
		}
		if e.Message == "" {
			e.Message = string(c.Error)
		}
	case c.ErrorType != "" || c.ErrorMsg != "":
		e = &StreamError{Code: c.ErrorType, Message: c.ErrorMsg, Raw: c.ErrorMsg}
	default:
		return nil, false
	}
	return api.EventErr{Code: streamErrorCode(e.Code), Err: e}, true
}

// streamErrorCode classifies an in band failure by the code string.
//
// context_length_exceeded sits apart from the request errors beside it even
// though every one of them is a request the server would not run, because the
// fixes are opposite: compacting the conversation is the only thing that helps
// and it is the one condition here a caller can act on without a person.
//
// An unrecognized code is api.CodeUnknown and never a guess at transport.
// This frame comes from a host in front of DeepSeek rather than from DeepSeek,
// so the vocabulary is open, and a code nobody here knows says nothing about
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
	case "server_error":
		return api.CodeNetwork
	default:
		return api.CodeUnknown
	}
}
