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

package layout

import "testing"

func TestModeFollowsTheAllowedModes(t *testing.T) {
	for _, tc := range []struct {
		name  string
		state State
		float bool
		dock  bool
		want  string
	}{
		{"collapsed stays collapsed", State{Dock: true}, true, true, "collapsed"},
		{"both allowed, floating", State{Expanded: true}, true, true, "float"},
		{"both allowed, docked", State{Expanded: true, Dock: true}, true, true, "dock"},
		{"dock off keeps a docked state floating", State{Expanded: true, Dock: true}, true, false, "float"},
		{"float off docks a floating state", State{Expanded: true}, false, true, "dock"},
		{"both off keeps float", State{Expanded: true, Dock: true}, false, false, "float"},
	} {
		if got := Mode(tc.state, tc.float, tc.dock); got != tc.want {
			t.Errorf("%s: Mode(%+v, float %v, dock %v) = %q, want %q", tc.name, tc.state, tc.float, tc.dock, got, tc.want)
		}
	}
}
