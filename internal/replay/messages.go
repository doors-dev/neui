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
	"errors"
	"math"
	"slices"

	"neui.dev/api"
)

func newMessages(compaction api.Compaction, config api.ThreadConfig, model api.Model) *messages {
	return &messages{
		compaction: compaction,
		config:     config,
		model:      model,
		first:      api.NoRoundID,
	}
}

type round struct {
	id     api.RoundID
	user   []api.Content
	agent  []api.Content
	usage  api.EventUsage
	weight int64
	images int
	media  int64
}

type messages struct {
	compaction  api.Compaction
	first       api.RoundID
	rounds      []round
	cursor      int
	weight      int64
	resultStash []api.ContentToolResult
	recordStash []api.Record
	config      api.ThreadConfig
	model       api.Model
}

func (s *messages) lastRound() (api.RoundID, error) {
	if len(s.rounds) == 0 {
		return api.NoRoundID, errors.New("no successful turns")
	}
	return s.rounds[len(s.rounds)-1].id, nil
}

func (s *messages) pendingContent(input api.Input) []api.Content {
	budget := RoundItemBudget(s.config, s.model)
	records := s.pendingRecords()
	content := make([]api.Content, 0, len(s.resultStash)+len(records)+input.Len())
	for _, res := range s.resultStash {
		content = append(content, capText(res, budget))
	}
	for _, rec := range records {
		content = append(content, recordContent(rec, budget))
	}
	input.Text = trimText(api.ContentText{Text: input.Text}, budget).Text
	content = append(content, input.Content()...)
	return omitOversizedMedia(content, s.model)
}

func (s *messages) replay(input api.Input) ([]api.Message, error) {
	if input.Len() == 0 && len(s.resultStash) == 0 && len(s.recordStash) == 0 {
		return nil, errors.New("all turnes are finished")
	}
	content := s.cut(s.pendingContent(input))
	return append(s.messages(0), api.Message{
		Role:    api.RoleUser,
		Content: content,
	}), nil
}

func (s *messages) weightAfterCut(pending []api.Content) int64 {
	weight, media := s.weight+weigh(pending...), mediaBytes(pending...)
	for _, r := range s.rounds[s.cursor:] {
		media += r.media
	}
	if memory := s.config.ThreadMemoryLimit; memory > 0 && media > memory*4/5 {
		weight -= media - memory/2
	}
	return weight
}

func (s *messages) cut(pending []api.Content) []api.Content {
	limit := s.model.ImagesPerRequest
	memory := s.config.ThreadMemoryLimit
	images, media := countImages(pending...), mediaBytes(pending...)
	totalImages, totalMedia := images, media
	for _, r := range s.rounds[s.cursor:] {
		totalImages += r.images
		totalMedia += r.media
	}
	over := func(imagesAt int, mediaAt int64) bool {
		return limit > 0 && totalImages > imagesAt || memory > 0 && totalMedia > mediaAt
	}
	if over(limit, memory*4/5) {
		for ; s.cursor < len(s.rounds) && over(limit*3/5, memory/2); s.cursor++ {
			r := &s.rounds[s.cursor]
			user := omitMedia(r.user, math.MaxInt, seenMedia)
			weight := weigh(user...) + weigh(r.agent...)
			s.weight += weight - r.weight
			totalImages -= r.images
			totalMedia -= r.media
			r.user, r.weight, r.images, r.media = user, weight, 0, 0
		}
	}
	if limit <= 0 || images <= limit {
		return pending
	}
	var charges []int
	for _, block := range pending {
		switch b := block.(type) {
		case api.ContentImage:
			charges = append(charges, 1)
		case api.ContentDocument:
			charges = append(charges, pdfPages(b))
		case api.ContentToolResult:
			for _, nested := range b.Content {
				switch n := nested.(type) {
				case api.ContentImage:
					charges = append(charges, 1)
				case api.ContentDocument:
					charges = append(charges, pdfPages(n))
				}
			}
		}
	}
	n, kept := len(charges), 0
	for n > 0 && kept+charges[n-1] <= limit {
		n--
		kept += charges[n]
	}
	return omitMedia(pending, n, unseenMedia)
}

func (s *messages) messages(limit int) []api.Message {
	rounds := s.rounds
	if limit > 0 && limit < len(rounds) {
		rounds = rounds[:limit]
	}
	m := make([]api.Message, 0, len(rounds)*2+1)
	for _, round := range rounds {
		m = append(m, api.Message{
			Role:    api.RoleUser,
			Content: round.user,
		}, api.Message{
			Role:    api.RoleAssistant,
			Content: round.agent,
		})
	}
	return m
}

func (s *messages) appendRound(r api.Round) {
	if r.State != api.RoundOk {
		s.cut(s.pendingContent(r.Input))
		if refuses(r.State, r.Error) {
			s.refuseResults()
		}
		return
	}
	budget := RoundItemBudget(s.config, s.model)
	userContent := make([]api.Content, 0, len(s.resultStash)+len(r.PinnedRecords)+r.Input.Len()+1)
	for _, res := range s.resultStash {
		userContent = append(userContent, capText(res, budget))
	}
	s.resultStash = nil
	opening := s.first == api.NoRoundID && !s.compaction.IsValid() && r.Input.Len() != 0
	if s.first == api.NoRoundID {
		s.first = r.ID
		if s.compaction.IsValid() {
			userContent = append(userContent, noteContent("summary", s.compaction.Text))
		}
	}
	for _, rec := range r.PinnedRecords {
		userContent = append(userContent, recordContent(rec, budget))
	}
	r.Input.Text = trimText(api.ContentText{Text: r.Input.Text}, budget).Text
	userContent = append(userContent, r.Input.Content()...)
	response := r.Response.Ordered()
	for _, content := range response {
		call, ok := content.(api.ContentToolUse)
		if !ok {
			continue
		}
		if res, ok := r.Results[call.ID]; ok {
			s.resultStash = append(s.resultStash, res)
		} else {
			s.resultStash = append(s.resultStash, call.LostResult())
		}
	}
	user := s.cut(omitOversizedMedia(userContent, s.model))
	if opening {
		user = append(user, api.ContentText{Text: nameInstruction})
	}
	weight := weigh(user...) + weigh(response...)
	s.rounds = append(s.rounds, round{
		id:     r.ID,
		user:   user,
		agent:  response,
		usage:  r.Usage,
		weight: weight,
		images: countImages(user...),
		media:  mediaBytes(user...),
	})
	s.weight += weight
}

type Compaction struct {
	Target api.RoundID
	Tail   api.RoundID
	index  int
}

func (c Compaction) Half() int {
	return c.index / 2
}

func (s *messages) isStashed() bool {
	return len(s.resultStash) != 0 || len(s.recordStash) != 0
}

func (s *messages) prepeareCompaction(force bool, chunk int) (Compaction, []api.Message) {
	last := len(s.rounds) - 1
	if last < 1 || !force && (s.rounds[last].id <= s.compaction.Tail || s.rounds[last-1].id <= s.compaction.Tail) {
		return Compaction{
			Target: api.NoRoundID,
			Tail:   api.NoRoundID,
			index:  -1,
		}, nil
	}
	total := s.rounds[last].usage.Total()
	budget := int(float32(total) * s.config.CompactionPreserve)
	target := last
	floor := total
	for i := last - 1; i >= 0; i-- {
		r := s.rounds[i]
		if r.id <= s.compaction.Tail {
			break
		}
		floor = min(floor, r.usage.Total())
		if total-floor > budget {
			break
		}
		target = i + 1
	}
	if force && chunk > 0 {
		target = min(target, chunk)
	}
	replay := s.messages(target)
	user := s.rounds[target].user
	body := slices.IndexFunc(user, func(c api.Content) bool {
		_, ok := c.(api.ContentToolResult)
		return !ok
	})
	if body == -1 {
		body = len(user)
	}
	if body > 0 {
		replay = append(replay, api.Message{
			Role:    api.RoleUser,
			Content: slices.Clip(user[:body]),
		})
	}
	if replay[len(replay)-1].Role == api.RoleUser {
		last := &replay[len(replay)-1]
		last.Content = append(last.Content, api.ContentText{Text: compactionInstruction})
	} else {
		replay = append(replay, api.Message{
			Role:    api.RoleUser,
			Content: []api.Content{api.ContentText{Text: compactionInstruction}},
		})
	}
	return Compaction{
		Target: s.rounds[target].id,
		Tail:   s.rounds[last].id,
		index:  target,
	}, replay
}

func (s *messages) stashResults(results map[string]api.ContentToolResult) {
	for i, res := range s.resultStash {
		if r, ok := results[res.ID]; ok {
			s.resultStash[i] = r
		}
	}
}

func (s *messages) stashRecords(records ...api.Record) {
	s.recordStash = append(s.recordStash, records...)
}

func (s *messages) restoreRecords(records []api.Record) {
	s.recordStash = append(records, s.recordStash...)
}

func (s *messages) pendingRecords() []api.Record {
	budget := RoundItemBudget(s.config, s.model)
	n, total := len(s.recordStash), 0
	for i, rec := range s.recordStash {
		total += len(recordBody(rec))
		if total > budget {
			n = max(i, 1)
			break
		}
	}
	return slices.Clip(s.recordStash[:n])
}

func (s *messages) unstashRecords() []api.Record {
	records := s.pendingRecords()
	s.recordStash = s.recordStash[len(records):]
	return records
}

func refuses(state api.RoundState, err string) bool {
	return state == api.RoundRefuse || state == api.RoundError && (err == string(api.CodeInvalid) || err == string(api.CodeTooLarge))
}

func (s *messages) refuseResults() {
	for i, res := range s.resultStash {
		s.resultStash[i] = res.Refused()
	}
}
