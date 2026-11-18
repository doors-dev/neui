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
	"errors"
	"strings"
	"testing"

	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/testkit"
)

type precheckNote struct {
	Text string `json:"text"`
}

func TestPrecheckRefusesPart(t *testing.T) {
	lib := NewLibrary()
	if err := lib.View("note", "", func(context.Context, precheckNote) (api.Widget[precheckNote], error) { return nil, nil }); err != nil {
		t.Fatal(err)
	}
	i := NewIsland(gox.Elem(func(gox.Cursor) error { return nil }), func(slot gox.Comp) gox.Elem { return slot.Main() }, nil, nil, lib)
	t.Cleanup(i.Unmount)
	ctx := context.Background()
	old := hungRun(i)
	refused := func(err error, want string) {
		t.Helper()
		if !errors.Is(err, errScriptRejected) || !strings.Contains(err.Error(), want) {
			t.Fatalf("got %v, want a refusal with %q", err, want)
		}
	}
	_, _, err := i.StartCode(ctx, "app", "", "var n = ui.note({ text: \"a\" })\nn.set()")
	refused(err, "line 2, column 1: TypeError: n.set: a view or control (ui.note) has no set, slots do: to change what it shows, call update(patch)")
	if i.run != old || i.seq != old.seq || old.ctx.Err() != nil {
		t.Fatal("a refused island_begin replaced the running app")
	}
	env, err := scriptEnv(ctx, lib)
	if err != nil {
		t.Fatal(err)
	}
	c := testkit.NewController(t, testkit.Setup{})
	if _, err := c.Task("", nil, nil); err != nil {
		t.Fatal(err)
	}
	old.task = c.Tasks()[0]
	old.env = env
	old.config.MaxQueue = 8
	if _, _, err := i.ContinueCode("var n = ui.note({ text: \"a\" })", false); err != nil {
		t.Fatal(err)
	}
	_, _, err = i.ContinueCode("var extra = ui.slot()\nn.set(extra)", true)
	refused(err, "line 2, column 1: TypeError: n.set:")
	if old.queue.Len() != 1 || old.scripts != 1 {
		t.Fatalf("a refused interrupt changed the queue: %d parts, %d scripts", old.queue.Len(), old.scripts)
	}
	_, _, err = i.ContinueCode("extra.set(n)", false)
	refused(err, "line 1, column 1: TypeError: extra is not defined: no part declared it, and the script has no such tool or built-in")
	if _, _, err := i.ContinueCode("n.update({ text: \"b\" })", false); err != nil {
		t.Fatal(err)
	}
	if old.queue.Len() != 2 || old.scripts != 2 {
		t.Fatalf("got %d parts, %d scripts, want 2 and 2", old.queue.Len(), old.scripts)
	}
}
