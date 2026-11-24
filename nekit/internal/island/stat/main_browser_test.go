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

//go:build browser

package stat_test

import (
	"testing"
	"time"

	"neui.dev/internal/testkit/stack"
	"neui.dev/nekit/internal/island/stat"
)

func TestMain(m *testing.M) {
	*stat.RefreshFloor = time.Second
	*stat.BackoffCeiling = 2 * time.Second
	stack.Main(m)
}
