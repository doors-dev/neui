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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand/v2"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
)

type fuzzRun struct {
	t      *testing.T
	rng    *rand.Rand
	s      api.MessageStorage
	r      Replay
	config api.ThreadConfig
	model  api.Model
	clones int
	media  int
	calls  []api.ContentToolUse
	log    []string
}

func (f *fuzzRun) fail(format string, args ...any) {
	f.t.Helper()
	f.t.Fatalf("after %v\n%s", f.log, fmt.Sprintf(format, args...))
}

func (f *fuzzRun) clone() Replay {
	f.t.Helper()
	f.clones++
	id := api.ThreadID(fmt.Sprintf("clone-%d", f.clones))
	if _, err := f.s.ThreadClone(context.Background(), "a", id); err != nil {
		f.fail("ThreadClone: %v", err)
	}
	l := NewReplay(context.Background(), id, f.s, f.config, f.model)
	if err := l.Load(); err != nil {
		f.fail("Load: %v", err)
	}
	return l
}

func (f *fuzzRun) same(l Replay) {
	f.t.Helper()
	r := f.r
	records := r.messages.recordStash
	if r.lastRound.isActive() {
		records = append(slices.Clone(r.lastRound.records), records...)
	}
	if l.messages.cursor != r.messages.cursor {
		f.fail("cursor %d after Load, %d in session", l.messages.cursor, r.messages.cursor)
	}
	if l.messages.weight != r.messages.weight {
		f.fail("weight %d after Load, %d in session", l.messages.weight, r.messages.weight)
	}
	if l.messages.first != r.messages.first {
		f.fail("first %d after Load, %d in session", l.messages.first, r.messages.first)
	}
	loaded, lerr := l.IsEmptyTurn(nil)
	session, serr := r.IsEmptyTurn(nil)
	if loaded != session || lerr != nil || serr != nil {
		f.fail("an empty response ends an empty turn %v (%v) after Load, %v (%v) in session", loaded, lerr, session, serr)
	}
	if r.lastRound.isActive() {
		var stored []api.Round
		for round, err := range f.s.RoundRange(context.Background(), "a", api.NoRoundID, api.NoRoundID) {
			if err != nil {
				f.fail("RoundRange: %v", err)
			}
			stored = append(stored, round)
		}
		empty, input := true, r.lastRound.input.Len() != 0
		for i := len(stored) - 1; empty && !input && i >= 0; i-- {
			if stored[i].State != api.RoundOk {
				continue
			}
			for _, c := range stored[i].Response {
				if text, ok := c.(api.ContentText); ok && strings.TrimSpace(text.Text) != "" {
					empty = false
				}
			}
			input = stored[i].Input.Len() != 0
		}
		if session != empty {
			f.fail("an empty response ends an empty turn %v in session, %v over every stored round", session, empty)
		}
	}
	for _, pair := range [][2]any{
		{l.messages.messages(0), r.messages.messages(0)},
		{l.messages.resultStash, r.messages.resultStash},
		{l.messages.recordStash, records},
		{l.messages.compaction, r.messages.compaction},
		{[]any{l.lastRound.id, l.lastRound.input, l.lastRound.state}, []any{r.lastRound.id, r.lastRound.input, r.lastRound.state}},
	} {
		g, _ := json.Marshal(pair[0])
		w, _ := json.Marshal(pair[1])
		if !bytes.Equal(g, w) && !(len(g)+len(w) == 6 && bytes.Contains([]byte("null[]"), g) && bytes.Contains([]byte("null[]"), w)) {
			f.fail("Load gives\n%s\nsession has\n%s", g, w)
		}
	}
	for i, round := range r.messages.rounds {
		if round.media != mediaBytes(round.user...) {
			f.fail("round %d caches %d media bytes, carries %d", i, round.media, mediaBytes(round.user...))
		}
		if i < r.messages.cursor && round.media != 0 {
			f.fail("round %d before the cursor keeps %d media bytes", i, round.media)
		}
	}
	limit := f.model.ImagesPerRequest
	if limit > 0 {
		total := 0
		for i, round := range r.messages.rounds {
			if round.images != countImages(round.user...) {
				f.fail("round %d caches %d images, carries %d", i, round.images, countImages(round.user...))
			}
			if i < r.messages.cursor && round.images != 0 {
				f.fail("round %d before the cursor keeps %d images", i, round.images)
			}
			total += round.images
		}
		if total > limit {
			f.fail("the rounds carry %d images, over the cap %d", total, limit)
		}
	}
}

func (f *fuzzRun) sameRequest(got, want []api.Message, op string) {
	f.t.Helper()
	g, _ := json.Marshal(got)
	w, _ := json.Marshal(want)
	if !bytes.Equal(g, w) {
		f.fail("%s after Load sends\n%s\nthe session sent\n%s", op, g, w)
	}
	if limit := f.model.ImagesPerRequest; limit > 0 && requestImages(want) > limit {
		f.fail("%s sends %d images, over the cap %d", op, requestImages(want), limit)
	}
	if kinds := sentKinds(want); slices.ContainsFunc(kinds, func(k api.Modality) bool { return !f.model.Inputs.Has(k) }) {
		f.fail("%s sends kinds %v to a model whose Inputs are %v", op, kinds, f.model.Inputs)
	}
}

func sentKinds(messages []api.Message) api.Modalities {
	var kinds api.Modalities
	for _, m := range messages {
		for _, block := range m.Content {
			blocks := []api.Content{block}
			if res, ok := block.(api.ContentToolResult); ok {
				blocks = make([]api.Content, len(res.Content))
				for i, c := range res.Content {
					blocks[i] = c
				}
			}
			for _, c := range blocks {
				switch c.(type) {
				case api.ContentImage:
					kinds = append(kinds, api.ModalityImage)
				case api.ContentDocument:
					kinds = append(kinds, api.ModalityDocument)
				}
			}
		}
	}
	slices.Sort(kinds)
	return slices.Compact(kinds)
}

func (f *fuzzRun) blocks() api.UserContents {
	var out api.UserContents
	for range f.rng.IntN(6) {
		f.media++
		switch f.rng.IntN(7) {
		case 0:
			out = append(out, api.ContentText{Text: fmt.Sprintf("text %d", f.media)})
		case 1:
			out = append(out, api.ContentDocument{Data: pdfDoc(1 + f.rng.IntN(4)), Mime: "application/pdf", Name: fmt.Sprintf("d%d.pdf", f.media)})
		case 2:
			out = append(out, api.ContentDocument{Data: []byte(fmt.Sprintf("plain %d", f.media)), Mime: "text/plain", Name: fmt.Sprintf("d%d.txt", f.media)})
		case 3:
			out = append(out, api.ContentDocument{Data: []byte{0xff, byte(f.media)}, Mime: "text/csv", Name: fmt.Sprintf("d%d.csv", f.media)})
		default:
			out = append(out, api.ContentImage{Data: []byte(fmt.Sprintf("img %d", f.media)), Mime: "image/png"})
		}
	}
	return out
}

func (f *fuzzRun) compact(force bool) bool {
	f.t.Helper()
	c, m, err := f.r.PrepareCompaction(force, f.rng.IntN(4))
	if err != nil {
		f.fail("PrepareCompaction: %v", err)
	}
	if len(m) == 0 {
		return false
	}
	if limit := f.model.ImagesPerRequest; limit > 0 && requestImages(m) > limit {
		f.fail("the compaction request carries %d images, over the cap %d", requestImages(m), limit)
	}
	f.log = append(f.log, "compact")
	if err := f.r.ApplyCompaction(c, api.Indexed[api.Content]{0: api.ContentText{Text: fmt.Sprintf("summary %d", len(f.log))}}, api.EventUsage{Output: 10}); err != nil {
		f.fail("ApplyCompaction: %v", err)
	}
	return true
}

func (f *fuzzRun) step() {
	f.t.Helper()
	r := f.r
	switch {
	case r.lastRound.isActive():
		switch f.rng.IntN(10) {
		case 0, 1, 2:
			n := 1 + f.rng.IntN(3)
			content := api.Indexed[api.Content]{}
			f.calls = nil
			for i := range n {
				call := api.ContentToolUse{ID: fmt.Sprintf("call-%d-%d", len(f.log), i), Name: "attach", Input: json.RawMessage(`{}`)}
				f.calls = append(f.calls, call)
				content[i] = call
			}
			if f.rng.IntN(2) == 0 {
				content[n] = api.ContentText{Text: []string{"checking", " \n"}[f.rng.IntN(2)]}
			}
			f.log = append(f.log, fmt.Sprintf("ok+%d calls+%d text", n, len(content)-n))
			if _, err := r.RoundOk(api.EventUsage{Input: 100 * len(f.log)}, content); err != nil {
				f.fail("RoundOk: %v", err)
			}
		case 3:
			f.log = append(f.log, "ok")
			if _, err := r.RoundOk(api.EventUsage{Input: 100 * len(f.log)}, api.Indexed[api.Content]{0: api.ContentText{Text: "answer"}}); err != nil {
				f.fail("RoundOk: %v", err)
			}
		case 4:
			code := []api.ErrCode{api.CodeInvalid, api.CodeTooLarge, api.CodeNetwork, api.CodeContextLength}[f.rng.IntN(4)]
			f.log = append(f.log, "error "+string(code))
			if err := r.RoundError(errors.New(string(code)), api.EventUsage{}, nil); err != nil {
				f.fail("RoundError: %v", err)
			}
		case 5:
			f.log = append(f.log, "refuse")
			if err := r.RoundRefuse(api.EventUsage{}, nil); err != nil {
				f.fail("RoundRefuse: %v", err)
			}
		case 6:
			f.log = append(f.log, "abort")
			if err := r.RoundAbort(api.EventUsage{}, nil); err != nil {
				f.fail("RoundAbort: %v", err)
			}
		case 7:
			note := f.rng.IntN(2) == 0
			f.log = append(f.log, fmt.Sprintf("resend note=%v", note))
			if _, _, _, err := r.Retry(note); err != nil {
				f.fail("Retry: %v", err)
			}
		default:
			if !f.compact(true) {
				return
			}
			f.log = append(f.log, "retry after compaction")
			if _, _, _, err := r.Retry(false); err != nil {
				f.fail("Retry: %v", err)
			}
		}
	case len(f.calls) != 0:
		if f.rng.IntN(5) == 0 {
			f.compact(true)
			return
		}
		results := map[string]api.ContentToolResult{}
		for _, call := range f.calls {
			results[call.ID] = api.ContentToolResult{ID: call.ID, Content: f.blocks()}
		}
		f.calls = nil
		f.log = append(f.log, "results")
		if err := r.RoundResult(results); err != nil {
			f.fail("RoundResult: %v", err)
		}
	default:
		switch f.rng.IntN(10) {
		case 0:
			if !r.IsRetriable() {
				return
			}
			l := f.clone()
			_, _, want, err := l.Retry(false)
			if err != nil {
				f.fail("Retry after Load: %v", err)
			}
			f.log = append(f.log, "retry")
			_, _, got, err := r.Retry(false)
			if err != nil {
				f.fail("Retry: %v", err)
			}
			f.sameRequest(want, got, "Retry")
		case 1:
			if len(r.messages.rounds) == 0 {
				return
			}
			end := r.messages.rounds[f.rng.IntN(len(r.messages.rounds))].id
			input := api.Input{Text: fmt.Sprintf("rewound %d", len(f.log))}
			l := f.clone()
			_, want, _, err := l.Rewind(end, input)
			if err != nil {
				f.fail("Rewind after Load: %v", err)
			}
			f.log = append(f.log, fmt.Sprintf("rewind %d", end))
			_, got, _, err := r.Rewind(end, input)
			if err != nil {
				f.fail("Rewind: %v", err)
			}
			f.sameRequest(want, got, "Rewind")
		case 2:
			f.log = append(f.log, "record")
			if err := r.Record(api.NoRoundID, "note", nil, fmt.Sprintf("record %d", len(f.log))); err != nil {
				f.fail("Record: %v", err)
			}
		case 3:
			f.compact(f.rng.IntN(2) == 0)
		default:
			input := api.Input{}
			if !r.messages.isStashed() || f.rng.IntN(3) == 0 {
				input.Text = fmt.Sprintf("input %d", len(f.log))
			}
			l := f.clone()
			_, want, err := l.RoundInit(input)
			if err != nil {
				f.fail("RoundInit after Load: %v", err)
			}
			f.log = append(f.log, "init")
			_, got, err := r.RoundInit(input)
			if err != nil {
				f.fail("RoundInit: %v", err)
			}
			f.sameRequest(want, got, "RoundInit")
		}
	}
}

func TestSessionMatchesLoadUnderRandomRounds(t *testing.T) {
	inputs := []api.Modalities{nil, {api.ModalityText}, {api.ModalityText, api.ModalityImage}, {api.ModalityDocument}}
	for seed := range uint64(300) {
		t.Run(fmt.Sprint(seed), func(t *testing.T) {
			rng := rand.New(rand.NewPCG(seed, 7))
			f := &fuzzRun{
				t:      t,
				rng:    rng,
				s:      local.NewMessageStorage(),
				config: api.ThreadConfig{ThreadMemoryLimit: []int64{-1, 300 + rng.Int64N(3000)}[seed%2], ReplayItemShare: 0.25, CompactionPreserve: 0.25},
				model:  api.Model{ImagesPerRequest: []int{1, 2, 3, 5, 8, 13}[rng.IntN(6)], Inputs: inputs[seed/2%4]},
			}
			f.r = NewReplay(context.Background(), "a", f.s, f.config, f.model)
			mustCreate(t, f.r)
			for range 80 {
				f.step()
				l := f.clone()
				f.same(l)
			}
			if f.r.lastRound.isActive() || len(f.calls) != 0 {
				return
			}
			switched := NewReplay(context.Background(), "a", f.s, f.config, api.Model{ImagesPerRequest: 1 + rng.IntN(4), Inputs: inputs[rng.IntN(4)]})
			if err := switched.Load(); err != nil {
				t.Fatalf("Load under another model: %v", err)
			}
			_, messages, err := switched.RoundInit(api.Input{Text: "after the switch"})
			if err != nil {
				t.Fatalf("RoundInit under another model: %v", err)
			}
			if got := requestImages(messages); got > switched.model.ImagesPerRequest {
				t.Fatalf("after a switch to a cap of %d the request carries %d images", switched.model.ImagesPerRequest, got)
			}
			if kinds := sentKinds(messages); slices.ContainsFunc(kinds, func(k api.Modality) bool { return !switched.model.Inputs.Has(k) }) {
				t.Fatalf("after a switch to Inputs %v the request carries kinds %v", switched.model.Inputs, kinds)
			}
		})
	}
}
