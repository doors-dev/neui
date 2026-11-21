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

package replay

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/testkit"
)

func TestProviderBlocksReplayByteIdentical(t *testing.T) {
	blocks := testkit.ProviderBlocks()
	model := api.Model{ImagesPerRequest: 6, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	s := local.NewMessageStorage()
	r := NewReplay(context.Background(), "a", s, api.ThreadConfig{ThreadMemoryLimit: -1, ReplayItemShare: 0.25, CompactionPreserve: 0.25}, model)
	mustCreate(t, r)
	for i := range 6 {
		name := fmt.Sprintf("c%d", i)
		if _, _, err := r.RoundInit(api.Input{Text: "round " + name}); err != nil {
			t.Fatalf("RoundInit: %v", err)
		}
		content := api.Indexed[api.Content]{}
		for j, c := range blocks {
			content[j] = c
		}
		content[len(blocks)] = api.ContentToolUse{ID: name, Name: "attach", Input: json.RawMessage(`{}`)}
		if _, err := r.RoundOk(api.EventUsage{Input: 100 * (i + 1)}, content); err != nil {
			t.Fatalf("RoundOk: %v", err)
		}
		if err := r.RoundResult(map[string]api.ContentToolResult{name: {ID: name, Content: shots(name, 2)}}); err != nil {
			t.Fatalf("RoundResult: %v", err)
		}
	}
	check := func(t *testing.T, messages []api.Message, rounds int) {
		t.Helper()
		seen := 0
		for _, m := range messages {
			if m.Role != api.RoleAssistant || len(m.Content) <= len(blocks) {
				continue
			}
			seen++
			if got := m.Content[:len(blocks)]; !reflect.DeepEqual(got, blocks) {
				t.Fatalf("an assistant message replays its blocks as\n%+v\nwant\n%+v", got, blocks)
			}
		}
		if seen != rounds {
			t.Fatalf("%d assistant messages carry the blocks, want %d", seen, rounds)
		}
	}
	replayed := func(t *testing.T, r Replay) []api.Message {
		t.Helper()
		messages, err := r.messages.replay(api.Input{})
		if err != nil {
			t.Fatalf("replay: %v", err)
		}
		return messages
	}
	t.Run("session", func(t *testing.T) {
		if r.messages.cursor == 0 {
			t.Fatalf("no image cut happened, so eviction is not exercised")
		}
		check(t, replayed(t, r), 6)
	})
	t.Run("reload", func(t *testing.T) {
		loaded := NewReplay(context.Background(), "a", s, r.config, r.model)
		if err := loaded.Load(); err != nil {
			t.Fatalf("Load: %v", err)
		}
		check(t, replayed(t, loaded), 6)
		r = loaded
	})
	t.Run("compaction", func(t *testing.T) {
		c, _, err := r.PrepareCompaction(true, 2)
		if err != nil {
			t.Fatalf("PrepareCompaction: %v", err)
		}
		if err := r.ApplyCompaction(c, api.Indexed[api.Content]{0: api.ContentText{Text: "summary"}}, api.EventUsage{Output: 10}); err != nil {
			t.Fatalf("ApplyCompaction: %v", err)
		}
		check(t, replayed(t, r), len(r.messages.rounds))
		loaded := NewReplay(context.Background(), "a", s, r.config, r.model)
		if err := loaded.Load(); err != nil {
			t.Fatalf("Load: %v", err)
		}
		check(t, replayed(t, loaded), len(r.messages.rounds))
		r = loaded
	})
	t.Run("rewind", func(t *testing.T) {
		_, messages, _, err := r.Rewind(r.messages.rounds[len(r.messages.rounds)-2].id, api.Input{Text: "instead"})
		if err != nil {
			t.Fatalf("Rewind: %v", err)
		}
		check(t, messages, 4)
		if r.messages.compaction.IsValid() {
			t.Fatalf("a rewind into the rounds the compaction kept still replays its summary")
		}
	})
}
