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
	"context"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
)

func MessageStorage(t *testing.T, open func(t *testing.T) api.MessageStorage) {
	for _, c := range []struct {
		name string
		run  func(t *testing.T, s api.MessageStorage)
	}{
		{"ThreadCreateDuplicate", storageThreadCreateDuplicate},
		{"ThreadBump", storageThreadBump},
		{"ThreadRename", storageThreadRename},
		{"ThreadList", storageThreadList},
		{"MissingThread", storageMissingThread},
		{"SeqGuard", storageSeqGuard},
		{"RoundLifecycle", storageRoundLifecycle},
		{"RoundResultReplaces", storageRoundResultReplaces},
		{"RoundRange", storageRoundRange},
		{"RoundRangeYieldsOutsideTheLock", storageRoundRangeYieldsOutsideTheLock},
		{"RoundBeforeAfter", storageRoundBeforeAfter},
		{"Compactions", storageCompactions},
		{"Records", storageRecords},
		{"PinnedRecordsHydration", storagePinnedRecordsHydration},
		{"ThreadCut", storageThreadCut},
		{"ThreadClone", storageThreadClone},
		{"ThreadDelete", storageThreadDelete},
		{"ThreadListName", storageThreadListName},
		{"RoundSearch", storageRoundSearch},
		{"RoundSearchCursor", storageRoundSearchCursor},
		{"RoundReadsOmitResults", storageRoundReadsOmitResults},
		{"RoundSearchSnippetAfterCaseChangingRunes", storageRoundSearchSnippetAfterCaseChangingRunes},
		{"ThreadListNameFoldsNonASCII", storageThreadListNameFoldsNonASCII},
		{"RoundSearchFoldsNonASCII", storageRoundSearchFoldsNonASCII},
	} {
		t.Run(c.name, func(t *testing.T) { c.run(t, open(t)) })
	}
}

func createThread(t *testing.T, s api.MessageStorage, id api.ThreadID) api.Thread {
	t.Helper()
	th, err := s.ThreadCreate(context.Background(), id)
	if err != nil {
		t.Fatalf("ThreadCreate(%q): %v", id, err)
	}
	if th.ID != id {
		t.Fatalf("ThreadCreate(%q): returned id %q", id, th.ID)
	}
	return th
}

func addInput(t *testing.T, s api.MessageStorage, thread api.ThreadID, seq api.Seq, text string) (api.RoundID, api.Seq) {
	t.Helper()
	id, next, err := s.RoundInput(context.Background(), thread, seq, api.Input{Text: text})
	if err != nil {
		t.Fatalf("RoundInput(%q, seq %d, %q): %v", thread, seq, text, err)
	}
	return id, next
}

func readRange(t *testing.T, s api.MessageStorage, thread api.ThreadID, start api.RoundID, end api.RoundID) []api.Round {
	t.Helper()
	var rounds []api.Round
	for r, err := range s.RoundRange(context.Background(), thread, start, end) {
		if err != nil {
			t.Fatalf("RoundRange(%q, %d, %d): %v", thread, start, end, err)
		}
		rounds = append(rounds, r)
	}
	return rounds
}

func readRound(t *testing.T, s api.MessageStorage, thread api.ThreadID, id api.RoundID) api.Round {
	t.Helper()
	rounds := readRange(t, s, thread, id, id)
	if len(rounds) != 1 || rounds[0].ID != id {
		t.Fatalf("RoundRange(%q, %d, %d): want the single round %d, got %v", thread, id, id, id, idsOfRounds(rounds))
	}
	return rounds[0]
}

func idsOfRounds(rounds []api.Round) []api.RoundID {
	ids := make([]api.RoundID, len(rounds))
	for i, r := range rounds {
		ids[i] = r.ID
	}
	return ids
}

func idsOfHeaders(headers []api.Record) []api.RecordID {
	ids := make([]api.RecordID, len(headers))
	for i, h := range headers {
		ids[i] = h.ID
	}
	return ids
}

func idsOfThreads(threads []api.Thread) []api.ThreadID {
	ids := make([]api.ThreadID, len(threads))
	for i, th := range threads {
		ids[i] = th.ID
	}
	return ids
}

func storageThreadCreateDuplicate(t *testing.T, s api.MessageStorage) {
	createThread(t, s, "a")
	if _, err := s.ThreadCreate(context.Background(), "a"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("duplicate ThreadCreate: want ErrExists, got %v", err)
	}
}

func storageThreadBump(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")

	first, err := s.ThreadBump(ctx, "a")
	if err != nil {
		t.Fatalf("ThreadBump: %v", err)
	}
	if first.ID != "a" {
		t.Fatalf("ThreadBump returned thread %q", first.ID)
	}
	second, err := s.ThreadBump(ctx, "a")
	if err != nil {
		t.Fatalf("second ThreadBump: %v", err)
	}
	if first.Seq == th.Seq || second.Seq == first.Seq {
		t.Fatalf("ThreadBump did not advance Seq: %d, %d, %d", th.Seq, first.Seq, second.Seq)
	}

	if _, _, err := s.RoundInput(ctx, "a", first.Seq, api.Input{Text: "stale"}); !errors.Is(err, api.ErrDrift) {
		t.Fatalf("write with a pre-bump seq: want ErrDrift, got %v", err)
	}
	if _, _, err := s.RoundInput(ctx, "a", second.Seq, api.Input{Text: "fresh"}); err != nil {
		t.Fatalf("write with the bumped seq: %v", err)
	}

	if _, err := s.ThreadBump(ctx, "missing"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadBump on a missing thread: want ErrNotFound, got %v", err)
	}
}

func storageThreadRename(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")

	seq, err := s.ThreadRename(ctx, "a", th.Seq, "billing migration")
	if err != nil {
		t.Fatalf("ThreadRename: %v", err)
	}
	named, err := s.ThreadGet(ctx, "a")
	if err != nil {
		t.Fatalf("ThreadGet after ThreadRename: %v", err)
	}
	if named.Name != "billing migration" {
		t.Fatalf("name after ThreadRename: got %q", named.Name)
	}
	if named.Seq != seq {
		t.Fatalf("ThreadRename returned seq %d, the thread is at %d", seq, named.Seq)
	}

	if _, err := s.ThreadRename(ctx, "a", th.Seq, "stale"); !errors.Is(err, api.ErrDrift) {
		t.Fatalf("ThreadRename with a stale seq: want ErrDrift, got %v", err)
	}
	if again, err := s.ThreadGet(ctx, "a"); err != nil || again.Name != "billing migration" {
		t.Fatalf("name after the stale rename: got %q, err %v", again.Name, err)
	}

	if _, err := s.ThreadRename(ctx, "missing", 0, "ghost"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadRename on a missing thread: want ErrNotFound, got %v", err)
	}
}

func storageThreadList(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()

	threads, err := s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList on an empty store: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("ThreadList on an empty store: got %v", idsOfThreads(threads))
	}

	createThread(t, s, "old")
	time.Sleep(5 * time.Millisecond)
	mid := createThread(t, s, "mid")
	time.Sleep(5 * time.Millisecond)
	createThread(t, s, "new")

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList: %v", err)
	}
	if want := []api.ThreadID{"new", "mid", "old"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList order: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, mid.Updated, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList from mid.Updated: %v", err)
	}
	if want := []api.ThreadID{"new", "mid"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList from mid.Updated: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, time.Time{}, mid.Updated, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList to mid.Updated: %v", err)
	}
	if want := []api.ThreadID{"old"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList to mid.Updated: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, false, 1, "")
	if err != nil {
		t.Fatalf("ThreadList limit 1: %v", err)
	}
	if want := []api.ThreadID{"new"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList limit 1: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, true, 0, "")
	if err != nil {
		t.Fatalf("ThreadList asc: %v", err)
	}
	if want := []api.ThreadID{"old", "mid", "new"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList asc: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, true, 1, "")
	if err != nil {
		t.Fatalf("ThreadList asc limit 1: %v", err)
	}
	if want := []api.ThreadID{"old"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList asc limit 1: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, false, -1, "")
	if err != nil {
		t.Fatalf("ThreadList limit -1: %v", err)
	}
	if len(threads) != 3 {
		t.Fatalf("ThreadList limit -1: got %v", idsOfThreads(threads))
	}
}

func storageMissingThread(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	if _, err := s.ThreadBump(ctx, "ghost"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadBump on a missing thread: want ErrNotFound, got %v", err)
	}
	if _, _, err := s.RoundInput(ctx, "ghost", 0, api.Input{Text: "x"}); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("RoundInput on a missing thread: want ErrNotFound, got %v", err)
	}
}

func storageSeqGuard(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")

	round, seq := addInput(t, s, "a", th.Seq, "hello")
	next, err := s.RoundOk(ctx, "a", seq, round, api.EventUsage{Output: 1}, api.Indexed[api.Content]{0: api.ContentText{Text: "hi"}})
	if err != nil {
		t.Fatalf("RoundOk with the seq returned by RoundInput: %v", err)
	}
	rec, live, err := s.RecordAdd(ctx, "a", next, round, "note", nil, "data")
	if err != nil {
		t.Fatalf("RecordAdd with the seq returned by RoundOk: %v", err)
	}

	stale := next
	staleWrites := []struct {
		name string
		call func() error
	}{
		{"RoundInput", func() error { _, _, err := s.RoundInput(ctx, "a", stale, api.Input{Text: "x"}); return err }},
		{"RoundOk", func() error { _, err := s.RoundOk(ctx, "a", stale, round, api.EventUsage{}, nil); return err }},
		{"RoundRefuse", func() error { _, err := s.RoundRefuse(ctx, "a", stale, round, api.EventUsage{}, nil); return err }},
		{"RoundError", func() error {
			_, err := s.RoundError(ctx, "a", stale, round, "boom", api.EventUsage{}, nil)
			return err
		}},
		{"RoundResult", func() error {
			_, err := s.RoundResult(ctx, "a", stale, round, map[string]api.ContentToolResult{"c": {ID: "c"}})
			return err
		}},
		{"RoundAbort", func() error { _, err := s.RoundAbort(ctx, "a", stale, round, api.EventUsage{}, nil); return err }},
		{"RecordAdd", func() error { _, _, err := s.RecordAdd(ctx, "a", stale, round, "extra", nil, "x"); return err }},
		{"RecordPin", func() error { _, err := s.RecordPin(ctx, "a", stale, round, []api.RecordID{rec}); return err }},
		{"ThreadCut", func() error { _, err := s.ThreadCut(ctx, "a", stale, api.NoRoundID); return err }},
		{"ThreadRename", func() error { _, err := s.ThreadRename(ctx, "a", stale, "x"); return err }},
		{"CompactionCreate", func() error {
			_, err := s.CompactionCreate(ctx, "a", stale, api.Compaction{Target: round, Tail: round, Text: "x"})
			return err
		}},
	}
	for _, w := range staleWrites {
		if err := w.call(); !errors.Is(err, api.ErrDrift) {
			t.Fatalf("%s with a stale seq: want ErrDrift, got %v", w.name, err)
		}
	}

	rounds := readRange(t, s, "a", api.NoRoundID, api.NoRoundID)
	if !reflect.DeepEqual(idsOfRounds(rounds), []api.RoundID{round}) {
		t.Fatalf("rounds after stale writes: got %v, want [%d]", idsOfRounds(rounds), round)
	}
	got := rounds[0]
	if got.State != api.RoundOk || got.Error != "" || len(got.Results) != 0 {
		t.Fatalf("round mutated by stale writes: state %q, error %q, %d results", got.State, got.Error, len(got.Results))
	}
	list, err := s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList: %v", err)
	}
	if !reflect.DeepEqual(idsOfHeaders(list), []api.RecordID{rec}) {
		t.Fatalf("records after stale writes: got %v, want [%d]", idsOfHeaders(list), rec)
	}
	if _, _, err := s.RoundInput(ctx, "a", live, api.Input{Text: "still live"}); err != nil {
		t.Fatalf("RoundInput with the seq returned by RecordAdd: %v", err)
	}
}

func storageRoundLifecycle(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	seq := th.Seq

	input := api.Input{Text: "question", Attachments: []string{"a.txt", "img/pic.png"}}
	rIn, seq, err := s.RoundInput(ctx, "a", seq, input)
	if err != nil {
		t.Fatalf("RoundInput: %v", err)
	}
	rOk, seq := addInput(t, s, "a", seq, "ok round")
	rRefuse, seq := addInput(t, s, "a", seq, "refused round")
	rErr, seq := addInput(t, s, "a", seq, "errored round")
	rAbort, seq := addInput(t, s, "a", seq, "aborted round")

	got := readRound(t, s, "a", rIn)
	if got.State != api.RoundInput {
		t.Fatalf("fresh round state: got %q, want input", got.State)
	}
	if !reflect.DeepEqual(got.Input, input) {
		t.Fatalf("input roundtrip: got %+v, want %+v", got.Input, input)
	}

	usage := api.EventUsage{Input: 10, Output: 3, Reasoning: 2, CacheRead: 7, CacheWrite: 5}
	response := api.Indexed[api.Content]{
		0: api.ContentText{Text: "sure"},
		2: api.ContentText{Text: "done"},
	}
	if seq, err = s.RoundOk(ctx, "a", seq, rOk, usage, response); err != nil {
		t.Fatalf("RoundOk: %v", err)
	}
	got = readRound(t, s, "a", rOk)
	if got.State != api.RoundOk {
		t.Fatalf("RoundOk state: got %q", got.State)
	}
	if got.Usage != usage {
		t.Fatalf("RoundOk usage: got %+v, want %+v", got.Usage, usage)
	}
	if !reflect.DeepEqual(got.Response, response) {
		t.Fatalf("RoundOk response: got %+v, want %+v", got.Response, response)
	}

	if seq, err = s.RoundRefuse(ctx, "a", seq, rRefuse, api.EventUsage{}, nil); err != nil {
		t.Fatalf("RoundRefuse: %v", err)
	}
	if got = readRound(t, s, "a", rRefuse); got.State != api.RoundRefuse {
		t.Fatalf("RoundRefuse state: got %q", got.State)
	}

	if seq, err = s.RoundError(ctx, "a", seq, rErr, "socket closed", api.EventUsage{}, nil); err != nil {
		t.Fatalf("RoundError: %v", err)
	}
	got = readRound(t, s, "a", rErr)
	if got.State != api.RoundError || got.Error != "socket closed" {
		t.Fatalf("RoundError: state %q, error %q", got.State, got.Error)
	}

	if _, err = s.RoundAbort(ctx, "a", seq, rAbort, api.EventUsage{}, nil); err != nil {
		t.Fatalf("RoundAbort: %v", err)
	}
	if got = readRound(t, s, "a", rAbort); got.State != api.RoundAbort {
		t.Fatalf("RoundAbort state: got %q", got.State)
	}

	miss := rAbort + 1000
	mutators := []struct {
		name string
		call func(seq api.Seq) error
	}{
		{"RoundOk", func(seq api.Seq) error { _, err := s.RoundOk(ctx, "a", seq, miss, api.EventUsage{}, nil); return err }},
		{"RoundRefuse", func(seq api.Seq) error {
			_, err := s.RoundRefuse(ctx, "a", seq, miss, api.EventUsage{}, nil)
			return err
		}},
		{"RoundError", func(seq api.Seq) error {
			_, err := s.RoundError(ctx, "a", seq, miss, "x", api.EventUsage{}, nil)
			return err
		}},
		{"RoundResult", func(seq api.Seq) error { _, err := s.RoundResult(ctx, "a", seq, miss, nil); return err }},
		{"RoundAbort", func(seq api.Seq) error {
			_, err := s.RoundAbort(ctx, "a", seq, miss, api.EventUsage{}, nil)
			return err
		}},
	}
	for _, m := range mutators {
		bumped, err := s.ThreadBump(ctx, "a")
		if err != nil {
			t.Fatalf("ThreadBump before %s: %v", m.name, err)
		}
		if err := m.call(bumped.Seq); !errors.Is(err, api.ErrNotFound) {
			t.Fatalf("%s on a missing round: want ErrNotFound, got %v", m.name, err)
		}
	}
}

func storageRoundResultReplaces(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	round, seq := addInput(t, s, "a", th.Seq, "tools")

	first := map[string]api.ContentToolResult{
		"call-1": {ID: "call-1", Content: api.UserContents{api.ContentText{Text: "one"}}},
		"call-2": {ID: "call-2", Content: api.UserContents{api.ContentText{Text: "two"}}, Error: api.ToolErrRuntime},
	}
	seq, err := s.RoundResult(ctx, "a", seq, round, first)
	if err != nil {
		t.Fatalf("RoundResult: %v", err)
	}
	if got := readRound(t, s, "a", round).Results; !reflect.DeepEqual(got, first) {
		t.Fatalf("results: got %+v, want %+v", got, first)
	}

	second := map[string]api.ContentToolResult{
		"call-3": {ID: "call-3", Content: api.UserContents{api.ContentText{Text: "three"}}},
	}
	if _, err := s.RoundResult(ctx, "a", seq, round, second); err != nil {
		t.Fatalf("second RoundResult: %v", err)
	}
	if got := readRound(t, s, "a", round).Results; !reflect.DeepEqual(got, second) {
		t.Fatalf("results after replace: got %+v, want %+v", got, second)
	}
}

func storageRoundRange(t *testing.T, s api.MessageStorage) {
	th := createThread(t, s, "a")
	seq := th.Seq
	ids := make([]api.RoundID, 5)
	for i := range ids {
		ids[i], seq = addInput(t, s, "a", seq, fmt.Sprintf("round %d", i))
	}

	cases := []struct {
		name  string
		start api.RoundID
		end   api.RoundID
		want  []api.RoundID
	}{
		{"both bounds inclusive", ids[1], ids[3], ids[1:4]},
		{"open start", api.NoRoundID, ids[2], ids[:3]},
		{"open end", ids[2], api.NoRoundID, ids[2:]},
		{"open both", api.NoRoundID, api.NoRoundID, ids},
		{"single round", ids[2], ids[2], ids[2:3]},
	}
	for _, c := range cases {
		got := idsOfRounds(readRange(t, s, "a", c.start, c.end))
		if !reflect.DeepEqual(got, c.want) {
			t.Fatalf("RoundRange %s (%d..%d): got %v, want %v", c.name, c.start, c.end, got, c.want)
		}
	}

	createThread(t, s, "empty")
	if got := readRange(t, s, "empty", api.NoRoundID, api.NoRoundID); len(got) != 0 {
		t.Fatalf("RoundRange on an empty thread: got %v", idsOfRounds(got))
	}

	var rangeErr error
	for _, err := range s.RoundRange(context.Background(), "ghost", api.NoRoundID, api.NoRoundID) {
		if err == nil {
			t.Fatalf("RoundRange on a missing thread yielded a round")
		}
		rangeErr = err
		break
	}
	if !errors.Is(rangeErr, api.ErrNotFound) {
		t.Fatalf("RoundRange on a missing thread: want ErrNotFound, got %v", rangeErr)
	}
}

func storageRoundRangeYieldsOutsideTheLock(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	seq := th.Seq
	ids := make([]api.RoundID, 3)
	for i := range ids {
		ids[i], seq = addInput(t, s, "a", seq, fmt.Sprintf("round %d", i))
	}

	var got []api.RoundID
	injected := false
	for r, err := range s.RoundRange(ctx, "a", api.NoRoundID, api.NoRoundID) {
		if err != nil {
			t.Fatalf("RoundRange: %v", err)
		}
		if !injected {
			injected = true
			bumped, err := s.ThreadBump(ctx, "a")
			if err != nil {
				t.Fatalf("ThreadBump inside the iteration: %v", err)
			}
			if _, _, err := s.RoundInput(ctx, "a", bumped.Seq, api.Input{Text: "mid-iteration"}); err != nil {
				t.Fatalf("RoundInput inside the iteration: %v", err)
			}
		}
		got = append(got, r.ID)
	}
	if !reflect.DeepEqual(got, ids) {
		t.Fatalf("iteration left its snapshot: got %v, want %v", got, ids)
	}
}

func storageRoundBeforeAfter(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	seq := th.Seq
	ids := make([]api.RoundID, 5)
	for i := range ids {
		ids[i], seq = addInput(t, s, "a", seq, fmt.Sprintf("round %d", i))
	}

	before := func(start api.RoundID, limit int) []api.RoundID {
		t.Helper()
		rounds, err := s.RoundBefore(ctx, "a", start, limit)
		if err != nil {
			t.Fatalf("RoundBefore(%d, %d): %v", start, limit, err)
		}
		return idsOfRounds(rounds)
	}
	after := func(start api.RoundID, limit int) []api.RoundID {
		t.Helper()
		rounds, err := s.RoundAfter(ctx, "a", start, limit)
		if err != nil {
			t.Fatalf("RoundAfter(%d, %d): %v", start, limit, err)
		}
		return idsOfRounds(rounds)
	}

	if got := before(ids[3], 2); !reflect.DeepEqual(got, ids[1:3]) {
		t.Fatalf("RoundBefore(%d, 2): got %v, want %v", ids[3], got, ids[1:3])
	}
	if got := before(ids[3], 10); !reflect.DeepEqual(got, ids[:3]) {
		t.Fatalf("RoundBefore(%d, 10): got %v, want %v", ids[3], got, ids[:3])
	}
	if got := before(ids[0], 3); len(got) != 0 {
		t.Fatalf("RoundBefore(%d, 3): got %v, want none", ids[0], got)
	}
	if got := before(api.NoRoundID, 2); !reflect.DeepEqual(got, ids[3:]) {
		t.Fatalf("RoundBefore(NoRoundID, 2): got %v, want %v", got, ids[3:])
	}

	if got := after(ids[1], 2); !reflect.DeepEqual(got, ids[2:4]) {
		t.Fatalf("RoundAfter(%d, 2): got %v, want %v", ids[1], got, ids[2:4])
	}
	if got := after(ids[1], 10); !reflect.DeepEqual(got, ids[2:]) {
		t.Fatalf("RoundAfter(%d, 10): got %v, want %v", ids[1], got, ids[2:])
	}
	if got := after(ids[4], 2); len(got) != 0 {
		t.Fatalf("RoundAfter(%d, 2): got %v, want none", ids[4], got)
	}
	if got := after(api.NoRoundID, 2); !reflect.DeepEqual(got, ids[:2]) {
		t.Fatalf("RoundAfter(NoRoundID, 2): got %v, want %v", got, ids[:2])
	}
}

func storageCompactions(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	seq := th.Seq

	comp, err := s.CompactionGet(ctx, "a", api.NoRoundID)
	if err != nil {
		t.Fatalf("CompactionGet on an empty thread: %v", err)
	}
	if comp != api.NoCompaction {
		t.Fatalf("CompactionGet on an empty thread: got %+v", comp)
	}

	ids := make([]api.RoundID, 4)
	for i := range ids {
		ids[i], seq = addInput(t, s, "a", seq, fmt.Sprintf("round %d", i))
	}

	if seq, err = s.CompactionCreate(ctx, "a", seq, api.Compaction{Target: ids[0], Tail: ids[1], Text: "first summary", Output: 120, Base: 900}); err != nil {
		t.Fatalf("CompactionCreate: %v", err)
	}
	if seq, err = s.CompactionCreate(ctx, "a", seq, api.Compaction{Target: ids[2], Tail: ids[3], Text: "second summary", Output: 80, Base: 700}); err != nil {
		t.Fatalf("second CompactionCreate: %v", err)
	}
	_, seq = addInput(t, s, "a", seq, "post-compaction")

	first := api.Compaction{Text: "first summary", Target: ids[0], Tail: ids[1], Output: 120, Base: 900}
	second := api.Compaction{Text: "second summary", Target: ids[2], Tail: ids[3], Output: 80, Base: 700}
	reads := []struct {
		name       string
		beforeTail api.RoundID
		want       api.Compaction
	}{
		{"NoRoundID means the latest", api.NoRoundID, second},
		{"Tail < beforeTail skips the head compaction", ids[3], first},
		{"beforeTail between the tails", ids[2], first},
		{"nothing below beforeTail", ids[1], api.NoCompaction},
	}
	for _, c := range reads {
		comp, err := s.CompactionGet(ctx, "a", c.beforeTail)
		if err != nil {
			t.Fatalf("CompactionGet, %s: %v", c.name, err)
		}
		if comp != c.want {
			t.Fatalf("CompactionGet, %s: got %+v, want %+v", c.name, comp, c.want)
		}
	}
}

func storageRecords(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	r0, seq := addInput(t, s, "a", th.Seq, "one")
	r1, seq := addInput(t, s, "a", seq, "two")

	attrs := map[string]any{"path": "notes/x.md", "size": "12"}
	recFile, seq, err := s.RecordAdd(ctx, "a", seq, r0, "file", attrs, "the payload")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	recNote, seq, err := s.RecordAdd(ctx, "a", seq, r0, "note", nil, "second")
	if err != nil {
		t.Fatalf("second RecordAdd: %v", err)
	}
	recLast, seq, err := s.RecordAdd(ctx, "a", seq, r1, "note", nil, "third")
	if err != nil {
		t.Fatalf("third RecordAdd: %v", err)
	}
	if recFile >= recNote || recNote >= recLast {
		t.Fatalf("record ids not increasing: %d, %d, %d", recFile, recNote, recLast)
	}

	list, err := s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList: %v", err)
	}
	if want := []api.RecordID{recFile, recNote, recLast}; !reflect.DeepEqual(idsOfHeaders(list), want) {
		t.Fatalf("RecordList: got %v, want %v", idsOfHeaders(list), want)
	}
	if list[0].Kind != "file" || !reflect.DeepEqual(list[0].Attrs, attrs) {
		t.Fatalf("RecordList record: got %+v", list[0])
	}
	if data, ok := list[0].Data.(string); !ok || data != "the payload" {
		t.Fatalf("RecordList data: got %+v", list[0].Data)
	}

	if seq, err = s.RecordPin(ctx, "a", seq, r1, []api.RecordID{recNote}); err != nil {
		t.Fatalf("RecordPin: %v", err)
	}
	list, err = s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList after pin: %v", err)
	}
	if want := []api.RecordID{recFile, recLast}; !reflect.DeepEqual(idsOfHeaders(list), want) {
		t.Fatalf("RecordList after pin: got %v, want %v", idsOfHeaders(list), want)
	}
	if _, err := s.RecordPin(ctx, "a", seq, r1, []api.RecordID{recLast, recLast + 1000}); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("RecordPin with a missing id: want ErrNotFound, got %v", err)
	}
	list, err = s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList after the failed pin: %v", err)
	}
	if want := []api.RecordID{recFile, recLast}; !reflect.DeepEqual(idsOfHeaders(list), want) {
		t.Fatalf("RecordList after the failed pin: got %v, want %v", idsOfHeaders(list), want)
	}
}

func storagePinnedRecordsHydration(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	r0, seq := addInput(t, s, "a", th.Seq, "one")
	r1, seq := addInput(t, s, "a", seq, "two")
	r2, seq := addInput(t, s, "a", seq, "three")

	attrs := map[string]any{"name": "alpha"}
	ra, seq, err := s.RecordAdd(ctx, "a", seq, r0, "alpha", attrs, "a")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	rb, seq, err := s.RecordAdd(ctx, "a", seq, r0, "beta", nil, "b")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	rc, seq, err := s.RecordAdd(ctx, "a", seq, r1, "gamma", nil, "c")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	rd, seq, err := s.RecordAdd(ctx, "a", seq, r1, "delta", nil, "d")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}

	if seq, err = s.RecordPin(ctx, "a", seq, r0, []api.RecordID{rb, ra}); err != nil {
		t.Fatalf("RecordPin: %v", err)
	}
	if _, err = s.RecordPin(ctx, "a", seq, r1, []api.RecordID{rc}); err != nil {
		t.Fatalf("second RecordPin: %v", err)
	}

	check := func(source string, rounds []api.Round, data bool) {
		t.Helper()
		if len(rounds) != 3 {
			t.Fatalf("%s: got rounds %v", source, idsOfRounds(rounds))
		}
		byID := map[api.RoundID][]api.Record{}
		for _, r := range rounds {
			byID[r.ID] = r.PinnedRecords
		}
		if got := idsOfHeaders(byID[r0]); !reflect.DeepEqual(got, []api.RecordID{ra, rb}) {
			t.Fatalf("%s: round %d pinned records %v, want [%d %d]", source, r0, got, ra, rb)
		}
		if got := idsOfHeaders(byID[r1]); !reflect.DeepEqual(got, []api.RecordID{rc}) {
			t.Fatalf("%s: round %d pinned records %v, want [%d]", source, r1, got, rc)
		}
		if got := byID[r2]; len(got) != 0 {
			t.Fatalf("%s: round %d pinned records %v, want none", source, r2, idsOfHeaders(got))
		}
		first := byID[r0][0]
		if first.Kind != "alpha" || !reflect.DeepEqual(first.Attrs, attrs) {
			t.Fatalf("%s: hydrated record %+v", source, first)
		}
		if !data {
			if first.Data != nil {
				t.Fatalf("%s: hydrated record carries data %#v, want none", source, first.Data)
			}
		} else if got, ok := first.Data.(string); !ok || got != "a" {
			t.Fatalf("%s: hydrated record data %#v, want %q", source, first.Data, "a")
		}
	}

	check("RoundRange", readRange(t, s, "a", api.NoRoundID, api.NoRoundID), true)
	rounds, err := s.RoundBefore(ctx, "a", api.NoRoundID, 10)
	if err != nil {
		t.Fatalf("RoundBefore: %v", err)
	}
	check("RoundBefore", rounds, false)
	rounds, err = s.RoundAfter(ctx, "a", api.NoRoundID, 10)
	if err != nil {
		t.Fatalf("RoundAfter: %v", err)
	}
	check("RoundAfter", rounds, false)

	list, err := s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList: %v", err)
	}
	if !reflect.DeepEqual(idsOfHeaders(list), []api.RecordID{rd}) {
		t.Fatalf("unpinned records: got %v, want [%d]", idsOfHeaders(list), rd)
	}
}

func storageThreadCut(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "a")
	r0, seq := addInput(t, s, "a", th.Seq, "zero")
	r1, seq := addInput(t, s, "a", seq, "one")
	r2, seq := addInput(t, s, "a", seq, "two")
	r3, seq := addInput(t, s, "a", seq, "three")

	seq, err := s.CompactionCreate(ctx, "a", seq, api.Compaction{Target: r0, Tail: r1, Text: "keep"})
	if err != nil {
		t.Fatalf("CompactionCreate: %v", err)
	}
	if seq, err = s.CompactionCreate(ctx, "a", seq, api.Compaction{Target: r2, Tail: r3, Text: "drop"}); err != nil {
		t.Fatalf("second CompactionCreate: %v", err)
	}

	kept, seq, err := s.RecordAdd(ctx, "a", seq, r0, "kept", nil, "kept")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	unpinned, seq, err := s.RecordAdd(ctx, "a", seq, r1, "unpinned", nil, "unpinned")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	gone, seq, err := s.RecordAdd(ctx, "a", seq, r2, "gone", nil, "gone")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	if seq, err = s.RecordPin(ctx, "a", seq, r1, []api.RecordID{kept}); err != nil {
		t.Fatalf("RecordPin: %v", err)
	}
	if seq, err = s.RecordPin(ctx, "a", seq, r2, []api.RecordID{unpinned}); err != nil {
		t.Fatalf("second RecordPin: %v", err)
	}

	if seq, err = s.ThreadCut(ctx, "a", seq, r2); err != nil {
		t.Fatalf("ThreadCut(%d): %v", r2, err)
	}
	if got := idsOfRounds(readRange(t, s, "a", api.NoRoundID, api.NoRoundID)); !reflect.DeepEqual(got, []api.RoundID{r0, r1}) {
		t.Fatalf("rounds after cut: got %v, want [%d %d]", got, r0, r1)
	}

	comp, err := s.CompactionGet(ctx, "a", api.NoRoundID)
	if err != nil {
		t.Fatalf("CompactionGet after cut: %v", err)
	}
	if want := (api.Compaction{Text: "keep", Target: r0, Tail: r1}); comp != want {
		t.Fatalf("compaction after cut: got %+v, want %+v", comp, want)
	}

	list, err := s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList after cut: %v", err)
	}
	if !reflect.DeepEqual(idsOfHeaders(list), []api.RecordID{unpinned}) {
		t.Fatalf("unpinned records after cut: got %v, want [%d]", idsOfHeaders(list), unpinned)
	}
	if got := idsOfHeaders(readRound(t, s, "a", r1).PinnedRecords); !reflect.DeepEqual(got, []api.RecordID{kept}) {
		t.Fatalf("hydration after cut: got %v, want [%d]", got, kept)
	}

	rNew, seq := addInput(t, s, "a", seq, "after cut")
	if rNew <= r3 {
		t.Fatalf("round id after cut: got %d, want above %d", rNew, r3)
	}

	if seq, err = s.ThreadCut(ctx, "a", seq, api.NoRoundID); err != nil {
		t.Fatalf("ThreadCut(NoRoundID): %v", err)
	}
	if got := readRange(t, s, "a", api.NoRoundID, api.NoRoundID); len(got) != 0 {
		t.Fatalf("rounds after the full cut: got %v", idsOfRounds(got))
	}
	comp, err = s.CompactionGet(ctx, "a", api.NoRoundID)
	if err != nil {
		t.Fatalf("CompactionGet after the full cut: %v", err)
	}
	if comp != api.NoCompaction {
		t.Fatalf("compaction after the full cut: got %+v", comp)
	}
	list, err = s.RecordList(ctx, "a")
	if err != nil {
		t.Fatalf("RecordList after the full cut: %v", err)
	}
	if len(list) != 0 {
		t.Fatalf("records after the full cut: got %v", idsOfHeaders(list))
	}

	rAfter, seq := addInput(t, s, "a", seq, "fresh start")
	if rAfter <= rNew {
		t.Fatalf("round id after the full cut: got %d, want above %d", rAfter, rNew)
	}
	recAfter, _, err := s.RecordAdd(ctx, "a", seq, rAfter, "note", nil, "fresh")
	if err != nil {
		t.Fatalf("RecordAdd after the full cut: %v", err)
	}
	if recAfter <= gone {
		t.Fatalf("record id after the full cut: got %d, want above %d", recAfter, gone)
	}
}

func storageThreadClone(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	src := createThread(t, s, "src")
	s0, seq := addInput(t, s, "src", src.Seq, "first")
	s1, seq := addInput(t, s, "src", seq, "second")

	usage := api.EventUsage{Input: 4, Output: 2}
	response := api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}
	seq, err := s.RoundOk(ctx, "src", seq, s0, usage, response)
	if err != nil {
		t.Fatalf("RoundOk: %v", err)
	}
	rec, seq, err := s.RecordAdd(ctx, "src", seq, s0, "note", map[string]any{"k": "v"}, "payload")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	if _, err = s.CompactionCreate(ctx, "src", seq, api.Compaction{Target: s0, Tail: s1, Text: "summary", Output: 30, Base: 400}); err != nil {
		t.Fatalf("CompactionCreate: %v", err)
	}

	clone, err := s.ThreadClone(ctx, "src", "dst")
	if err != nil {
		t.Fatalf("ThreadClone: %v", err)
	}
	if clone.ID != "dst" {
		t.Fatalf("ThreadClone returned thread %q", clone.ID)
	}

	dstRounds := readRange(t, s, "dst", api.NoRoundID, api.NoRoundID)
	if got := idsOfRounds(dstRounds); !reflect.DeepEqual(got, []api.RoundID{s0, s1}) {
		t.Fatalf("cloned rounds: got %v, want [%d %d]", got, s0, s1)
	}
	if dstRounds[0].State != api.RoundOk || !reflect.DeepEqual(dstRounds[0].Response, response) {
		t.Fatalf("cloned round %d: %+v", s0, dstRounds[0])
	}
	if dstRounds[1].State != api.RoundInput || dstRounds[1].Input.Text != "second" {
		t.Fatalf("cloned round %d: %+v", s1, dstRounds[1])
	}
	comp, err := s.CompactionGet(ctx, "dst", api.NoRoundID)
	if err != nil {
		t.Fatalf("CompactionGet on the clone: %v", err)
	}
	if want := (api.Compaction{Text: "summary", Target: s0, Tail: s1, Output: 30, Base: 400}); comp != want {
		t.Fatalf("cloned compaction: got %+v, want %+v", comp, want)
	}

	if _, err := s.ThreadClone(ctx, "ghost", "elsewhere"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadClone from a missing source: want ErrNotFound, got %v", err)
	}
	if _, err := s.ThreadClone(ctx, "src", "dst"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("ThreadClone onto a taken target: want ErrExists, got %v", err)
	}

	dseq := clone.Seq
	if dseq, err = s.RoundOk(ctx, "dst", dseq, s1, api.EventUsage{Output: 9}, api.Indexed[api.Content]{0: api.ContentText{Text: "clone answer"}}); err != nil {
		t.Fatalf("RoundOk on the clone: %v", err)
	}
	results := map[string]api.ContentToolResult{"c": {ID: "c", Content: api.UserContents{api.ContentText{Text: "r"}}}}
	if dseq, err = s.RoundResult(ctx, "dst", dseq, s0, results); err != nil {
		t.Fatalf("RoundResult on the clone: %v", err)
	}
	if dseq, err = s.RecordPin(ctx, "dst", dseq, s1, []api.RecordID{rec}); err != nil {
		t.Fatalf("RecordPin on the clone: %v", err)
	}
	dstList, err := s.RecordList(ctx, "dst")
	if err != nil {
		t.Fatalf("RecordList on the clone: %v", err)
	}
	if len(dstList) != 0 {
		t.Fatalf("clone unpinned records after pin: got %v", idsOfHeaders(dstList))
	}

	if got := readRound(t, s, "src", s1); got.State != api.RoundInput {
		t.Fatalf("source round %d after a clone write: state %q, want input", s1, got.State)
	}
	if got := readRound(t, s, "src", s0); len(got.Results) != 0 {
		t.Fatalf("source round %d after a clone write: results %+v", s0, got.Results)
	}
	srcList, err := s.RecordList(ctx, "src")
	if err != nil {
		t.Fatalf("RecordList on the source: %v", err)
	}
	if !reflect.DeepEqual(idsOfHeaders(srcList), []api.RecordID{rec}) {
		t.Fatalf("source unpinned records: got %v, want [%d]", idsOfHeaders(srcList), rec)
	}

	bumped, err := s.ThreadBump(ctx, "src")
	if err != nil {
		t.Fatalf("ThreadBump on the source: %v", err)
	}
	sseq := bumped.Seq
	if sseq, err = s.RoundAbort(ctx, "src", sseq, s1, api.EventUsage{}, nil); err != nil {
		t.Fatalf("RoundAbort on the source: %v", err)
	}
	if sseq, err = s.CompactionCreate(ctx, "src", sseq, api.Compaction{Target: s1, Tail: s1, Text: "late summary"}); err != nil {
		t.Fatalf("late CompactionCreate: %v", err)
	}
	if got := readRound(t, s, "dst", s1); got.State != api.RoundOk {
		t.Fatalf("clone round %d after a source write: state %q, want ok", s1, got.State)
	}
	comp, err = s.CompactionGet(ctx, "dst", api.NoRoundID)
	if err != nil {
		t.Fatalf("CompactionGet on the clone: %v", err)
	}
	if comp.Text != "summary" {
		t.Fatalf("clone compaction after a source write: got %+v", comp)
	}

	dNew, dseq := addInput(t, s, "dst", dseq, "clone grows")
	if dNew <= s1 {
		t.Fatalf("clone round id: got %d, want above %d", dNew, s1)
	}
	dRec, _, err := s.RecordAdd(ctx, "dst", dseq, dNew, "note", nil, "x")
	if err != nil {
		t.Fatalf("RecordAdd on the clone: %v", err)
	}
	if dRec <= rec {
		t.Fatalf("clone record id: got %d, want above %d", dRec, rec)
	}
	sNew, _ := addInput(t, s, "src", sseq, "source grows")
	if sNew <= s1 {
		t.Fatalf("source round id: got %d, want above %d", sNew, s1)
	}
}

func storageThreadDelete(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	th := createThread(t, s, "gone")
	other := createThread(t, s, "stays")
	r0, seq := addInput(t, s, "gone", th.Seq, "first")
	r1, seq := addInput(t, s, "gone", seq, "second")
	_, seq, err := s.RecordAdd(ctx, "gone", seq, r0, "note", map[string]any{"k": "v"}, "payload")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	if _, err := s.CompactionCreate(ctx, "gone", seq, api.Compaction{Target: r0, Tail: r1, Text: "summary"}); err != nil {
		t.Fatalf("CompactionCreate: %v", err)
	}
	kept, _ := addInput(t, s, "stays", other.Seq, "untouched")

	if err := s.ThreadDelete(ctx, "gone"); err != nil {
		t.Fatalf("ThreadDelete: %v", err)
	}
	if _, err := s.ThreadGet(ctx, "gone"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ThreadGet after the delete: want ErrNotFound, got %v", err)
	}
	for _, err := range s.RoundRange(ctx, "gone", api.NoRoundID, api.NoRoundID) {
		if !errors.Is(err, api.ErrNotFound) {
			t.Fatalf("RoundRange after the delete: want ErrNotFound, got %v", err)
		}
	}
	if _, err := s.CompactionGet(ctx, "gone", api.NoRoundID); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("CompactionGet after the delete: want ErrNotFound, got %v", err)
	}
	if _, err := s.RecordList(ctx, "gone"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("RecordList after the delete: want ErrNotFound, got %v", err)
	}
	threads, err := s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList after the delete: %v", err)
	}
	if want := []api.ThreadID{"stays"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList after the delete: got %v, want %v", idsOfThreads(threads), want)
	}

	if err := s.ThreadDelete(ctx, "gone"); err != nil {
		t.Fatalf("second ThreadDelete: %v", err)
	}
	if err := s.ThreadDelete(ctx, "ghost"); err != nil {
		t.Fatalf("ThreadDelete of a thread never created: %v", err)
	}

	if got := idsOfRounds(readRange(t, s, "stays", api.NoRoundID, api.NoRoundID)); !reflect.DeepEqual(got, []api.RoundID{kept}) {
		t.Fatalf("rounds of the surviving thread: got %v, want [%d]", got, kept)
	}

	fresh := createThread(t, s, "gone")
	if fresh.Name != "" || fresh.Seq != 0 {
		t.Fatalf("thread recreated under the deleted id: %+v", fresh)
	}
	if got := readRange(t, s, "gone", api.NoRoundID, api.NoRoundID); len(got) != 0 {
		t.Fatalf("rounds under the recreated id: got %v", idsOfRounds(got))
	}
	list, err := s.RecordList(ctx, "gone")
	if err != nil || len(list) != 0 {
		t.Fatalf("records under the recreated id: got %v, err %v", idsOfHeaders(list), err)
	}
	comp, err := s.CompactionGet(ctx, "gone", api.NoRoundID)
	if err != nil || comp != api.NoCompaction {
		t.Fatalf("compaction under the recreated id: got %+v, err %v", comp, err)
	}
}

func storageThreadListName(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	a := createThread(t, s, "a")
	b := createThread(t, s, "b")
	if _, err := s.ThreadRename(ctx, "a", a.Seq, "Billing Migration"); err != nil {
		t.Fatalf("rename a: %v", err)
	}
	if _, err := s.ThreadRename(ctx, "b", b.Seq, "drawer focus trap"); err != nil {
		t.Fatalf("rename b: %v", err)
	}

	threads, err := s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "migration")
	if err != nil {
		t.Fatalf("ThreadList name: %v", err)
	}
	if want := []api.ThreadID{"a"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
		t.Fatalf("ThreadList name folds case: got %v, want %v", idsOfThreads(threads), want)
	}

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "ghost")
	if err != nil {
		t.Fatalf("ThreadList name miss: %v", err)
	}
	if len(threads) != 0 {
		t.Fatalf("ThreadList name miss: got %v", idsOfThreads(threads))
	}

	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList empty name: %v", err)
	}
	if len(threads) != 2 {
		t.Fatalf("an empty name must not filter: got %v", idsOfThreads(threads))
	}
}

func storageRoundSearch(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	createThread(t, s, "a")
	createThread(t, s, "b")
	_, seqA := addInput(t, s, "a", 0, "rewrite the billing MIGRATION")
	_, seqB := addInput(t, s, "b", 0, "unrelated input")
	round, _ := addInput(t, s, "b", seqB, "second")
	if _, err := s.RoundOk(ctx, "b", seqB+1, round, api.EventUsage{}, api.Indexed[api.Content]{
		0: api.ContentText{Text: "the migration ran twice before the checkpoint settled"},
	}); err != nil {
		t.Fatalf("RoundOk: %v", err)
	}
	_ = seqA

	hits, err := s.RoundSearch(ctx, "", false, "migration", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch: %v", err)
	}
	if len(hits) != 2 {
		t.Fatalf("RoundSearch across threads: got %d hits, %+v", len(hits), hits)
	}
	if !hits[0].Created.After(hits[1].Created) {
		t.Fatalf("RoundSearch must come newest first: %v then %v", hits[0].Created, hits[1].Created)
	}
	for _, h := range hits {
		if !strings.Contains(strings.ToLower(h.Snippet), "migration") {
			t.Fatalf("snippet must contain the match: %q", h.Snippet)
		}
	}

	hits, err = s.RoundSearch(ctx, "a", false, "migration", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch in thread: %v", err)
	}
	if len(hits) != 1 || hits[0].Thread != "a" {
		t.Fatalf("RoundSearch restricted to a: %+v", hits)
	}

	hits, err = s.RoundSearch(ctx, "a", true, "migration", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch excluding: %v", err)
	}
	if len(hits) != 1 || hits[0].Thread != "b" {
		t.Fatalf("RoundSearch excluding a: %+v", hits)
	}

	hits, err = s.RoundSearch(ctx, "", false, "migration", nil, false, false, 1)
	if err != nil {
		t.Fatalf("RoundSearch limit: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("RoundSearch limit 1: got %d", len(hits))
	}

	hits, err = s.RoundSearch(ctx, "", false, "", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch empty query: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("an empty query must match nothing: %+v", hits)
	}
}

func storageRoundSearchCursor(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	createThread(t, s, "a")
	_, seq := addInput(t, s, "a", 0, "the first migration")
	addInput(t, s, "a", seq, "the second migration")
	for _, id := range []api.ThreadID{"b", "c", "d"} {
		if _, err := s.ThreadClone(ctx, "a", id); err != nil {
			t.Fatalf("ThreadClone(%q): %v", id, err)
		}
	}
	full, err := s.RoundSearch(ctx, "", false, "migration", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch: %v", err)
	}
	if len(full) != 8 {
		t.Fatalf("RoundSearch over four clones: got %d hits, %+v", len(full), full)
	}
	if !full[0].Created.Equal(full[3].Created) || full[0].Created.Equal(full[4].Created) {
		t.Fatalf("a clone must copy created, leaving two groups of four: %+v", full)
	}
	for _, asc := range []bool{false, true} {
		want := []string{}
		for _, h := range full {
			want = append(want, fmt.Sprintf("%s/%d@%d", h.Thread, h.Round, h.Created.UnixNano()))
		}
		if asc {
			slices.Reverse(want)
		}
		got := []string{}
		var cursor *api.Cursor
		for range len(want) + 1 {
			page, err := s.RoundSearch(ctx, "", false, "migration", cursor, false, asc, 2)
			if err != nil {
				t.Fatalf("RoundSearch page (asc %v): %v", asc, err)
			}
			if len(page) == 0 {
				break
			}
			for _, h := range page {
				got = append(got, fmt.Sprintf("%s/%d@%d", h.Thread, h.Round, h.Created.UnixNano()))
			}
			last := page[len(page)-1]
			cursor = &api.Cursor{Created: last.Created, Thread: last.Thread, Round: last.Round}
		}
		if !slices.Equal(got, want) {
			t.Fatalf("paging (asc %v) must return every hit exactly once, in order: got %v, want %v", asc, got, want)
		}
	}
	at := full[1]
	cursor := &api.Cursor{Created: at.Created, Thread: at.Thread, Round: at.Round}
	hits, err := s.RoundSearch(ctx, "", false, "migration", cursor, true, false, 1)
	if err != nil {
		t.Fatalf("RoundSearch inclusive: %v", err)
	}
	if len(hits) != 1 || hits[0].Thread != at.Thread || hits[0].Round != at.Round {
		t.Fatalf("an inclusive cursor must take the round it names: %+v", hits)
	}
	hits, err = s.RoundSearch(ctx, "", false, "migration", cursor, false, false, 1)
	if err != nil {
		t.Fatalf("RoundSearch exclusive: %v", err)
	}
	if len(hits) != 1 || hits[0].Thread != full[2].Thread || hits[0].Round != full[2].Round {
		t.Fatalf("an exclusive cursor must start past the round it names: %+v", hits)
	}
}

func storageRoundReadsOmitResults(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	createThread(t, s, "a")
	round, seq := addInput(t, s, "a", 0, "hello")
	if _, err := s.RoundResult(ctx, "a", seq, round, map[string]api.ContentToolResult{
		"call": {ID: "call", Content: api.UserContents{api.ContentText{Text: "tool output"}}},
	}); err != nil {
		t.Fatalf("RoundResult: %v", err)
	}
	if got := readRound(t, s, "a", round); len(got.Results) == 0 {
		t.Fatal("RoundRange must still hydrate Results")
	}
	before, err := s.RoundBefore(ctx, "a", api.NoRoundID, 10)
	if err != nil {
		t.Fatalf("RoundBefore: %v", err)
	}
	if len(before) != 1 || len(before[0].Results) != 0 {
		t.Fatalf("RoundBefore must omit Results: %+v", before)
	}
	after, err := s.RoundAfter(ctx, "a", api.NoRoundID-1, 10)
	if err != nil {
		t.Fatalf("RoundAfter: %v", err)
	}
	if len(after) != 1 || len(after[0].Results) != 0 {
		t.Fatalf("RoundAfter must omit Results: %+v", after)
	}
}

func storageRoundSearchSnippetAfterCaseChangingRunes(t *testing.T, s api.MessageStorage) {
	createThread(t, s, "a")
	createThread(t, s, "b")
	createThread(t, s, "c")
	addInput(t, s, "a", 0, strings.Repeat("Ⱥ", 100)+"abc")
	addInput(t, s, "b", 0, strings.Repeat("İ", 1000)+"abc")
	addInput(t, s, "c", 0, strings.Repeat("\xff", 100)+"abc")
	hits, err := s.RoundSearch(context.Background(), "", false, "ABC", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch: %v", err)
	}
	if len(hits) != 3 {
		t.Fatalf("got %d hits: %+v", len(hits), hits)
	}
	for _, h := range hits {
		if !strings.HasSuffix(h.Snippet, "abc") {
			t.Fatalf("snippet in %s must end with the match: %q", h.Thread, h.Snippet)
		}
	}
}

func storageThreadListNameFoldsNonASCII(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	a := createThread(t, s, "a")
	createThread(t, s, "b")
	if _, err := s.ThreadRename(ctx, "a", a.Seq, "Отчёт за квартал"); err != nil {
		t.Fatalf("rename a: %v", err)
	}
	for _, name := range []string{"отчёт", "ОТЧЁТ", "За Квартал"} {
		threads, err := s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, name)
		if err != nil {
			t.Fatalf("ThreadList name %q: %v", name, err)
		}
		if want := []api.ThreadID{"a"}; !reflect.DeepEqual(idsOfThreads(threads), want) {
			t.Fatalf("ThreadList name %q folds case: got %v, want %v", name, idsOfThreads(threads), want)
		}
	}
}

func storageRoundSearchFoldsNonASCII(t *testing.T, s api.MessageStorage) {
	ctx := context.Background()
	createThread(t, s, "a")
	createThread(t, s, "b")
	addInput(t, s, "a", 0, "Привет мир")
	round, seq := addInput(t, s, "b", 0, "unrelated input")
	if _, err := s.RoundOk(ctx, "b", seq, round, api.EventUsage{}, api.Indexed[api.Content]{
		0: api.ContentText{Text: "Die ÜBERSICHT ist fertig"},
	}); err != nil {
		t.Fatalf("RoundOk: %v", err)
	}
	for _, c := range []struct {
		query   string
		thread  api.ThreadID
		snippet string
	}{
		{"привет", "a", "Привет"},
		{"МИР", "a", "мир"},
		{"übersicht", "b", "ÜBERSICHT"},
	} {
		hits, err := s.RoundSearch(ctx, "", false, c.query, nil, false, false, 0)
		if err != nil {
			t.Fatalf("RoundSearch(%q): %v", c.query, err)
		}
		if len(hits) != 1 || hits[0].Thread != c.thread {
			t.Fatalf("RoundSearch(%q): want one hit in %s, got %+v", c.query, c.thread, hits)
		}
		if !strings.Contains(hits[0].Snippet, c.snippet) {
			t.Fatalf("RoundSearch(%q): snippet %q must contain %q", c.query, hits[0].Snippet, c.snippet)
		}
	}
}
