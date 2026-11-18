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

package island

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"

	"neui.dev/internal/common"
	timemod "neui.dev/internal/modules/time"
	"neui.dev/internal/testkit"
)

func TestSpecIntroIsLogsToolsThenReference(t *testing.T) {
	i, _ := newTestIsland(t)
	ctrl := testkit.NewController(t, testkit.Setup{Modules: []common.Module{timemod.Module(context.Background())}})
	spec, ends, items := i.Spec(ctrl.Ctx(), "board")
	if !slices.Equal(items, []string{"logs and files", "your tools", "reading the reference"}) || len(ends) != 3 || ends[2] != len(spec) {
		t.Fatalf("items %v ending at %v of %d bytes, want logs and files, your tools, then reading the reference:\n%s", items, ends, len(spec), spec)
	}
	logs := spec[:ends[0]]
	if !strings.HasPrefix(logs, "Logs and files\n") || !strings.Contains(logs, " "+logFile("board")+", ") || !strings.Contains(logs, " "+islandDir("board")+", ") || !strings.HasSuffix(logs, "\n") {
		t.Fatalf("the logs and files item reads:\n%s", logs)
	}
	if spec[ends[0]:ends[1]] != fmt.Sprintf(specTools, "time__*") || !strings.HasPrefix(specTools, "\nYour tools\n") {
		t.Fatalf("the your tools item reads:\n%s", spec[ends[0]:ends[1]])
	}
	if spec[ends[1]:] != specReference || !strings.HasPrefix(specReference, "\nReading the reference\n") {
		t.Fatalf("the reading the reference item reads:\n%s", spec[ends[1]:])
	}
	if strings.Contains(spec, "Limits") {
		t.Fatalf("the intro still carries a limits line:\n%s", spec)
	}
}
