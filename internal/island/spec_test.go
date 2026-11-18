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

func TestSpecIsGuideThenReference(t *testing.T) {
	i, _ := newTestIsland(t)
	ctrl := testkit.NewController(t, testkit.Setup{Modules: []common.Module{timemod.Module(context.Background())}})
	spec, ends, items := i.Spec(ctrl.Ctx(), "board")
	want := []string{"what to build", "how an app runs", "the screen", "views and controls", "app functions and your tools", "logs and files", "the typical loop", "reading the reference"}
	if !slices.Equal(items, want) || len(ends) != len(want) || ends[len(ends)-1] != len(spec) {
		t.Fatalf("items %v ending at %v of %d bytes, want %v:\n%s", items, ends, len(spec), want, spec)
	}
	texts := []string{
		specWhatToBuild,
		specHowAnAppRuns,
		specScreen,
		specViewsAndControls,
		fmt.Sprintf(specAppFunctionsAndTools, "time__*"),
		fmt.Sprintf(specLogsAndFiles, logFile("board"), islandDir("board")),
		specTypicalLoop,
		specReference,
	}
	titles := []string{"What to build", "How an app runs", "The screen", "Views and controls", "App functions and your tools", "Logs and files", "The typical loop", "Reading the reference"}
	prev := 0
	for n, end := range ends {
		item := spec[prev:end]
		head := "\n" + titles[n] + "\n"
		if n == 0 {
			head = titles[n] + "\n"
		}
		if item != texts[n] || !strings.HasPrefix(item, head) || !strings.HasSuffix(item, "\n") || strings.HasSuffix(item, "\n\n") {
			t.Fatalf("the %s item reads:\n%s", items[n], item)
		}
		prev = end
	}
	if !strings.Contains(spec, "the script has time__*.") || strings.Contains(spec, "Your tools") {
		t.Fatalf("the spec does not name the script's tool modules in its app functions and tools section:\n%s", spec)
	}
	if strings.Contains(spec, "Limits") {
		t.Fatalf("the spec still carries a limits line:\n%s", spec)
	}
}
