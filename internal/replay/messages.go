package replay

import (
	"errors"
	"slices"

	"neui.dev/api"
)

func newMessages(compaction api.Compaction, config api.ThreadConfig, model api.Model, factor float32) *messages {
	return &messages{
		compaction: compaction,
		config:     config,
		model:      model,
		factor:     factor,
		first:      api.NoRoundID,
	}
}

type round struct {
	id     api.RoundID
	user   []api.Content
	agent  []api.Content
	usage  api.EventUsage
	weight int64
}

type messages struct {
	compaction  api.Compaction
	first       api.RoundID
	rounds      []round
	weight      int64
	resultStash []api.ContentToolResult
	recordStash []api.Record
	config      api.ThreadConfig
	model       api.Model
	factor      float32
}

func (s *messages) lastRound() (api.RoundID, error) {
	if len(s.rounds) == 0 {
		return api.NoRoundID, errors.New("no successful turns")
	}
	return s.rounds[len(s.rounds)-1].id, nil
}

func (s *messages) pendingContent(input api.Input) []api.Content {
	resultsCap := ResultTextCap(s.config, s.model)
	content := make([]api.Content, 0, len(s.resultStash)+len(s.recordStash)+input.Len())
	for _, res := range s.resultStash {
		content = append(content, capText(res, resultsCap))
	}
	for _, rec := range s.recordStash {
		content = append(content, recordContent(rec, resultsCap))
	}
	content = append(content, input.Content()...)
	return omitOversizedMedia(content, s.model, memoryLimit(s.config, s.model, s.factor))
}

func (s *messages) replay(input api.Input) ([]api.Message, error) {
	if input.Len() == 0 && len(s.resultStash) == 0 && len(s.recordStash) == 0 {
		return nil, errors.New("all turnes are finished")
	}
	limit := memoryLimit(s.config, s.model, s.factor)
	content := s.pendingContent(input)
	pendingBytes := weigh(content...)
	shed := false
	switch {
	case limit <= 0 || s.weight+pendingBytes <= limit:
	case pendingBytes > limit:
		shed = true
	default:
		target := evictionTarget(limit)
		total := s.weight + pendingBytes
		users := make([][]api.Content, len(s.rounds))
		weights := make([]int64, len(s.rounds))
		for i := range s.rounds {
			if total <= target {
				break
			}
			user := omitAllMedia(s.rounds[i].user, evictedMedia)
			weight := weigh(user...) + weigh(s.rounds[i].agent...)
			if weight >= s.rounds[i].weight {
				continue
			}
			total -= s.rounds[i].weight - weight
			users[i] = user
			weights[i] = weight
		}
		if total > limit {
			shed = true
			break
		}
		for i, user := range users {
			if user == nil {
				continue
			}
			s.weight -= s.rounds[i].weight - weights[i]
			s.rounds[i].user = user
			s.rounds[i].weight = weights[i]
		}
	}
	if !shed && s.model.ImagesPerRequest > 0 && countImages(content...) > s.model.ImagesPerRequest {
		shed = true
	}
	if shed {
		s.shedStash(evictedMedia)
		content = s.pendingContent(input)
	}
	if s.model.ImagesPerRequest > 0 {
		pendingImages := countImages(content...)
		images := pendingImages
		for _, r := range s.rounds {
			images += countImages(r.user...)
		}
		if images > s.model.ImagesPerRequest && pendingImages <= s.model.ImagesPerRequest {
			target := evictionTarget(s.model.ImagesPerRequest)
			users := make([][]api.Content, len(s.rounds))
			weights := make([]int64, len(s.rounds))
			for i := range s.rounds {
				if images <= target {
					break
				}
				users[i] = omitAllMedia(s.rounds[i].user, evictedMedia)
				weights[i] = weigh(users[i]...) + weigh(s.rounds[i].agent...)
				images -= countImages(s.rounds[i].user...)
			}
			if images <= s.model.ImagesPerRequest {
				for i, user := range users {
					if user == nil {
						continue
					}
					s.weight -= s.rounds[i].weight - weights[i]
					s.rounds[i].user = user
					s.rounds[i].weight = weights[i]
				}
			}
		}
	}
	return append(s.messages(0), api.Message{
		Role:    api.RoleUser,
		Content: content,
	}), nil
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
	if r.State == api.RoundRefuse {
		s.refuseResults()
	}
	if r.State != api.RoundOk {
		return
	}
	resultsCap := ResultTextCap(s.config, s.model)
	userContent := make([]api.Content, 0, len(s.resultStash)+len(r.PinnedRecords)+r.Input.Len()+1)
	for _, res := range s.resultStash {
		userContent = append(userContent, capText(res, resultsCap))
	}
	s.resultStash = nil
	if s.first == api.NoRoundID {
		s.first = r.ID
		if s.compaction.IsValid() {
			userContent = append(userContent, noteContent("summary", s.compaction.Text))
		}
	}
	for _, rec := range r.PinnedRecords {
		userContent = append(userContent, recordContent(rec, resultsCap))
	}
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
	limit := memoryLimit(s.config, s.model, s.factor)
	user := omitOversizedMedia(userContent, s.model, limit)
	weight := weigh(user...) + weigh(response...)
	s.rounds = append(s.rounds, round{
		id:     r.ID,
		user:   user,
		agent:  response,
		usage:  r.Usage,
		weight: weight,
	})
	s.weight += weight
}

type Compaction struct {
	Target api.RoundID
	Tail   api.RoundID
	index  int
}

func (s *messages) isStashed() bool {
	return len(s.resultStash) != 0 || len(s.recordStash) != 0
}

func (s *messages) applyCompaction(c Compaction, record api.Compaction) {
	target := s.rounds[c.index]
	body := slices.IndexFunc(target.user, func(c api.Content) bool {
		_, ok := c.(api.ContentToolResult)
		return !ok
	})
	if body == -1 {
		body = len(target.user)
	}
	target.user = slices.Concat([]api.Content{noteContent("summary", record.Text)}, target.user[body:])
	target.weight = weigh(target.user...) + weigh(target.agent...)
	s.rounds = slices.Concat([]round{target}, s.rounds[c.index+1:])
	s.weight = 0
	for _, r := range s.rounds {
		s.weight += r.weight
	}
	s.first = record.Target
	s.compaction = record
}

func (s *messages) prepeareCompaction() (Compaction, []api.Message) {
	last := len(s.rounds) - 1
	if last < 1 || s.rounds[last].id <= s.compaction.Tail || s.rounds[last-1].id <= s.compaction.Tail {
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

func (s *messages) unstashRecords() []api.Record {
	records := s.recordStash
	s.recordStash = nil
	return records
}

func (s *messages) refuseResults() {
	for i, res := range s.resultStash {
		s.resultStash[i] = res.Refused()
	}
}

func (s *messages) shedStash(reason string) {
	for i, res := range s.resultStash {
		s.resultStash[i] = omitAllMedia([]api.Content{res}, reason)[0].(api.ContentToolResult)
	}
}
