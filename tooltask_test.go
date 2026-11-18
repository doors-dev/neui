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

package neui

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
)

func TestToolTaskRefusedFromScript(t *testing.T) {
	m := NewModule("jobs", "")
	if _, err := m.Tool("start", "", func(ctx context.Context, _ struct{}) (string, error) {
		_, err := ToolTask(ctx, "work", nil, func(error) {})
		return "", err
	}); err != nil {
		t.Fatal(err)
	}
	tool, ok := m.Get("start")
	if !ok {
		t.Fatal("no start tool")
	}
	_, err := tool.JS(context.Background())([]any{json.RawMessage(`{}`)})
	if err == nil || !strings.Contains(err.Error(), "a background task cannot be started from a script") {
		t.Fatalf("ToolTask from a script: %v", err)
	}
}
