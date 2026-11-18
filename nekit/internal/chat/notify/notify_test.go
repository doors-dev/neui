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

package notify

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

func TestKind(t *testing.T) {
	cases := []struct {
		err  error
		want string
	}{
		{nil, ""},
		{context.Canceled, ""},
		{fmt.Errorf("rename: %w", context.Canceled), ""},
		{context.DeadlineExceeded, "server"},
		{errors.New("disk full"), "server"},
	}
	for _, c := range cases {
		if got := kind(c.err); got != c.want {
			t.Errorf("kind(%v) = %q, want %q", c.err, got, c.want)
		}
	}
}
