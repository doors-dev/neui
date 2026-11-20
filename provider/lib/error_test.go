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
	"errors"
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestAPIErrorFormat(t *testing.T) {
	for want, e := range map[string]APIError{
		"claude: http 429 rate_limit_error enforced_spend_limit_reached: spend (request req_1)": {Provider: "claude", Status: 429, Type: "rate_limit_error", Code: "enforced_spend_limit_reached", Message: "spend", RequestID: "req_1", RetryAfter: time.Second},
		"gpt: http 400 invalid_request_error: bad (param input[0]) (request req_2)":             {Provider: "gpt", Status: 400, Type: "invalid_request_error", Message: "bad", Param: "input[0]", RequestID: "req_2", NoRetry: true},
		"deepseek: length: the turn ended on this finish_reason":                                {Provider: "deepseek", Code: "length", Message: "the turn ended on this finish_reason"},
		"gpt: the response was cancelled":                                                       {Provider: "gpt", Message: "the response was cancelled"},
		"deepseek: http 502":                                                                    {Provider: "deepseek", Status: 502},
		"claude: no message":                                                                    {Provider: "claude"},
	} {
		if got := e.Error(); got != want {
			t.Errorf("Error() = %q, want %q", got, want)
		}
	}
}

func TestErrorMessageIsTrimmedAndCut(t *testing.T) {
	if got := ErrorMessage([]byte("  <html>down</html>\n")); got != "<html>down</html>" {
		t.Errorf("message %q, want the trimmed body", got)
	}
	got := ErrorMessage([]byte(strings.Repeat("é", 2000)))
	if n := len([]rune(got)); n != 513 || !strings.HasSuffix(got, "…") {
		t.Errorf("message has %d runes, want 512 and the mark", n)
	}
}

func TestRetryAfter(t *testing.T) {
	for header, want := range map[string]bool{
		"":         false,
		"7":        true,
		" 7 ":      true,
		"-1":       false,
		"nonsense": false,
		time.Now().Add(time.Minute).UTC().Format(http.TimeFormat):  true,
		time.Now().Add(-time.Minute).UTC().Format(http.TimeFormat): false,
	} {
		h := http.Header{}
		if header != "" {
			h.Set("Retry-After", header)
		}
		if got := RetryAfter(h) > 0; got != want {
			t.Errorf("Retry-After %q: got %v, want %v", header, got, want)
		}
	}
	if got := RetryAfter(http.Header{"Retry-After": {"7"}}); got != 7*time.Second {
		t.Errorf("Retry-After 7 read as %v", got)
	}
	if got := RetryAfter(nil); got != 0 {
		t.Errorf("no headers read as %v", got)
	}
}

func TestMeasuredTooLargeNamesTheSizes(t *testing.T) {
	refusal := &APIError{Provider: "gpt", Status: 413}
	if got := MeasuredTooLarge("m", 10, 0, refusal); got != error(refusal) {
		t.Errorf("an unmeasured request came back as %v", got)
	}
	got := MeasuredTooLarge("m", 10, 20, refusal)
	var carried *APIError
	if !errors.As(got, &carried) || carried != refusal {
		t.Errorf("a measured request came back as %v", got)
	}
	if !strings.Contains(got.Error(), "sent 10 bytes") || !strings.Contains(got.Error(), "20 declared for m") {
		t.Errorf("error %q names neither size", got)
	}
}

func TestImageMime(t *testing.T) {
	for in, want := range map[string]string{
		"image/png":    "image/png",
		" IMAGE/Jpeg ": "image/jpeg",
		"image/jpg":    "image/jpg",
		"image/webp":   "image/webp",
		"image/gif":    "image/gif",
	} {
		if got, ok := ImageMime(in); !ok || got != want {
			t.Errorf("ImageMime(%q) = %q, %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "image/bmp", "application/pdf"} {
		if _, ok := ImageMime(in); ok {
			t.Errorf("ImageMime(%q) was accepted", in)
		}
	}
}
