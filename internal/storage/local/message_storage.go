package local

import (
	"cmp"
	"context"
	"iter"
	"maps"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"neui.dev/api"
)

const (
	snippetLead  = 40
	snippetTrail = 120
)

func NewMessageStorage() api.MessageStorage {
	return &MessageStorage{threads: map[api.ThreadID]*localThread{}}
}

type MessageStorage struct {
	mu      sync.Mutex
	threads map[api.ThreadID]*localThread
}

type localThread struct {
	thread      api.Thread
	rounds      []api.Round
	compactions []api.Compaction
	records     []api.Record
	nextRound   api.RoundID
	nextRecord  api.RecordID
}

func (l *MessageStorage) ThreadList(ctx context.Context, from, to time.Time, asc bool, limit int, name string) ([]api.Thread, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fold := strings.ToLower(name)
	out := make([]api.Thread, 0, len(l.threads))
	for _, t := range l.threads {
		if !from.IsZero() && t.thread.Updated.Before(from) {
			continue
		}
		if !to.IsZero() && !t.thread.Updated.Before(to) {
			continue
		}
		if fold != "" && !strings.Contains(strings.ToLower(t.thread.Name), fold) {
			continue
		}
		out = append(out, t.thread)
	}
	slices.SortFunc(out, func(a, b api.Thread) int {
		if asc {
			a, b = b, a
		}
		return cmp.Or(b.Updated.Compare(a.Updated), cmp.Compare(b.ID, a.ID))
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (l *MessageStorage) ThreadBump(ctx context.Context, thread api.ThreadID) (api.Thread, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return api.Thread{}, api.ErrNotFound
	}
	t.thread.Seq++
	return t.thread, nil
}

func (l *MessageStorage) ThreadGet(ctx context.Context, thread api.ThreadID) (api.Thread, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return api.Thread{}, api.ErrNotFound
	}
	return t.thread, nil
}

func (l *MessageStorage) ThreadCreate(ctx context.Context, id api.ThreadID) (api.Thread, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.threads[id]; ok {
		return api.Thread{}, api.ErrExists
	}
	now := time.Now()
	t := &localThread{thread: api.Thread{ID: id, Created: now, Updated: now}}
	l.threads[id] = t
	return t.thread, nil
}

func (l *MessageStorage) ThreadCut(ctx context.Context, id api.ThreadID, seq api.Seq, round api.RoundID) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[id]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	cut, _ := slices.BinarySearchFunc(t.rounds, round, byRoundID)
	t.rounds = t.rounds[:cut]
	t.compactions = slices.DeleteFunc(t.compactions, func(c api.Compaction) bool {
		return c.Tail >= round
	})
	t.records = slices.DeleteFunc(t.records, func(r api.Record) bool {
		return r.Origin >= round
	})
	for i := range t.records {
		if t.records[i].Pin >= round {
			t.records[i].Pin = api.NoRoundID
		}
	}
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) ThreadRename(ctx context.Context, id api.ThreadID, seq api.Seq, name string) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[id]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	t.thread.Name = name
	t.thread.Seq++
	return t.thread.Seq, nil
}

func (l *MessageStorage) ThreadClone(ctx context.Context, source api.ThreadID, target api.ThreadID) (api.Thread, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.threads[target]; ok {
		return api.Thread{}, api.ErrExists
	}
	s, ok := l.threads[source]
	if !ok {
		return api.Thread{}, api.ErrNotFound
	}
	rounds := slices.Clone(s.rounds)
	for i := range rounds {
		rounds[i].Thread = target
	}
	now := time.Now()
	t := &localThread{
		thread: api.Thread{
			ID:      target,
			Name:    s.thread.Name,
			Created: now,
			Updated: now,
		},
		rounds:      rounds,
		compactions: slices.Clone(s.compactions),
		records:     slices.Clone(s.records),
		nextRound:   s.nextRound,
		nextRecord:  s.nextRecord,
	}
	l.threads[target] = t
	return t.thread, nil
}

func (l *MessageStorage) ThreadDelete(ctx context.Context, id api.ThreadID) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.threads, id)
	return nil
}

func (l *MessageStorage) CompactionCreate(ctx context.Context, thread api.ThreadID, seq api.Seq, compaction api.Compaction) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	t.compactions = append(t.compactions, compaction)
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) CompactionGet(ctx context.Context, thread api.ThreadID, beforeTail api.RoundID) (api.Compaction, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return api.NoCompaction, api.ErrNotFound
	}
	for i := len(t.compactions) - 1; i >= 0; i-- {
		if beforeTail == api.NoRoundID || t.compactions[i].Tail < beforeTail {
			return t.compactions[i], nil
		}
	}
	return api.NoCompaction, nil
}

func (l *MessageStorage) RoundRange(ctx context.Context, thread api.ThreadID, start api.RoundID, end api.RoundID) iter.Seq2[api.Round, error] {
	return func(yield func(api.Round, error) bool) {
		l.mu.Lock()
		t, ok := l.threads[thread]
		if !ok {
			l.mu.Unlock()
			yield(api.Round{}, api.ErrNotFound)
			return
		}
		lo, _ := slices.BinarySearchFunc(t.rounds, start, byRoundID)
		hi := len(t.rounds)
		if end != api.NoRoundID {
			hi, _ = slices.BinarySearchFunc(t.rounds, end+1, byRoundID)
		}
		if hi < lo {
			hi = lo
		}
		rounds := t.hydrate(t.rounds[lo:hi], true)
		l.mu.Unlock()
		for _, r := range rounds {
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (l *MessageStorage) RoundBefore(ctx context.Context, thread api.ThreadID, start api.RoundID, limit int) ([]api.Round, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return nil, api.ErrNotFound
	}
	hi := len(t.rounds)
	if start != api.NoRoundID {
		hi, _ = slices.BinarySearchFunc(t.rounds, start, byRoundID)
	}
	lo := max(hi-max(limit, 0), 0)
	return t.hydrate(t.rounds[lo:hi], false), nil
}

func (l *MessageStorage) RoundAfter(ctx context.Context, thread api.ThreadID, start api.RoundID, limit int) ([]api.Round, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return nil, api.ErrNotFound
	}
	lo, _ := slices.BinarySearchFunc(t.rounds, start+1, byRoundID)
	hi := min(lo+max(limit, 0), len(t.rounds))
	return t.hydrate(t.rounds[lo:hi], false), nil
}

func (l *MessageStorage) RoundSearch(ctx context.Context, thread api.ThreadID, exclude bool, query string, from *api.Cursor, inclusive bool, asc bool, limit int) ([]api.Hit, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	fold := strings.ToLower(query)
	if fold == "" {
		return nil, nil
	}
	out := []api.Hit{}
	for id, t := range l.threads {
		if thread != "" && (id == thread) == exclude {
			continue
		}
		for _, r := range t.rounds {
			snippet, ok := matchSnippet(r.Input.Text, fold)
			if !ok {
				for _, i := range slices.Sorted(maps.Keys(r.Response)) {
					text, is := r.Response[i].(api.ContentText)
					if !is {
						continue
					}
					if snippet, ok = matchSnippet(text.Text, fold); ok {
						break
					}
				}
			}
			if !ok {
				continue
			}
			out = append(out, api.Hit{
				Thread:  id,
				Round:   r.ID,
				Created: r.Created,
				Snippet: snippet,
			})
		}
	}
	slices.SortFunc(out, func(a, b api.Hit) int {
		if asc {
			a, b = b, a
		}
		return cmp.Or(b.Created.Compare(a.Created), cmp.Compare(b.Thread, a.Thread), cmp.Compare(b.Round, a.Round))
	})
	if from != nil {
		start := 0
		for start < len(out) {
			order := cmp.Or(out[start].Created.Compare(from.Created), cmp.Compare(out[start].Thread, from.Thread), cmp.Compare(out[start].Round, from.Round))
			if !asc {
				order = -order
			}
			if order > 0 || (order == 0 && inclusive) {
				break
			}
			start++
		}
		out = out[start:]
	}
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func matchSnippet(text string, fold string) (string, bool) {
	i := strings.Index(strings.ToLower(text), fold)
	if i == -1 {
		return "", false
	}
	start, end := 0, len(text)
	lowered := 0
	for at, r := range text {
		if lowered == i {
			start = at
		}
		if lowered == i+len(fold) {
			end = at
			break
		}
		lowered += utf8.RuneLen(unicode.ToLower(r))
	}
	lo := max(start-snippetLead, 0)
	for lo > 0 && !utf8.RuneStart(text[lo]) {
		lo--
	}
	hi := min(end+snippetTrail, len(text))
	for hi < len(text) && !utf8.RuneStart(text[hi]) {
		hi++
	}
	return text[lo:hi], true
}

func (l *MessageStorage) RoundInput(ctx context.Context, thread api.ThreadID, seq api.Seq, input api.Input) (api.RoundID, api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return api.NoRoundID, 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return api.NoRoundID, 0, api.ErrDrift
	}
	id := t.nextRound
	t.nextRound++
	t.rounds = append(t.rounds, api.Round{
		ID:      id,
		Thread:  thread,
		State:   api.RoundInput,
		Created: time.Now(),
		Input:   input,
	})
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return id, t.thread.Seq, nil
}

func (l *MessageStorage) RoundOk(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	i, ok := slices.BinarySearchFunc(t.rounds, round, byRoundID)
	if !ok {
		return 0, api.ErrNotFound
	}
	t.rounds[i].State = api.RoundOk
	t.rounds[i].Usage = usage
	t.rounds[i].Response = content
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) RoundRefuse(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	i, ok := slices.BinarySearchFunc(t.rounds, round, byRoundID)
	if !ok {
		return 0, api.ErrNotFound
	}
	t.rounds[i].State = api.RoundRefuse
	t.rounds[i].Usage = usage
	t.rounds[i].Response = content
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) RoundError(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, err string, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	i, ok := slices.BinarySearchFunc(t.rounds, round, byRoundID)
	if !ok {
		return 0, api.ErrNotFound
	}
	t.rounds[i].State = api.RoundError
	t.rounds[i].Error = err
	t.rounds[i].Usage = usage
	t.rounds[i].Response = content
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) RoundResult(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, result map[string]api.ContentToolResult) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	i, ok := slices.BinarySearchFunc(t.rounds, round, byRoundID)
	if !ok {
		return 0, api.ErrNotFound
	}
	t.rounds[i].Results = result
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) RoundAbort(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	i, ok := slices.BinarySearchFunc(t.rounds, round, byRoundID)
	if !ok {
		return 0, api.ErrNotFound
	}
	t.rounds[i].State = api.RoundAbort
	t.rounds[i].Usage = usage
	t.rounds[i].Response = content
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) RecordAdd(ctx context.Context, thread api.ThreadID, seq api.Seq, origin api.RoundID, kind string, attrs map[string]any, data any) (api.RecordID, api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, 0, api.ErrDrift
	}
	id := t.nextRecord
	t.nextRecord++
	t.records = append(t.records, api.Record{
		ID:     id,
		Origin: origin,
		Pin:    api.NoRoundID,
		Kind:   kind,
		Attrs:  attrs,
		Data:   data,
	})
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return id, t.thread.Seq, nil
}

func (l *MessageStorage) RecordPin(ctx context.Context, thread api.ThreadID, seq api.Seq, pin api.RoundID, records []api.RecordID) (api.Seq, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return 0, api.ErrNotFound
	}
	if t.thread.Seq != seq {
		return 0, api.ErrDrift
	}
	idx := make([]int, len(records))
	for j, id := range records {
		i, ok := slices.BinarySearchFunc(t.records, id, byRecordID)
		if !ok {
			return 0, api.ErrNotFound
		}
		idx[j] = i
	}
	for _, i := range idx {
		t.records[i].Pin = pin
	}
	t.thread.Seq++
	t.thread.Updated = time.Now()
	return t.thread.Seq, nil
}

func (l *MessageStorage) RecordList(ctx context.Context, thread api.ThreadID) ([]api.Record, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	t, ok := l.threads[thread]
	if !ok {
		return nil, api.ErrNotFound
	}
	var out []api.Record
	for _, r := range t.records {
		if r.Pin != api.NoRoundID {
			continue
		}
		out = append(out, r)
	}
	return out, nil
}

func (t *localThread) hydrate(rounds []api.Round, results bool) []api.Round {
	out := slices.Clone(rounds)
	pins := map[api.RoundID][]api.Record{}
	for _, r := range t.records {
		if r.Pin == api.NoRoundID {
			continue
		}
		if !results {
			r.Data = nil
		}
		pins[r.Pin] = append(pins[r.Pin], r)
	}
	for i := range out {
		out[i].PinnedRecords = pins[out[i].ID]
		if !results {
			out[i].Results = nil
		}
	}
	return out
}

func byRoundID(r api.Round, id api.RoundID) int { return cmp.Compare(r.ID, id) }

func byRecordID(r api.Record, id api.RecordID) int { return cmp.Compare(r.ID, id) }
