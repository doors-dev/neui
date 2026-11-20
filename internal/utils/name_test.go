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

package utils

import (
	"regexp"
	"strings"
	"testing"
)

func TestSanitizeName(t *testing.T) {
	for in, want := range map[string]string{
		"read_file":             "read_file",
		"  Read--File!! ":       "Read_File",
		"a__b":                  "a_b",
		strings.Repeat("a", 31): strings.Repeat("a", 31),
		"0day":                  "x0day",
		"9 lives":               "x9_lives",
	} {
		if got := SanitizeName(in); got != want {
			t.Errorf("SanitizeName(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSanitizeNameCapsAndNeverEmpties(t *testing.T) {
	shape := regexp.MustCompile(`^[A-Za-z0-9]+(_[A-Za-z0-9]+)*$`)
	long := regexp.MustCompile(`^.{1,24}_[0-9a-f]{6}$`)
	empty := regexp.MustCompile(`^x[0-9a-f]{6}$`)
	seen := map[string]string{}
	for _, in := range []string{
		strings.Repeat("a", 32),
		strings.Repeat("a", 32) + "b",
		strings.Repeat("a", 23) + "_" + strings.Repeat("b", 20),
		strings.Repeat("a", 23) + "_" + strings.Repeat("c", 20),
		strings.Repeat("ab_", 40),
	} {
		got := SanitizeName(in)
		if len(got) > 31 || !shape.MatchString(got) || !long.MatchString(got) {
			t.Errorf("SanitizeName(%q) = %q, want at most 31 characters ending in a hash", in, got)
		}
		if strings.Contains(got, "__") {
			t.Errorf("SanitizeName(%q) = %q, which holds a double underscore", in, got)
		}
		if again := SanitizeName(got); again != got {
			t.Errorf("SanitizeName(%q) = %q, and sanitizing that gives %q", in, got, again)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("%q and %q both sanitize to %q", prev, in, got)
		}
		seen[got] = in
	}
	for _, in := range []string{"", "!!!", "___", "日本語"} {
		got := SanitizeName(in)
		if !empty.MatchString(got) {
			t.Errorf("SanitizeName(%q) = %q, want a letter and six hex characters", in, got)
		}
		if again := SanitizeName(got); again != got {
			t.Errorf("SanitizeName(%q) = %q, and sanitizing that gives %q", in, got, again)
		}
		if prev, dup := seen[got]; dup {
			t.Errorf("%q and %q both sanitize to %q", prev, in, got)
		}
		seen[got] = in
	}
}
