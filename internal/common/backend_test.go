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

package common

import (
	"context"
	"testing"
)

func TestBackendSimpleSchemas(t *testing.T) {
	app := context.WithValue(context.Background(), SimpleSchemasKey{}, true)
	for _, c := range []struct {
		name string
		ctx  context.Context
		opts []Option
		want bool
	}{
		{"default", context.Background(), nil, false},
		{"app", app, nil, true},
		{"session off over the app", app, []Option{OptionSimpleSchemas(false)}, false},
		{"session on", context.Background(), []Option{OptionSimpleSchemas(true)}, true},
	} {
		if got := NewBackend(c.ctx, nil, nil, nil, c.opts).SimpleSchemas(); got != c.want {
			t.Errorf("%s: simple schemas %v, want %v", c.name, got, c.want)
		}
	}
}
