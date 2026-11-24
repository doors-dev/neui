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

package feature

import "testing"

func TestSetKeepsPartsAndModesApart(t *testing.T) {
	s := NewSet([]Feature{Tokens, Dock, nil})
	for _, f := range []Feature{Tokens, Dock} {
		if !s.Has(f) {
			t.Errorf("the set lacks %T %v", f, f)
		}
	}
	for _, f := range []Feature{Thinking, Greeting, Float} {
		if s.Has(f) {
			t.Errorf("the set has %T %v, which was not turned off", f, f)
		}
	}
}
