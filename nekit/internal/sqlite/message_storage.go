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

package sqlite

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"iter"
	"maps"
	"slices"
	"time"

	"neui.dev/api"
	"neui.dev/internal/storage/match"
)

const threadColumns = "id, name, seq, created, updated"

type MessageStorage struct {
	db   *sql.DB
	user string
}

type storedResponse struct {
	Keys    []int        `json:"keys"`
	Content api.Contents `json:"content"`
}

type storedResult struct {
	Content api.UserContents `json:"content"`
	Error   api.ToolError    `json:"error"`
}

type rowScanner interface {
	Scan(dest ...any) error
}

func (m *MessageStorage) ThreadList(ctx context.Context, from, to time.Time, asc bool, limit int, name string) ([]api.Thread, error) {
	query := "SELECT " + threadColumns + " FROM threads WHERE user = ?"
	args := []any{m.user}
	if !from.IsZero() {
		query += " AND updated >= ?"
		args = append(args, from.UnixNano())
	}
	if !to.IsZero() {
		query += " AND updated < ?"
		args = append(args, to.UnixNano())
	}
	if name != "" {
		query += " AND instr(name_fold, ?) > 0"
		args = append(args, match.Fold(name))
	}
	if asc {
		query += " ORDER BY updated ASC, id ASC LIMIT ?"
	} else {
		query += " ORDER BY updated DESC, id DESC LIMIT ?"
	}
	if limit <= 0 {
		limit = -1
	}
	args = append(args, limit)
	rows, err := m.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Thread{}
	for rows.Next() {
		t, err := scanThread(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (m *MessageStorage) ThreadBump(ctx context.Context, thread api.ThreadID) (api.Thread, error) {
	var t api.Thread
	err := write(ctx, m.db, func(conn *sql.Conn) error {
		var err error
		t, err = scanThread(conn.QueryRowContext(ctx,
			"UPDATE threads SET seq = seq + 1 WHERE user = ? AND id = ? RETURNING "+threadColumns,
			m.user, thread))
		return err
	})
	return t, err
}

func (m *MessageStorage) ThreadGet(ctx context.Context, thread api.ThreadID) (api.Thread, error) {
	return scanThread(m.db.QueryRowContext(ctx,
		"SELECT "+threadColumns+" FROM threads WHERE user = ? AND id = ?",
		m.user, thread))
}

func (m *MessageStorage) ThreadCreate(ctx context.Context, id api.ThreadID) (api.Thread, error) {
	var t api.Thread
	err := write(ctx, m.db, func(conn *sql.Conn) error {
		now := time.Now().UnixNano()
		var err error
		t, err = scanThread(conn.QueryRowContext(ctx,
			"INSERT INTO threads (user, id, created, updated) VALUES (?, ?, ?, ?) ON CONFLICT (id) DO NOTHING RETURNING "+threadColumns,
			m.user, id, now, now))
		if errors.Is(err, api.ErrNotFound) {
			return api.ErrExists
		}
		return err
	})
	return t, err
}

func (m *MessageStorage) ThreadCut(ctx context.Context, id api.ThreadID, seq api.Seq, round api.RoundID) (api.Seq, error) {
	return m.guarded(ctx, id, seq, true, func(conn *sql.Conn) error {
		for _, stmt := range []string{
			"DELETE FROM rounds WHERE user = ? AND thread = ? AND id >= ?",
			"DELETE FROM compactions WHERE user = ? AND thread = ? AND tail >= ?",
			"DELETE FROM records WHERE user = ? AND thread = ? AND origin >= ?",
			"UPDATE records SET pin = -1 WHERE user = ? AND thread = ? AND pin >= ?",
		} {
			if _, err := conn.ExecContext(ctx, stmt, m.user, id, round); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *MessageStorage) ThreadRename(ctx context.Context, id api.ThreadID, seq api.Seq, name string) (api.Seq, error) {
	return m.guarded(ctx, id, seq, false, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx,
			"UPDATE threads SET name = ?, name_fold = ? WHERE user = ? AND id = ?",
			name, match.Fold(name), m.user, id)
		return err
	})
}

func (m *MessageStorage) ThreadClone(ctx context.Context, source api.ThreadID, target api.ThreadID) (api.Thread, error) {
	var t api.Thread
	err := write(ctx, m.db, func(conn *sql.Conn) error {
		var taken bool
		if err := conn.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM threads WHERE id = ?)", target).Scan(&taken); err != nil {
			return err
		}
		if taken {
			return api.ErrExists
		}
		now := time.Now().UnixNano()
		var err error
		t, err = scanThread(conn.QueryRowContext(ctx,
			`INSERT INTO threads (user, id, name, name_fold, created, updated, next_round, next_record)
			SELECT user, ?, name, name_fold, ?, ?, next_round, next_record FROM threads WHERE user = ? AND id = ?
			RETURNING `+threadColumns,
			target, now, now, m.user, source))
		if err != nil {
			return err
		}
		for _, stmt := range []string{
			`INSERT INTO rounds (user, thread, id, created, state, input, error, usage, response, results, input_fold, response_fold)
			SELECT user, ?, id, created, state, input, error, usage, response, results, input_fold, response_fold FROM rounds WHERE user = ? AND thread = ?`,
			`INSERT INTO compactions (user, thread, target, tail, text, output, base)
			SELECT user, ?, target, tail, text, output, base FROM compactions WHERE user = ? AND thread = ? ORDER BY n`,
			`INSERT INTO records (user, thread, id, origin, pin, kind, attrs, data, data_string)
			SELECT user, ?, id, origin, pin, kind, attrs, data, data_string FROM records WHERE user = ? AND thread = ?`,
		} {
			if _, err := conn.ExecContext(ctx, stmt, target, m.user, source); err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return api.Thread{}, err
	}
	return t, nil
}

func (m *MessageStorage) ThreadDelete(ctx context.Context, id api.ThreadID) error {
	return write(ctx, m.db, func(conn *sql.Conn) error {
		for _, stmt := range []string{
			"DELETE FROM rounds WHERE user = ? AND thread = ?",
			"DELETE FROM compactions WHERE user = ? AND thread = ?",
			"DELETE FROM records WHERE user = ? AND thread = ?",
			"DELETE FROM threads WHERE user = ? AND id = ?",
		} {
			if _, err := conn.ExecContext(ctx, stmt, m.user, id); err != nil {
				return err
			}
		}
		return nil
	})
}

func (m *MessageStorage) CompactionCreate(ctx context.Context, thread api.ThreadID, seq api.Seq, compaction api.Compaction) (api.Seq, error) {
	return m.guarded(ctx, thread, seq, true, func(conn *sql.Conn) error {
		_, err := conn.ExecContext(ctx,
			"INSERT INTO compactions (user, thread, target, tail, text, output, base) VALUES (?, ?, ?, ?, ?, ?, ?)",
			m.user, thread, compaction.Target, compaction.Tail, compaction.Text, compaction.Output, compaction.Base)
		return err
	})
}

func (m *MessageStorage) CompactionGet(ctx context.Context, thread api.ThreadID, beforeTail api.RoundID) (api.Compaction, error) {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return api.NoCompaction, err
	}
	defer tx.Rollback()
	if err := m.threadExists(ctx, tx, thread); err != nil {
		return api.NoCompaction, err
	}
	var c api.Compaction
	err = tx.QueryRowContext(ctx,
		"SELECT text, target, tail, output, base FROM compactions WHERE user = ? AND thread = ? AND (? = -1 OR tail < ?) ORDER BY n DESC LIMIT 1",
		m.user, thread, beforeTail, beforeTail).Scan(&c.Text, &c.Target, &c.Tail, &c.Output, &c.Base)
	if errors.Is(err, sql.ErrNoRows) {
		return api.NoCompaction, nil
	}
	if err != nil {
		return api.NoCompaction, err
	}
	return c, nil
}

func (m *MessageStorage) RoundRange(ctx context.Context, thread api.ThreadID, start api.RoundID, end api.RoundID) iter.Seq2[api.Round, error] {
	return func(yield func(api.Round, error) bool) {
		rounds, err := m.loadRounds(ctx, thread, true, "AND id >= ? AND (? = -1 OR id <= ?)", start, end, end)
		if err != nil {
			yield(api.Round{}, err)
			return
		}
		for _, r := range rounds {
			if !yield(r, nil) {
				return
			}
		}
	}
}

func (m *MessageStorage) RoundBefore(ctx context.Context, thread api.ThreadID, start api.RoundID, limit int) ([]api.Round, error) {
	return m.loadRounds(ctx, thread, false, "AND (? = -1 OR id < ?) ORDER BY id DESC LIMIT ?", start, start, max(limit, 0))
}

func (m *MessageStorage) RoundAfter(ctx context.Context, thread api.ThreadID, start api.RoundID, limit int) ([]api.Round, error) {
	return m.loadRounds(ctx, thread, false, "AND id > ? ORDER BY id LIMIT ?", start, max(limit, 0))
}

func (m *MessageStorage) RoundSearch(ctx context.Context, thread api.ThreadID, exclude bool, query string, from *api.Cursor, inclusive bool, asc bool, limit int) ([]api.Hit, error) {
	fold := match.Fold(query)
	if fold == "" {
		return nil, nil
	}
	stmt := "SELECT thread, id, created, input, response FROM rounds WHERE user = ?"
	args := []any{m.user}
	if thread != "" {
		if exclude {
			stmt += " AND thread <> ?"
		} else {
			stmt += " AND thread = ?"
		}
		args = append(args, thread)
	}
	if from != nil {
		op := "<"
		if asc {
			op = ">"
		}
		if inclusive {
			op += "="
		}
		stmt += " AND (created, thread, id) " + op + " (?, ?, ?)"
		args = append(args, from.Created.UnixNano(), from.Thread, from.Round)
	}
	stmt += " AND (instr(input_fold, ?) > 0 OR instr(response_fold, ?) > 0)"
	args = append(args, fold, fold)
	if asc {
		stmt += " ORDER BY created ASC, thread ASC, id ASC"
	} else {
		stmt += " ORDER BY created DESC, thread DESC, id DESC"
	}
	rows, err := m.db.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []api.Hit{}
	for rows.Next() {
		if limit > 0 && len(out) == limit {
			break
		}
		var hit api.Hit
		var created int64
		var input string
		var response sql.NullString
		if err := rows.Scan(&hit.Thread, &hit.Round, &created, &input, &response); err != nil {
			return nil, err
		}
		var in api.Input
		if err := json.Unmarshal([]byte(input), &in); err != nil {
			return nil, err
		}
		content, err := decodeResponse(response)
		if err != nil {
			return nil, err
		}
		snippet, ok := match.Round(in, content, fold)
		if !ok {
			continue
		}
		hit.Created = time.Unix(0, created)
		hit.Snippet = snippet
		out = append(out, hit)
	}
	return out, rows.Err()
}

func (m *MessageStorage) RoundInput(ctx context.Context, thread api.ThreadID, seq api.Seq, input api.Input) (api.RoundID, api.Seq, error) {
	raw, err := json.Marshal(input)
	if err != nil {
		return api.NoRoundID, 0, err
	}
	var id api.RoundID
	next, err := m.guarded(ctx, thread, seq, true, func(conn *sql.Conn) error {
		if err := conn.QueryRowContext(ctx,
			"UPDATE threads SET next_round = next_round + 1 WHERE user = ? AND id = ? RETURNING next_round - 1",
			m.user, thread).Scan(&id); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx,
			"INSERT INTO rounds (user, thread, id, created, input, input_fold) VALUES (?, ?, ?, ?, ?, ?)",
			m.user, thread, id, time.Now().UnixNano(), string(raw), match.Fold(input.Text))
		return err
	})
	if err != nil {
		return api.NoRoundID, 0, err
	}
	return id, next, nil
}

func (m *MessageStorage) RoundOk(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	rawUsage, err := json.Marshal(usage)
	if err != nil {
		return 0, err
	}
	response, err := encodeResponse(content)
	if err != nil {
		return 0, err
	}
	return m.updateRound(ctx, thread, seq, round,
		"state = ?, usage = ?, response = ?, response_fold = ?",
		string(api.RoundOk), string(rawUsage), response, match.FoldResponse(content))
}

func (m *MessageStorage) RoundRefuse(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	rawUsage, err := json.Marshal(usage)
	if err != nil {
		return 0, err
	}
	response, err := encodeResponse(content)
	if err != nil {
		return 0, err
	}
	return m.updateRound(ctx, thread, seq, round,
		"state = ?, usage = ?, response = ?, response_fold = ?",
		string(api.RoundRefuse), string(rawUsage), response, match.FoldResponse(content))
}

func (m *MessageStorage) RoundError(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, errText string, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	rawUsage, err := json.Marshal(usage)
	if err != nil {
		return 0, err
	}
	response, err := encodeResponse(content)
	if err != nil {
		return 0, err
	}
	return m.updateRound(ctx, thread, seq, round,
		"state = ?, error = ?, usage = ?, response = ?, response_fold = ?",
		string(api.RoundError), errText, string(rawUsage), response, match.FoldResponse(content))
}

func (m *MessageStorage) RoundResult(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, result map[string]api.ContentToolResult) (api.Seq, error) {
	results, err := encodeResults(result)
	if err != nil {
		return 0, err
	}
	return m.updateRound(ctx, thread, seq, round, "results = ?", results)
}

func (m *MessageStorage) RoundAbort(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, usage api.EventUsage, content api.Indexed[api.Content]) (api.Seq, error) {
	rawUsage, err := json.Marshal(usage)
	if err != nil {
		return 0, err
	}
	response, err := encodeResponse(content)
	if err != nil {
		return 0, err
	}
	return m.updateRound(ctx, thread, seq, round,
		"state = ?, usage = ?, response = ?, response_fold = ?",
		string(api.RoundAbort), string(rawUsage), response, match.FoldResponse(content))
}

func (m *MessageStorage) RecordAdd(ctx context.Context, thread api.ThreadID, seq api.Seq, origin api.RoundID, kind string, attrs map[string]any, data any) (api.RecordID, api.Seq, error) {
	var rawAttrs any
	if attrs != nil {
		raw, err := json.Marshal(attrs)
		if err != nil {
			return 0, 0, err
		}
		rawAttrs = string(raw)
	}
	text, isString := data.(string)
	if !isString {
		raw, err := json.Marshal(data)
		if err != nil {
			return 0, 0, err
		}
		text = string(raw)
	}
	var id api.RecordID
	next, err := m.guarded(ctx, thread, seq, true, func(conn *sql.Conn) error {
		if err := conn.QueryRowContext(ctx,
			"UPDATE threads SET next_record = next_record + 1 WHERE user = ? AND id = ? RETURNING next_record - 1",
			m.user, thread).Scan(&id); err != nil {
			return err
		}
		_, err := conn.ExecContext(ctx,
			"INSERT INTO records (user, thread, id, origin, kind, attrs, data, data_string) VALUES (?, ?, ?, ?, ?, ?, ?, ?)",
			m.user, thread, id, origin, kind, rawAttrs, text, isString)
		return err
	})
	if err != nil {
		return 0, 0, err
	}
	return id, next, nil
}

func (m *MessageStorage) RecordPin(ctx context.Context, thread api.ThreadID, seq api.Seq, pin api.RoundID, records []api.RecordID) (api.Seq, error) {
	return m.guarded(ctx, thread, seq, true, func(conn *sql.Conn) error {
		for _, id := range records {
			res, err := conn.ExecContext(ctx,
				"UPDATE records SET pin = ? WHERE user = ? AND thread = ? AND id = ?",
				pin, m.user, thread, id)
			if err != nil {
				return err
			}
			n, err := res.RowsAffected()
			if err != nil {
				return err
			}
			if n == 0 {
				return api.ErrNotFound
			}
		}
		return nil
	})
}

func (m *MessageStorage) RecordList(ctx context.Context, thread api.ThreadID) ([]api.Record, error) {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := m.threadExists(ctx, tx, thread); err != nil {
		return nil, err
	}
	rows, err := tx.QueryContext(ctx,
		"SELECT id, origin, pin, kind, attrs, data, data_string FROM records WHERE user = ? AND thread = ? AND pin = -1 ORDER BY id",
		m.user, thread)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []api.Record
	for rows.Next() {
		var rec api.Record
		var attrs, data sql.NullString
		var isString bool
		if err := rows.Scan(&rec.ID, &rec.Origin, &rec.Pin, &rec.Kind, &attrs, &data, &isString); err != nil {
			return nil, err
		}
		if rec.Attrs, err = decodeAttrs(attrs); err != nil {
			return nil, err
		}
		rec.Data = decodeRecordData(data, isString)
		out = append(out, rec)
	}
	return out, rows.Err()
}

func (m *MessageStorage) guarded(ctx context.Context, thread api.ThreadID, seq api.Seq, touch bool, change func(conn *sql.Conn) error) (api.Seq, error) {
	var current api.Seq
	err := write(ctx, m.db, func(conn *sql.Conn) error {
		err := conn.QueryRowContext(ctx, "SELECT seq FROM threads WHERE user = ? AND id = ?", m.user, thread).Scan(&current)
		if errors.Is(err, sql.ErrNoRows) {
			return api.ErrNotFound
		}
		if err != nil {
			return err
		}
		if current != seq {
			return api.ErrDrift
		}
		if err := change(conn); err != nil {
			return err
		}
		current++
		if touch {
			_, err = conn.ExecContext(ctx, "UPDATE threads SET seq = ?, updated = ? WHERE user = ? AND id = ?", current, time.Now().UnixNano(), m.user, thread)
		} else {
			_, err = conn.ExecContext(ctx, "UPDATE threads SET seq = ? WHERE user = ? AND id = ?", current, m.user, thread)
		}
		return err
	})
	if err != nil {
		return 0, err
	}
	return current, nil
}

func (m *MessageStorage) updateRound(ctx context.Context, thread api.ThreadID, seq api.Seq, round api.RoundID, set string, args ...any) (api.Seq, error) {
	return m.guarded(ctx, thread, seq, true, func(conn *sql.Conn) error {
		res, err := conn.ExecContext(ctx,
			"UPDATE rounds SET "+set+" WHERE user = ? AND thread = ? AND id = ?",
			append(args, m.user, thread, round)...)
		if err != nil {
			return err
		}
		n, err := res.RowsAffected()
		if err != nil {
			return err
		}
		if n == 0 {
			return api.ErrNotFound
		}
		return nil
	})
}

func (m *MessageStorage) threadExists(ctx context.Context, tx *sql.Tx, thread api.ThreadID) error {
	var exists bool
	if err := tx.QueryRowContext(ctx, "SELECT EXISTS (SELECT 1 FROM threads WHERE user = ? AND id = ?)", m.user, thread).Scan(&exists); err != nil {
		return err
	}
	if !exists {
		return api.ErrNotFound
	}
	return nil
}

func (m *MessageStorage) loadRounds(ctx context.Context, thread api.ThreadID, results bool, filter string, args ...any) ([]api.Round, error) {
	tx, err := m.db.BeginTx(ctx, &sql.TxOptions{ReadOnly: true})
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := m.threadExists(ctx, tx, thread); err != nil {
		return nil, err
	}
	resultsColumn := "results"
	if !results {
		resultsColumn = "NULL"
	}
	rows, err := tx.QueryContext(ctx,
		"SELECT id, created, state, input, error, usage, response, "+resultsColumn+" FROM rounds WHERE user = ? AND thread = ? "+filter,
		append([]any{m.user, thread}, args...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rounds := []api.Round{}
	for rows.Next() {
		r := api.Round{Thread: thread}
		var input, usage string
		var created int64
		var response, results sql.NullString
		if err := rows.Scan(&r.ID, &created, &r.State, &input, &r.Error, &usage, &response, &results); err != nil {
			return nil, err
		}
		r.Created = time.Unix(0, created)
		if err := json.Unmarshal([]byte(input), &r.Input); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(usage), &r.Usage); err != nil {
			return nil, err
		}
		if r.Response, err = decodeResponse(response); err != nil {
			return nil, err
		}
		if r.Results, err = decodeResults(results); err != nil {
			return nil, err
		}
		rounds = append(rounds, r)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	if len(rounds) == 0 {
		return rounds, nil
	}
	slices.SortFunc(rounds, func(a, b api.Round) int { return int(a.ID - b.ID) })

	dataColumns := "NULL, 0"
	if results {
		dataColumns = "data, data_string"
	}
	pinned, err := tx.QueryContext(ctx,
		"SELECT id, origin, pin, kind, attrs, "+dataColumns+" FROM records WHERE user = ? AND thread = ? AND pin BETWEEN ? AND ? ORDER BY id",
		m.user, thread, rounds[0].ID, rounds[len(rounds)-1].ID)
	if err != nil {
		return nil, err
	}
	defer pinned.Close()
	pins := map[api.RoundID][]api.Record{}
	for pinned.Next() {
		var rec api.Record
		var attrs, data sql.NullString
		var isString bool
		if err := pinned.Scan(&rec.ID, &rec.Origin, &rec.Pin, &rec.Kind, &attrs, &data, &isString); err != nil {
			return nil, err
		}
		if rec.Attrs, err = decodeAttrs(attrs); err != nil {
			return nil, err
		}
		rec.Data = decodeRecordData(data, isString)
		pins[rec.Pin] = append(pins[rec.Pin], rec)
	}
	if err := pinned.Err(); err != nil {
		return nil, err
	}
	for i := range rounds {
		rounds[i].PinnedRecords = pins[rounds[i].ID]
	}
	return rounds, nil
}

func scanThread(row rowScanner) (api.Thread, error) {
	var t api.Thread
	var created, updated int64
	if err := row.Scan(&t.ID, &t.Name, &t.Seq, &created, &updated); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return api.Thread{}, api.ErrNotFound
		}
		return api.Thread{}, err
	}
	t.Created = time.Unix(0, created)
	t.Updated = time.Unix(0, updated)
	return t, nil
}

func encodeResponse(content api.Indexed[api.Content]) (any, error) {
	if content == nil {
		return nil, nil
	}
	stored := storedResponse{}
	for _, key := range slices.Sorted(maps.Keys(content)) {
		stored.Keys = append(stored.Keys, key)
		stored.Content = append(stored.Content, content[key])
	}
	raw, err := json.Marshal(stored)
	return string(raw), err
}

func decodeResponse(raw sql.NullString) (api.Indexed[api.Content], error) {
	if !raw.Valid {
		return nil, nil
	}
	stored := storedResponse{}
	if err := json.Unmarshal([]byte(raw.String), &stored); err != nil {
		return nil, err
	}
	if len(stored.Keys) != len(stored.Content) {
		return nil, errors.New("sqlite: response keys and content differ in length")
	}
	content := make(api.Indexed[api.Content], len(stored.Keys))
	for i, key := range stored.Keys {
		content[key] = stored.Content[i]
	}
	return content, nil
}

func encodeResults(results map[string]api.ContentToolResult) (any, error) {
	if results == nil {
		return nil, nil
	}
	stored := make(map[string]storedResult, len(results))
	for id, result := range results {
		stored[id] = storedResult{Content: result.Content, Error: result.Error}
	}
	raw, err := json.Marshal(stored)
	return string(raw), err
}

func decodeResults(raw sql.NullString) (map[string]api.ContentToolResult, error) {
	if !raw.Valid {
		return nil, nil
	}
	stored := map[string]storedResult{}
	if err := json.Unmarshal([]byte(raw.String), &stored); err != nil {
		return nil, err
	}
	results := make(map[string]api.ContentToolResult, len(stored))
	for id, entry := range stored {
		results[id] = api.ContentToolResult{ID: id, Content: entry.Content, Error: entry.Error}
	}
	return results, nil
}

func decodeAttrs(raw sql.NullString) (map[string]any, error) {
	if !raw.Valid {
		return nil, nil
	}
	var attrs map[string]any
	err := json.Unmarshal([]byte(raw.String), &attrs)
	return attrs, err
}

func decodeRecordData(raw sql.NullString, isString bool) any {
	switch {
	case !raw.Valid:
		return nil
	case isString:
		return raw.String
	case raw.String != "null":
		return json.RawMessage(raw.String)
	}
	return nil
}
