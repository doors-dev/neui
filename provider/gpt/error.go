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
	"net/http"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// APIError describes a failed turn: an HTTP status before the stream
// started, an error or failed frame inside it, or a response the service
// ended as incomplete, filtered, refused or cancelled. Status is zero for a
// failure that arrived inside the stream.
//
// Type, Code and Param are the service's own, and Code is the
// incomplete_details reason for a response that ended incomplete. Message is
// this adapter's description when the service gave none. RequestID is the
// x-request-id header, which OpenAI support asks for, RetryAfter the
// Retry-After header's wait, and NoRetry says the response carried
// x-should-retry: false; all three are set for HTTP failures only.
type APIError = lib.APIError

// httpError turns a refused request into the one EventErr that ends the turn.
// The body is parsed tolerantly: a decode failure must not become the reported
// failure, because the status is the part that classifies.
//
// An unparseable body is cut by lib.ErrorMessage.
func httpError(h *lib.HTTPError, model string, sent int, declared int) api.EventErr {
	e := &APIError{
		Provider:   providerName,
		Status:     h.Status,
		NoRetry:    strings.EqualFold(h.Header.Get("x-should-retry"), "false"),
		RequestID:  h.Header.Get("x-request-id"),
		RetryAfter: lib.RetryAfter(h.Header),
	}
	raw := h.Body
	var wrapper struct {
		Error *wireError `json:"error"`
	}
	if err := json.Unmarshal(raw, &wrapper); err == nil && wrapper.Error != nil {
		e.Type = wrapper.Error.Type
		e.Code = wrapper.Error.Code
		e.Param = wrapper.Error.Param
		e.Message = wrapper.Error.Message
	} else {
		e.Message = lib.ErrorMessage(raw)
	}
	code := statusCode(e)
	if code == api.CodeTooLarge {
		return api.EventErr{Code: code, Err: lib.MeasuredTooLarge(model, sent, declared, e)}
	}
	return api.EventErr{Code: code, Err: e}
}

// statusCode classifies on the status first, then on the code where the status
// alone would hide a condition with a fix of its own. The type field is not a
// classifier: a rejected key answers 401 with type invalid_request_error and
// only code says invalid_api_key. Precedence is status, then code, then type.
//
// 429 is two unrelated failures. A quota, credit balance or spend limit that ran
// out is an account level fact the docs say no retry restores, and retrying it
// is a hot loop against a bill nobody paid, so it goes with the credentials
// rather than with the rate limit. A tier with no spare capacity answers 429 as
// well and stays CodeBusy alongside the rate limit, because backing off and
// sending the same request again is the answer to both; the only thing that
// ever told the two apart was a phrase in the message, which is not something
// to classify on.
//
// A timeout, a 5xx and a dead socket are one code, because the request was sent
// as written and the fix is to send it again. 503 is the exception among the
// 5xx: it names a service with no capacity, which asks for a backoff before that
// resend, so it joins 429 on CodeBusy and matches what the sibling adapters make
// of the same status.
//
// 409 is CodeInvalid rather than CodeBusy. The only documented meaning behind it
// is a conflicting state the request named, which no amount of backing off
// changes, and it is what this service's own conflict spelling maps to elsewhere.
//
// 413 is CodeTooLarge rather than CodeInvalid. Where the body was measured
// against Model.RequestBytes, the error names the bytes sent and the declared
// limit.
//
// 402 is a balance, which is an account fact rather than a bad request, so it
// goes with the credentials.
//
// x-should-retry is a Stainless convention rather than documented API. It is
// retry advice and not a classification, so it is kept on the APIError for a
// retry layer to read and never overwrites a code that was recognized: a caller
// told CodeUnknown about a 429 it already identified has been handed back less
// than the adapter knew.
func statusCode(e *APIError) api.ErrCode {
	switch e.Status {
	case http.StatusBadRequest:
		// A 400 is the status every rejected body shares, and the only one an
		// over-length conversation arrives on before the stream starts. This
		// service says so in error.code, which is a value and not a sentence, so
		// the code is consulted here as well as in the stream: classifying the
		// status alone makes CodeContextLength unreachable on this path and sends
		// the caller off to rewrite a request whose only fault is its length,
		// while the compaction that is the fix never runs. Only that one code is
		// read, so every other 400 lands on CodeInvalid as it did.
		if errorVocabulary(e.Code) == api.CodeContextLength {
			return api.CodeContextLength
		}
		return api.CodeInvalid
	case http.StatusNotFound, http.StatusConflict,
		http.StatusUnprocessableEntity:
		return api.CodeInvalid
	case http.StatusRequestEntityTooLarge:
		return api.CodeTooLarge
	case http.StatusUnauthorized, http.StatusPaymentRequired, http.StatusForbidden:
		return api.CodeAccess
	case http.StatusRequestTimeout:
		return api.CodeNetwork
	case http.StatusTooManyRequests:
		if errorVocabulary(e.Code) == api.CodeAccess || e.Type == "insufficient_quota" {
			return api.CodeAccess
		}
		return api.CodeBusy
	case http.StatusServiceUnavailable:
		return api.CodeBusy
	}
	if e.Status >= 500 {
		return api.CodeNetwork
	}
	return api.CodeUnknown
}

// inBandCode classifies a failure that arrived inside the stream, where there
// is no status to classify on. Precedence is the same as statusCode's: code,
// then type. The type is not decoration here — the nested error object seen on
// the wire is the standard OpenAI error object, whose code is frequently null
// and which then says server_error in its type instead, and a retryable mid
// stream 5xx reported as CodeUnknown is one a backoff layer will not retry.
func inBandCode(code, typ string) api.ErrCode {
	if c := errorVocabulary(code); c != api.CodeUnknown {
		return c
	}
	return errorVocabulary(typ)
}

// errorVocabulary is the service's own spellings, read from both the stream and
// the status path. It is where CodeContextLength is decided from a
// machine-readable value: this API names an over-long conversation in the error
// code, where the sibling adapters have only a sentence written for a person to
// match and pay for it with a matcher that a rewording silently defeats.
//
// CodeContextLength comes from context_length_exceeded alone.
// string_above_max_length is a per-field schema limit, one string past its
// maxLength with the field named in param, and compaction does not shorten it,
// so it is CodeInvalid: the request is refused as written.
//
// A spelling nobody here knows is CodeUnknown rather than the nearest code that
// looks close, so a vocabulary that grew arrives unclassified and the raw string
// stays on the APIError for whoever reads it.
func errorVocabulary(code string) api.ErrCode {
	switch code {
	case "context_length_exceeded":
		return api.CodeContextLength
	case "rate_limit_exceeded", "rate_limit_error", "overloaded_error",
		"slow_down", "server_is_overloaded", "service_unavailable_error":
		return api.CodeBusy
	case "insufficient_quota", "invalid_api_key", "credit_balance_exhausted",
		"organization_spend_limit_exceeded", "project_spend_limit_exceeded",
		"organization_usage_limit_exceeded":
		return api.CodeAccess
	case "server_error", "vector_store_timeout":
		return api.CodeNetwork
	case "invalid_request_error", "invalid_prompt", "bio_policy", "data_residency_mismatch",
		"string_above_max_length":
		return api.CodeInvalid
	case "":
		return api.CodeUnknown
	}
	if strings.HasPrefix(code, "invalid_image") || strings.Contains(code, "image") {
		return api.CodeInvalid
	}
	return api.CodeUnknown
}
