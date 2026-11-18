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

package testkit

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"slices"
	"testing"
	"time"

	"neui.dev/api"
)

func MessageStorageUsers(t *testing.T, open func(t *testing.T) func(user string) api.MessageStorage) {
	for _, pair := range []struct{ name, owner, intruder string }{
		{"Named", "alice", "bob"},
		{"EmptyIntruder", "alice", ""},
		{"EmptyOwner", "", "alice"},
	} {
		t.Run(pair.name, func(t *testing.T) {
			for _, c := range []struct {
				name string
				run  func(t *testing.T, owner, intruder api.MessageStorage)
			}{
				{"Reads", usersReads},
				{"Writes", usersWrites},
				{"Clone", usersClone},
				{"Delete", usersDelete},
				{"SameIDs", usersSameIDs},
				{"Search", usersSearch},
			} {
				t.Run(c.name, func(t *testing.T) {
					user := open(t)
					c.run(t, user(pair.owner), user(pair.intruder))
				})
			}
		})
	}
}

func seedOwner(t *testing.T, s api.MessageStorage) api.Seq {
	t.Helper()
	ctx := context.Background()
	th := createThread(t, s, "t")
	seq, err := s.ThreadRename(ctx, "t", th.Seq, "Quarterly Report")
	if err != nil {
		t.Fatalf("ThreadRename: %v", err)
	}
	r0, seq := addInput(t, s, "t", seq, "the quarterly numbers")
	if seq, err = s.RoundOk(ctx, "t", seq, r0, api.EventUsage{Input: 3}, api.Indexed[api.Content]{
		0: api.ContentText{Text: "the quarterly answer"},
	}); err != nil {
		t.Fatalf("RoundOk: %v", err)
	}
	if seq, err = s.RoundResult(ctx, "t", seq, r0, map[string]api.ContentToolResult{
		"call": {ID: "call", Content: api.UserContents{api.ContentText{Text: "tool output"}}},
	}); err != nil {
		t.Fatalf("RoundResult: %v", err)
	}
	r1, seq := addInput(t, s, "t", seq, "more quarterly numbers")
	pinned, seq, err := s.RecordAdd(ctx, "t", seq, r0, "note", map[string]any{"k": "v"}, "pinned")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	if _, seq, err = s.RecordAdd(ctx, "t", seq, r1, "note", nil, "loose"); err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	if seq, err = s.RecordPin(ctx, "t", seq, r1, []api.RecordID{pinned}); err != nil {
		t.Fatalf("RecordPin: %v", err)
	}
	if seq, err = s.CompactionCreate(ctx, "t", seq, api.Compaction{Target: r1, Tail: r0, Text: "summary"}); err != nil {
		t.Fatalf("CompactionCreate: %v", err)
	}
	return seq
}

func ownerState(t *testing.T, s api.MessageStorage) []byte {
	t.Helper()
	ctx := context.Background()
	th, err := s.ThreadGet(ctx, "t")
	if err != nil {
		t.Fatalf("owner ThreadGet: %v", err)
	}
	records, err := s.RecordList(ctx, "t")
	if err != nil {
		t.Fatalf("owner RecordList: %v", err)
	}
	compaction, err := s.CompactionGet(ctx, "t", api.NoRoundID)
	if err != nil {
		t.Fatalf("owner CompactionGet: %v", err)
	}
	threads, err := s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("owner ThreadList: %v", err)
	}
	state, err := json.Marshal(map[string]any{
		"thread":     th,
		"rounds":     readRange(t, s, "t", api.NoRoundID, api.NoRoundID),
		"records":    records,
		"compaction": compaction,
		"threads":    threads,
	})
	if err != nil {
		t.Fatalf("encode owner state: %v", err)
	}
	return state
}

func checkOwnerState(t *testing.T, s api.MessageStorage, want []byte) {
	t.Helper()
	if got := ownerState(t, s); !bytes.Equal(got, want) {
		t.Fatalf("the owner's thread changed:\n got %s\nwant %s", got, want)
	}
}

func usersReads(t *testing.T, owner, intruder api.MessageStorage) {
	ctx := context.Background()
	seedOwner(t, owner)
	if _, err := intruder.ThreadGet(ctx, "t"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadGet: want ErrNotFound, got %v", err)
	}
	for _, name := range []string{"", "quarterly"} {
		threads, err := intruder.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, name)
		if err != nil {
			t.Fatalf("ThreadList(%q): %v", name, err)
		}
		if len(threads) != 0 {
			t.Fatalf("ThreadList(%q): got %v", name, idsOfThreads(threads))
		}
	}
	for _, err := range intruder.RoundRange(ctx, "t", api.NoRoundID, api.NoRoundID) {
		if !errors.Is(err, api.ErrNotFound) {
			t.Fatalf("RoundRange: want ErrNotFound, got %v", err)
		}
	}
	if _, err := intruder.RoundBefore(ctx, "t", api.NoRoundID, 10); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("RoundBefore: want ErrNotFound, got %v", err)
	}
	if _, err := intruder.RoundAfter(ctx, "t", api.NoRoundID, 10); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("RoundAfter: want ErrNotFound, got %v", err)
	}
	if _, err := intruder.CompactionGet(ctx, "t", api.NoRoundID); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("CompactionGet: want ErrNotFound, got %v", err)
	}
	if _, err := intruder.RecordList(ctx, "t"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("RecordList: want ErrNotFound, got %v", err)
	}
}

func usersWrites(t *testing.T, owner, intruder api.MessageStorage) {
	ctx := context.Background()
	seq := seedOwner(t, owner)
	want := ownerState(t, owner)
	usage := api.EventUsage{}
	content := api.Indexed[api.Content]{0: api.ContentText{Text: "hijack"}}
	for _, c := range []struct {
		name string
		run  func() error
	}{
		{"ThreadBump", func() error { _, err := intruder.ThreadBump(ctx, "t"); return err }},
		{"ThreadRename", func() error { _, err := intruder.ThreadRename(ctx, "t", seq, "taken"); return err }},
		{"ThreadCut", func() error { _, err := intruder.ThreadCut(ctx, "t", seq, api.NoRoundID); return err }},
		{"CompactionCreate", func() error {
			_, err := intruder.CompactionCreate(ctx, "t", seq, api.Compaction{Target: 0, Tail: 0, Text: "x"})
			return err
		}},
		{"RoundInput", func() error { _, _, err := intruder.RoundInput(ctx, "t", seq, api.Input{Text: "x"}); return err }},
		{"RoundOk", func() error { _, err := intruder.RoundOk(ctx, "t", seq, 0, usage, content); return err }},
		{"RoundRefuse", func() error { _, err := intruder.RoundRefuse(ctx, "t", seq, 0, usage, content); return err }},
		{"RoundError", func() error { _, err := intruder.RoundError(ctx, "t", seq, 0, "x", usage, content); return err }},
		{"RoundAbort", func() error { _, err := intruder.RoundAbort(ctx, "t", seq, 0, usage, content); return err }},
		{"RoundResult", func() error {
			_, err := intruder.RoundResult(ctx, "t", seq, 0, map[string]api.ContentToolResult{})
			return err
		}},
		{"RecordAdd", func() error { _, _, err := intruder.RecordAdd(ctx, "t", seq, 0, "note", nil, "x"); return err }},
		{"RecordPin", func() error { _, err := intruder.RecordPin(ctx, "t", seq, 0, []api.RecordID{1}); return err }},
	} {
		if err := c.run(); !errors.Is(err, api.ErrNotFound) {
			t.Fatalf("%s: want ErrNotFound, got %v", c.name, err)
		}
	}
	checkOwnerState(t, owner, want)
}

func usersClone(t *testing.T, owner, intruder api.MessageStorage) {
	ctx := context.Background()
	seedOwner(t, owner)
	want := ownerState(t, owner)
	if _, err := intruder.ThreadClone(ctx, "t", "copy"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadClone: want ErrNotFound, got %v", err)
	}
	for _, s := range []api.MessageStorage{owner, intruder} {
		if _, err := s.ThreadGet(ctx, "copy"); !errors.Is(err, api.ErrNotFound) {
			t.Fatalf("the refused clone left a thread: %v", err)
		}
	}
	createThread(t, intruder, "copy")
	checkOwnerState(t, owner, want)
}

func usersDelete(t *testing.T, owner, intruder api.MessageStorage) {
	ctx := context.Background()
	seedOwner(t, owner)
	want := ownerState(t, owner)
	if err := intruder.ThreadDelete(ctx, "t"); err != nil {
		t.Fatalf("ThreadDelete: %v", err)
	}
	checkOwnerState(t, owner, want)
}

func usersSameIDs(t *testing.T, owner, intruder api.MessageStorage) {
	ctx := context.Background()
	seedOwner(t, owner)
	want := ownerState(t, owner)
	if _, err := intruder.ThreadCreate(ctx, "t"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("ThreadCreate of the owner's id: want ErrExists, got %v", err)
	}
	mine := createThread(t, intruder, "mine")
	addInput(t, intruder, "mine", mine.Seq, "my own words")
	if _, err := intruder.ThreadClone(ctx, "mine", "t"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("ThreadClone onto the owner's id: want ErrExists, got %v", err)
	}
	if _, err := owner.ThreadClone(ctx, "t", "mine"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("ThreadClone onto the intruder's id: want ErrExists, got %v", err)
	}
	if _, err := owner.ThreadCreate(ctx, "mine"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("ThreadCreate of the intruder's id: want ErrExists, got %v", err)
	}
	checkOwnerState(t, owner, want)
	rounds := readRange(t, intruder, "mine", api.NoRoundID, api.NoRoundID)
	if len(rounds) != 1 || rounds[0].Input.Text != "my own words" {
		t.Fatalf("the intruder's own thread changed: %+v", rounds)
	}
}

func usersSearch(t *testing.T, owner, intruder api.MessageStorage) {
	ctx := context.Background()
	seedOwner(t, owner)
	mine := createThread(t, intruder, "mine")
	addInput(t, intruder, "mine", mine.Seq, "my quarterly numbers")
	for _, c := range []struct {
		name    string
		s       api.MessageStorage
		thread  api.ThreadID
		exclude bool
		want    []api.ThreadID
	}{
		{"intruder everywhere", intruder, "", false, []api.ThreadID{"mine"}},
		{"intruder in the owner's thread", intruder, "t", false, nil},
		{"intruder outside the owner's thread", intruder, "t", true, []api.ThreadID{"mine"}},
		{"owner everywhere", owner, "", false, []api.ThreadID{"t", "t"}},
		{"owner in the intruder's thread", owner, "mine", false, nil},
	} {
		hits, err := c.s.RoundSearch(ctx, c.thread, c.exclude, "quarterly", nil, false, false, 0)
		if err != nil {
			t.Fatalf("RoundSearch, %s: %v", c.name, err)
		}
		var got []api.ThreadID
		for _, h := range hits {
			got = append(got, h.Thread)
		}
		if !slices.Equal(got, c.want) {
			t.Fatalf("RoundSearch, %s: got %v, want %v", c.name, got, c.want)
		}
	}
}
