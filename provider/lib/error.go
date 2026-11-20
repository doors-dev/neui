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

package lib

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// APIError is a failure the API itself reported, as opposed to one the
// connection produced: a non 200 response, an error the API put inside a
// stream that had already returned 200, or a turn the API ended without a
// reply worth keeping, such as one cut at the token cap or refused. Each
// provider package names it as its own APIError.
//
// It arrives as the Err of an api.EventErr, wrapped after a 413 on a request
// measured against Model.RequestBytes; read it with errors.As. Branch on the
// event's Code, and use these fields for logs and support requests. Which
// fields an API fills differs, and an empty one means it gave none.
type APIError struct {
	// Provider is the name of the provider that built the error, such as
	// "claude".
	Provider string
	// Status is the HTTP status, zero for a failure that arrived inside the
	// stream.
	Status int
	// Type is the API's error type, such as "invalid_request_error".
	Type string
	// Code is the API's error code, such as "context_length_exceeded". For a
	// turn the API ended without a reply it is the stop reason exactly as the
	// API sent it.
	Code string
	// Message is the API's explanation. For a response body that was not the
	// documented JSON it is the trimmed body, cut to 512 characters, and for
	// a stop reason it is the adapter's own description.
	Message string
	// Param is the request parameter the error is about, where the API names
	// one.
	Param string
	// RequestID is the API's id for the request, to quote to its support.
	RequestID string
	// RetryAfter is the Retry-After header's wait, zero when the header is
	// absent or unreadable.
	RetryAfter time.Duration
	// NoRetry says the response advised against retrying, which only OpenAI
	// does, with x-should-retry: false.
	NoRetry bool
}

// Error returns "<provider>: http <status> <type> <code>: <message> (param
// <param>) (request <id>)", leaving out every part that is empty or zero, and
// writing "no message" when status, type, code and message all are.
func (e *APIError) Error() string {
	var b []byte
	b = append(b, e.Provider...)
	b = append(b, ':')
	if e.Status != 0 {
		b = append(b, " http "...)
		b = strconv.AppendInt(b, int64(e.Status), 10)
	}
	if e.Type != "" {
		b = append(b, ' ')
		b = append(b, e.Type...)
	}
	if e.Code != "" {
		b = append(b, ' ')
		b = append(b, e.Code...)
	}
	described := e.Status != 0 || e.Type != "" || e.Code != ""
	switch {
	case e.Message != "" && described:
		b = append(b, ": "...)
		b = append(b, e.Message...)
	case e.Message != "":
		b = append(b, ' ')
		b = append(b, e.Message...)
	case !described:
		b = append(b, " no message"...)
	}
	if e.Param != "" {
		b = append(b, " (param "...)
		b = append(b, e.Param...)
		b = append(b, ')')
	}
	if e.RequestID != "" {
		b = append(b, " (request "...)
		b = append(b, e.RequestID...)
		b = append(b, ')')
	}
	return string(b)
}

const maxErrorMessage = 512

// ErrorMessage returns an error response body as the Message of an APIError,
// for a body that was not the JSON the API documents: trimmed of surrounding
// space and cut to 512 characters, with "…" marking the cut. At a gateway the
// body is often a whole HTML page.
func ErrorMessage(body []byte) string {
	text := strings.TrimSpace(string(body))
	if r := []rune(text); len(r) > maxErrorMessage {
		text = string(r[:maxErrorMessage]) + "…"
	}
	return text
}

// RetryAfter reads the Retry-After header in either form RFC 9110 gives it,
// a whole number of seconds or an HTTP date. It returns zero for a header
// that is absent, unreadable, negative or in the past.
func RetryAfter(h http.Header) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
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

// MeasuredTooLarge wraps err, the API's refusal of a request by its size,
// with the bytes sent and the request limit the model declares. With declared
// at zero or less the request was never measured and err is returned as it
// is.
func MeasuredTooLarge(model string, sent int, declared int, err error) error {
	if declared <= 0 {
		return err
	}
	return fmt.Errorf("sent %d bytes, measured against the %d declared for %s: %w", sent, declared, model, err)
}
