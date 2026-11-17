package sqlite_test

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	_ "modernc.org/sqlite"
	"neui.dev/api"
	"neui.dev/internal/testkit"
	"neui.dev/nekit/internal/sqlite"
)

func openDB(t *testing.T, path string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func openStore(t *testing.T) *sqlite.Store {
	t.Helper()
	store, err := sqlite.New(openDB(t, filepath.Join(t.TempDir(), "neui.db")))
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return store
}

func TestMessageStorage(t *testing.T) {
	testkit.MessageStorage(t, func(t *testing.T) api.MessageStorage { return openStore(t).User("user") })
}

func TestMessageStorageUsers(t *testing.T) {
	testkit.MessageStorageUsers(t, func(t *testing.T) func(string) api.MessageStorage { return openStore(t).User })
}

func TestSchemaVersion(t *testing.T) {
	ctx := context.Background()
	db := openDB(t, filepath.Join(t.TempDir(), "neui.db"))
	if _, err := sqlite.New(db); err != nil {
		t.Fatalf("New: %v", err)
	}
	var version int
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != 1 {
		t.Fatalf("user_version after New: got %d, want 1", version)
	}
	var mode string
	if err := db.QueryRowContext(ctx, "PRAGMA journal_mode").Scan(&mode); err != nil {
		t.Fatalf("journal_mode: %v", err)
	}
	if mode != "wal" {
		t.Fatalf("journal_mode after New: got %q, want wal", mode)
	}
	store, err := sqlite.New(db)
	if err != nil {
		t.Fatalf("New on a current database: %v", err)
	}
	if _, err := store.User("u").ThreadCreate(ctx, "a"); err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}

	if _, err := db.ExecContext(ctx, "PRAGMA user_version = 2"); err != nil {
		t.Fatalf("set user_version: %v", err)
	}
	if _, err := sqlite.New(db); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Fatalf("New on a newer database: want a refusal, got %v", err)
	}
	if err := db.QueryRowContext(ctx, "PRAGMA user_version").Scan(&version); err != nil {
		t.Fatalf("user_version: %v", err)
	}
	if version != 2 {
		t.Fatalf("a refused New changed user_version to %d", version)
	}
}

func TestPersistence(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "persist.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatalf("sql.Open: %v", err)
	}
	store, err := sqlite.New(db)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	s := store.User("user")

	th, err := s.ThreadCreate(ctx, "a")
	if err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	r0, seq := addInput(t, s, "a", th.Seq, "zero")
	r1, seq := addInput(t, s, "a", seq, "one")
	r2, seq := addInput(t, s, "a", seq, "two")

	usage := api.EventUsage{Input: 10, Output: 3, Reasoning: 2, CacheRead: 7, CacheWrite: 5}
	response := api.Indexed[api.Content]{
		0: api.ContentReasoning{Model: "m", Summary: api.Indexed[string]{0: "s"}, Thoughts: api.Indexed[string]{1: "t"}, Meta: json.RawMessage(`{"sig":"x"}`)},
		1: api.ContentText{Text: "calling"},
		3: api.ContentToolUse{ID: "call-1", Name: "tool", Input: json.RawMessage(`{"k":1}`)},
	}
	if seq, err = s.RoundOk(ctx, "a", seq, r0, usage, response); err != nil {
		t.Fatalf("RoundOk: %v", err)
	}
	results := map[string]api.ContentToolResult{
		"call-1": {ID: "call-1", Content: api.UserContents{api.ContentText{Text: "out"}}, Error: api.ToolErrRuntime},
		"call-2": {ID: "call-2", Content: api.UserContents{api.ContentImage{Data: []byte{1, 2, 3}, Mime: "image/png"}}},
	}
	if seq, err = s.RoundResult(ctx, "a", seq, r1, results); err != nil {
		t.Fatalf("RoundResult: %v", err)
	}
	if seq, err = s.RoundError(ctx, "a", seq, r1, "boom", api.EventUsage{}, nil); err != nil {
		t.Fatalf("RoundError: %v", err)
	}
	if seq, err = s.CompactionCreate(ctx, "a", seq, api.Compaction{Target: r0, Tail: r1, Text: "summary", Output: 12, Base: 300}); err != nil {
		t.Fatalf("CompactionCreate: %v", err)
	}
	if seq, err = s.ThreadRename(ctx, "a", seq, "Отчёт"); err != nil {
		t.Fatalf("ThreadRename: %v", err)
	}

	attrs := map[string]any{"path": "x.md"}
	recText, seq, err := s.RecordAdd(ctx, "a", seq, r0, "file", attrs, "the payload")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	_, seq, err = s.RecordAdd(ctx, "a", seq, r1, "rows", nil, map[string]any{"n": 1})
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	recGone, seq, err := s.RecordAdd(ctx, "a", seq, r2, "gone", nil, "gone")
	if err != nil {
		t.Fatalf("RecordAdd: %v", err)
	}
	if seq, err = s.RecordPin(ctx, "a", seq, r1, []api.RecordID{recText}); err != nil {
		t.Fatalf("RecordPin: %v", err)
	}
	if seq, err = s.ThreadCut(ctx, "a", seq, r2); err != nil {
		t.Fatalf("ThreadCut: %v", err)
	}

	other, err := s.ThreadCreate(ctx, "other")
	if err != nil {
		t.Fatalf("ThreadCreate: %v", err)
	}
	before, err := s.ThreadGet(ctx, "a")
	if err != nil {
		t.Fatalf("ThreadGet: %v", err)
	}
	if err := db.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	store, err = sqlite.New(openDB(t, path))
	if err != nil {
		t.Fatalf("New after reopen: %v", err)
	}
	s = store.User("user")

	after, err := s.ThreadGet(ctx, "a")
	if err != nil {
		t.Fatalf("ThreadGet after reopen: %v", err)
	}
	if after.ID != before.ID || after.Name != "Отчёт" || after.Seq != seq || !after.Created.Equal(before.Created) || !after.Updated.Equal(before.Updated) {
		t.Fatalf("thread after reopen: got %+v, want %+v at seq %d", after, before, seq)
	}
	threads, err := s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "")
	if err != nil {
		t.Fatalf("ThreadList after reopen: %v", err)
	}
	if got, want := threadIDs(threads), []api.ThreadID{other.ID, "a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ThreadList after reopen: got %v, want %v", got, want)
	}
	threads, err = s.ThreadList(ctx, time.Time{}, time.Time{}, false, 0, "ОТЧЁТ")
	if err != nil {
		t.Fatalf("ThreadList name after reopen: %v", err)
	}
	if got, want := threadIDs(threads), []api.ThreadID{"a"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("ThreadList name after reopen: got %v, want %v", got, want)
	}

	var rounds []api.Round
	for r, err := range s.RoundRange(ctx, "a", api.NoRoundID, api.NoRoundID) {
		if err != nil {
			t.Fatalf("RoundRange after reopen: %v", err)
		}
		rounds = append(rounds, r)
	}
	if len(rounds) != 2 || rounds[0].ID != r0 || rounds[1].ID != r1 {
		t.Fatalf("rounds after reopen: got %+v, want [%d %d]", rounds, r0, r1)
	}
	if got := rounds[0]; got.State != api.RoundOk || got.Usage != usage || got.Input.Text != "zero" || !reflect.DeepEqual(got.Response, response) {
		t.Fatalf("round %d after reopen: %+v", r0, got)
	}
	if got := rounds[1]; got.State != api.RoundError || got.Error != "boom" || !reflect.DeepEqual(got.Results, results) {
		t.Fatalf("round %d after reopen: %+v", r1, got)
	}
	if got := rounds[1].PinnedRecords; len(got) != 1 || got[0].ID != recText || got[0].Kind != "file" || !reflect.DeepEqual(got[0].Attrs, attrs) {
		t.Fatalf("hydration after reopen: %+v", got)
	}
	if data, ok := rounds[1].PinnedRecords[0].Data.(string); !ok || data != "the payload" {
		t.Fatalf("hydrated record data after reopen: %+v", rounds[1].PinnedRecords[0].Data)
	}
	hits, err := s.RoundSearch(ctx, "", false, "CALLING", nil, false, false, 0)
	if err != nil {
		t.Fatalf("RoundSearch after reopen: %v", err)
	}
	if len(hits) != 1 || hits[0].Thread != "a" || hits[0].Round != r0 || hits[0].Snippet != "calling" {
		t.Fatalf("RoundSearch after reopen: %+v", hits)
	}

	comp, err := s.CompactionGet(ctx, "a", api.NoRoundID)
	if err != nil {
		t.Fatalf("CompactionGet after reopen: %v", err)
	}
	if want := (api.Compaction{Text: "summary", Target: r0, Tail: r1, Output: 12, Base: 300}); comp != want {
		t.Fatalf("compaction after reopen: got %+v, want %+v", comp, want)
	}

	if _, _, err := s.RoundInput(ctx, "a", seq-1, api.Input{Text: "stale"}); !errors.Is(err, api.ErrDrift) {
		t.Fatalf("stale write after reopen: want ErrDrift, got %v", err)
	}
	rNew, seq := addInput(t, s, "a", seq, "after reopen")
	if rNew <= r2 {
		t.Fatalf("round id after reopen: got %d, want above %d", rNew, r2)
	}
	recNew, _, err := s.RecordAdd(ctx, "a", seq, rNew, "note", nil, "x")
	if err != nil {
		t.Fatalf("RecordAdd after reopen: %v", err)
	}
	if recNew <= recGone {
		t.Fatalf("record id after reopen: got %d, want above %d", recNew, recGone)
	}
}

func TestConcurrentWrites(t *testing.T) {
	ctx := context.Background()
	store := openStore(t)
	var wg sync.WaitGroup
	errs := make(chan error, 16)
	for w := range 16 {
		wg.Go(func() {
			s := store.User(fmt.Sprintf("user%d", w%4))
			id := api.ThreadID(fmt.Sprintf("thread%d", w))
			th, err := s.ThreadCreate(ctx, id)
			if err != nil {
				errs <- err
				return
			}
			seq := th.Seq
			for i := range 25 {
				var round api.RoundID
				if round, seq, err = s.RoundInput(ctx, id, seq, api.Input{Text: fmt.Sprintf("input %d", i)}); err != nil {
					errs <- err
					return
				}
				if seq, err = s.RoundOk(ctx, id, seq, round, api.EventUsage{}, api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}); err != nil {
					errs <- err
					return
				}
				if _, err := s.RoundBefore(ctx, id, api.NoRoundID, 5); err != nil {
					errs <- err
					return
				}
			}
		})
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent write: %v", err)
	}
	for w := range 16 {
		s := store.User(fmt.Sprintf("user%d", w%4))
		rounds, err := s.RoundBefore(ctx, api.ThreadID(fmt.Sprintf("thread%d", w)), api.NoRoundID, 100)
		if err != nil {
			t.Fatalf("RoundBefore: %v", err)
		}
		if len(rounds) != 25 {
			t.Fatalf("thread%d: got %d rounds, want 25", w, len(rounds))
		}
	}
}

func addInput(t *testing.T, s api.MessageStorage, thread api.ThreadID, seq api.Seq, text string) (api.RoundID, api.Seq) {
	t.Helper()
	id, next, err := s.RoundInput(context.Background(), thread, seq, api.Input{Text: text})
	if err != nil {
		t.Fatalf("RoundInput(%q, seq %d, %q): %v", thread, seq, text, err)
	}
	return id, next
}

func threadIDs(threads []api.Thread) []api.ThreadID {
	ids := make([]api.ThreadID, len(threads))
	for i, th := range threads {
		ids[i] = th.ID
	}
	return ids
}
