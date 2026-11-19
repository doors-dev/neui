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

func TestGuideIsItsSectionsInOrder(t *testing.T) {
	guide := Guide()
	texts := []string{guideWhatToBuild, guideHowAnAppRuns, guideScreen, guideViewsAndControls, guideAppFunctionsAndTools, guideTypicalLoop, guideReference}
	titles := []string{"What to build", "How an app runs", "The screen", "Views and controls", "App functions and your tools", "The typical loop", "Reading the reference"}
	if guide != strings.Join(texts, "") {
		t.Fatalf("the guide is not its sections in order:\n%s", guide)
	}
	for n, text := range texts {
		head := "\n" + titles[n] + "\n"
		if n == 0 {
			head = titles[n] + "\n"
		}
		if !strings.HasPrefix(text, head) || !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
			t.Fatalf("the %s section reads:\n%s", titles[n], text)
		}
	}
	for _, text := range []string{"below", "%", "Logs and files", "the script has", "Your tools", "Limits"} {
		if strings.Contains(guide, text) {
			t.Fatalf("the guide carries %q, which only an island's spec can say:\n%s", text, guide)
		}
	}
	for _, text := range []string{
		"the real views, controls, layouts and app functions are those the island's spec lists.",
		"app.<name>(input) calls an app function the island's spec lists and returns its result.",
		"except the tools of the page and js modules. To act on the page, the script calls app functions.",
		"The catalog in the island's spec lists each view, control, layout and app function; island_spec_schema gives the schemas of the ones you name.",
	} {
		if !strings.Contains(guide, text) {
			t.Fatalf("the guide does not say %q:\n%s", text, guide)
		}
	}
}

func TestSpecIsTheIslandsOwnPart(t *testing.T) {
	i, _ := newTestIsland(t)
	ctrl := testkit.NewController(t, testkit.Setup{Modules: []common.Module{timemod.Module(context.Background())}})
	spec, ends, items := i.Spec(ctrl.Ctx(), "board")
	want := []string{"the island_guide note", "your tools", "logs and files"}
	if !slices.Equal(items, want) || len(ends) != len(want) || ends[len(ends)-1] != len(spec) {
		t.Fatalf("items %v ending at %v of %d bytes, want %v:\n%s", items, ends, len(spec), want, spec)
	}
	texts := []string{
		"Read island_guide first if you have not in this conversation.\n",
		"\nYour tools\nThe script has time__*.\n",
		fmt.Sprintf(specLogsAndFiles, logFile("board"), islandDir("board")),
	}
	prev := 0
	for n, end := range ends {
		if item := spec[prev:end]; item != texts[n] || !strings.HasSuffix(item, "\n") || strings.HasSuffix(item, "\n\n") {
			t.Fatalf("the %s item reads:\n%s", items[n], item)
		}
		prev = end
	}
	if !strings.HasPrefix(texts[2], "\nLogs and files\nconsole.log, info, warn and error append to the workspace file board.island.log, ") || !strings.Contains(texts[2], "under the workspace dir board.island, ") {
		t.Fatalf("the logs and files item does not name the island's paths:\n%s", texts[2])
	}
	for _, text := range []string{guideWhatToBuild, guideHowAnAppRuns, guideScreen, guideViewsAndControls, guideAppFunctionsAndTools, guideTypicalLoop, guideReference} {
		if strings.Contains(spec, text) {
			t.Fatalf("the spec still carries a section of the guide:\n%s", spec)
		}
	}
	bare := testkit.NewController(t, testkit.Setup{})
	if spec, _, items := i.Spec(bare.Ctx(), "board"); !slices.Equal(items, []string{"the island_guide note", "logs and files"}) || strings.Contains(spec, "The script has") {
		t.Fatalf("with no module tools for the script, the spec has items %v:\n%s", items, spec)
	}
}
