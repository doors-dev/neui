package replay

import (
	"context"
	"errors"
	"slices"
	"strings"
	"time"

	"neui.dev/api"
)

var ErrEmptySummary = errors.New("compaction response carries no text")

type Replay = *replay

func NewReplay(ctx context.Context, thread api.ThreadID, s api.MessageStorage, config api.ThreadConfig, model api.Model) Replay {
	return &replay{
		ctx:     ctx,
		id:      thread,
		storage: s,
		config:  config,
		model:   model,
		factor:  1,
		lastRound: lastRound{
			id: api.NoRoundID,
		},
	}
}

type replay struct {
	ctx          context.Context
	id           api.ThreadID
	storage      api.MessageStorage
	config       api.ThreadConfig
	model        api.Model
	factor       float32
	thread       api.Thread
	messages     *messages
	lastRound    lastRound
	failed       bool
	failedWeight int64
	failedTokens int
}

func (r Replay) Tighten() bool {
	if r.factor < 1 {
		return false
	}
	r.factor = 0.8
	if r.messages != nil {
		r.messages.factor = r.factor
	}
	return true
}

func (r Replay) IsPending() bool {
	if !r.IsLoaded() {
		return false
	}
	return r.messages.isStashed()
}

func (r Replay) Usage() api.EventUsage {
	if !r.IsLoaded() || len(r.messages.rounds) == 0 {
		return api.EventUsage{}
	}
	last := r.messages.rounds[len(r.messages.rounds)-1]
	compaction := r.messages.compaction
	if last.id <= compaction.Tail {
		return api.EventUsage{Input: max(0, compaction.Output+last.usage.Total()-compaction.Base)}
	}
	return last.usage
}

func (r Replay) IsRetriable() bool {
	if !r.IsLoaded() {
		return false
	}
	return r.lastRound.isRetriable()
}

type CompactionCause int

const (
	CompactionNone CompactionCause = iota
	CompactionTokens
	CompactionBytes
)

func (r Replay) NeedCompaction() CompactionCause {
	if !r.IsLoaded() {
		return CompactionNone
	}
	rounds := r.messages.rounds
	last := len(rounds) - 1
	tail := r.messages.compaction.Tail
	if last < 1 || rounds[last].id <= tail || rounds[last-1].id <= tail {
		return CompactionNone
	}
	if r.failed && r.messages.weight*4 <= r.failedWeight*5 && r.Usage().Total()*4 <= r.failedTokens*5 {
		return CompactionNone
	}
	limit := memoryLimit(r.config, r.model, r.factor)
	if limit > 0 && r.messages.weight+weigh(r.messages.pendingContent(api.Input{})...) > limit {
		return CompactionBytes
	}
	if r.model.Context != 0 && float32(r.Usage().Total()) >= r.config.CompactionTrigger*float32(r.model.Context) {
		return CompactionTokens
	}
	return CompactionNone
}

func (r Replay) CanCompact() bool {
	if !r.IsLoaded() {
		return false
	}
	rounds := r.messages.rounds
	last := len(rounds) - 1
	tail := r.messages.compaction.Tail
	if last < 1 || rounds[last].id <= tail || rounds[last-1].id <= tail {
		return false
	}
	return r.model.Context == 0 || float32(r.Usage().Total()) >= r.config.CompactionAllowed*float32(r.model.Context)
}

func (r Replay) CompactionFailed() {
	if !r.IsLoaded() {
		return
	}
	r.failed = true
	r.failedWeight = r.messages.weight
	r.failedTokens = r.Usage().Total()
}

func (r Replay) PrepareCompaction(force bool, chunk int) (Compaction, []api.Message, error) {
	if !r.IsLoaded() {
		return Compaction{}, nil, errors.New("state missmatch")
	}
	c, m := r.messages.prepeareCompaction(force, chunk)
	return c, m, nil
}

func (r Replay) ApplyCompaction(c Compaction, content api.Indexed[api.Content], usage api.EventUsage) error {
	var b strings.Builder
	for part := range content.Iter() {
		text, ok := part.(api.ContentText)
		if !ok {
			continue
		}
		if b.Len() != 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(text.Text)
	}
	text := b.String()
	if text == "" {
		return ErrEmptySummary
	}
	rounds := r.messages.rounds
	if c.index < 1 || c.index >= len(rounds) || rounds[c.index].id != c.Target {
		panic("prepared compaction does not match the replay any more")
	}
	previous := rounds[c.index-1]
	base := previous.usage.Total()
	if previous.id <= r.messages.compaction.Tail && rounds[len(rounds)-1].id > r.messages.compaction.Tail {
		base = r.messages.compaction.Output + base - r.messages.compaction.Base
	}
	record := api.Compaction{
		Text:   text,
		Target: c.Target,
		Tail:   rounds[len(rounds)-1].id,
		Output: usage.Output,
		Base:   base,
	}
	seq, err := r.storage.CompactionCreate(r.ctx, r.id, r.seq(), record)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.messages.applyCompaction(c, record)
	r.failed = false
	return nil
}

func (r Replay) Rename(name string) error {
	seq, err := r.storage.ThreadRename(r.ctx, r.id, r.seq(), name)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.thread.Name = name
	return nil
}

func (r Replay) Name() string {
	return r.thread.Name
}

func (r Replay) Rewind(end api.RoundID, input api.Input) (api.Round, []api.Message, []string, error) {
	if !r.IsLoaded() {
		return api.Round{}, nil, nil, errors.New("state missmatch")
	}
	var removed []string
	pending := input.Len() != 0
	for round, err := range r.storage.RoundRange(r.ctx, r.id, end, api.NoRoundID) {
		if err != nil {
			return api.Round{}, nil, nil, err
		}
		if slices.ContainsFunc(round.PinnedRecords, func(rec api.Record) bool { return rec.Origin < end }) {
			pending = true
		}
		if round.ID != end {
			removed = append(removed, round.Input.Attachments...)
			continue
		}
		for _, name := range round.Input.Attachments {
			if !slices.Contains(input.Attachments, name) {
				removed = append(removed, name)
			}
		}
	}
	if !pending {
		records, err := r.storage.RecordList(r.ctx, r.id)
		if err != nil {
			return api.Round{}, nil, nil, err
		}
		pending = slices.ContainsFunc(records, func(rec api.Record) bool { return rec.Origin < end })
	}
	for start, found := end, false; !pending && !found; {
		rounds, err := r.storage.RoundBefore(r.ctx, r.id, start, 16)
		if err != nil {
			return api.Round{}, nil, nil, err
		}
		if len(rounds) == 0 {
			break
		}
		for i := len(rounds) - 1; i >= 0 && !found; i-- {
			if rounds[i].State != api.RoundOk {
				continue
			}
			found = true
			for _, c := range rounds[i].Response {
				if _, ok := c.(api.ContentToolUse); ok {
					pending = true
				}
			}
		}
		start = rounds[0].ID
	}
	if !pending {
		return api.Round{}, nil, nil, api.ErrWrongState
	}
	_, err := r.storage.ThreadCut(r.ctx, r.id, r.seq(), end)
	if err != nil {
		return api.Round{}, nil, nil, err
	}
	r.messages = nil
	if err := r.Load(); err != nil {
		return api.Round{}, nil, nil, err
	}
	round, messages, err := r.RoundInit(input)
	return round, messages, removed, err
}

func (r Replay) RoundInit(input api.Input) (api.Round, []api.Message, error) {
	if !r.IsLoaded() {
		return api.Round{}, nil, errors.New("state missmatch")
	}
	messages, err := r.messages.replay(input)
	if err != nil {
		return api.Round{}, nil, err
	}
	if r.thread.Name == "" && input.Len() != 0 {
		last := &messages[len(messages)-1]
		last.Content = append(last.Content, api.ContentText{Text: nameInstruction})
	}
	id, seq, err := r.storage.RoundInput(r.ctx, r.id, r.seq(), input)
	if err != nil {
		return api.Round{}, nil, err
	}
	r.updateSeq(seq)
	r.lastRound = lastRound{
		id:      id,
		created: time.Now(),
		input:   input,
		state:   api.RoundInput,
		records: r.messages.unstashRecords(),
	}
	return r.lastRound.round(), messages, nil
}

func (r Replay) RoundOk(usage api.EventUsage, content api.Indexed[api.Content]) (api.RoundID, error) {
	if !r.IsLoaded() || !r.lastRound.isActive() {
		return api.NoRoundID, errors.New("state missmatch")
	}
	seq, err := r.storage.RoundOk(
		r.ctx,
		r.id,
		r.seq(),
		r.lastRound.id,
		usage,
		content,
	)
	if err != nil {
		return api.NoRoundID, err
	}
	r.updateSeq(seq)
	if len(r.lastRound.records) != 0 {
		ids := make([]api.RecordID, len(r.lastRound.records))
		for i, rec := range r.lastRound.records {
			ids[i] = rec.ID
		}
		seq, err = r.storage.RecordPin(r.ctx, r.id, r.seq(), r.lastRound.id, ids)
		if err != nil {
			return api.NoRoundID, err
		}
		r.updateSeq(seq)
	}
	r.lastRound.state = api.RoundOk
	r.messages.appendRound(api.Round{
		ID:            r.lastRound.id,
		Thread:        r.id,
		State:         api.RoundOk,
		Input:         r.lastRound.input,
		Usage:         usage,
		Response:      content,
		PinnedRecords: r.lastRound.records,
	})
	return r.lastRound.id, nil
}

func (r Replay) RoundError(err error, usage api.EventUsage, content api.Indexed[api.Content]) error {
	if !r.IsLoaded() || !r.lastRound.isActive() {
		return errors.New("state missmatch")
	}
	seq, err := r.storage.RoundError(r.ctx, r.id, r.seq(), r.lastRound.id, err.Error(), usage, content)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.lastRound.state = api.RoundError
	if len(r.lastRound.records) != 0 {
		r.messages.restoreRecords(r.lastRound.records)
		r.lastRound.records = nil
	}
	r.messages.appendRound(api.Round{
		ID:       r.lastRound.id,
		Thread:   r.id,
		State:    api.RoundError,
		Input:    r.lastRound.input,
		Usage:    usage,
		Response: content,
	})
	return nil
}

func (r Replay) RoundAbort(usage api.EventUsage, content api.Indexed[api.Content]) error {
	if !r.IsLoaded() || !r.lastRound.isActive() {
		return errors.New("state missmatch")
	}
	seq, err := r.storage.RoundAbort(r.ctx, r.id, r.seq(), r.lastRound.id, usage, content)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.lastRound.state = api.RoundAbort
	if len(r.lastRound.records) != 0 {
		r.messages.restoreRecords(r.lastRound.records)
		r.lastRound.records = nil
	}
	r.messages.appendRound(api.Round{
		ID:       r.lastRound.id,
		Thread:   r.id,
		State:    api.RoundAbort,
		Input:    r.lastRound.input,
		Usage:    usage,
		Response: content,
	})
	return nil
}

func (r Replay) RoundRefuse(usage api.EventUsage, content api.Indexed[api.Content]) error {
	if !r.IsLoaded() || !r.lastRound.isActive() {
		return errors.New("state missmatch")
	}
	seq, err := r.storage.RoundRefuse(r.ctx, r.id, r.seq(), r.lastRound.id, usage, content)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.lastRound.state = api.RoundRefuse
	if len(r.lastRound.records) != 0 {
		r.messages.restoreRecords(r.lastRound.records)
		r.lastRound.records = nil
	}
	r.messages.appendRound(api.Round{
		ID:       r.lastRound.id,
		Thread:   r.id,
		State:    api.RoundRefuse,
		Input:    r.lastRound.input,
		Usage:    usage,
		Response: content,
	})
	return nil
}

func (r Replay) RoundReject(code api.ErrCode, usage api.EventUsage, content api.Indexed[api.Content]) error {
	if err := r.RoundError(errors.New(string(code)), usage, content); err != nil {
		return err
	}
	r.messages.shedStash(rejectedMedia)
	return nil
}

func (r Replay) RoundResult(results map[string]api.ContentToolResult) error {
	if !r.IsLoaded() || !r.lastRound.isCompleted() {
		return errors.New("state missmatch")
	}
	seq, err := r.storage.RoundResult(r.ctx, r.id, r.seq(), r.lastRound.id, results)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.messages.stashResults(results)
	return nil
}

func (r Replay) Record(origin api.RoundID, kind string, attrs map[string]any, data any) error {
	if !r.IsLoaded() {
		return errors.New("state missmatch")
	}
	if origin == api.NoRoundID {
		if last, err := r.messages.lastRound(); err == nil {
			origin = last
		}
	}
	id, seq, err := r.storage.RecordAdd(r.ctx, r.id, r.seq(), origin, kind, attrs, data)
	if err != nil {
		return err
	}
	r.updateSeq(seq)
	r.messages.stashRecords(api.Record{
		ID:     id,
		Origin: origin,
		Pin:    api.NoRoundID,
		Kind:   kind,
		Attrs:  attrs,
		Data:   data,
	})
	return nil
}

func (r Replay) Retry() (api.Round, api.RoundID, []api.Message, error) {
	if !r.lastRound.isRetriable() {
		return api.Round{}, api.NoRoundID, nil, errors.New("nothing to retry")
	}
	truncate := r.lastRound.id
	seq, err := r.storage.ThreadCut(r.ctx, r.id, r.seq(), truncate)
	if err != nil {
		return api.Round{}, api.NoRoundID, nil, err
	}
	r.updateSeq(seq)
	if len(r.lastRound.records) != 0 {
		r.messages.restoreRecords(r.lastRound.records)
		r.lastRound.records = nil
	}
	round, replay, err := r.RoundInit(r.lastRound.input)
	return round, truncate, replay, err
}

func (r Replay) Load() error {
	if r.IsLoaded() {
		return errors.New("state missmatch")
	}
	thread, err := r.storage.ThreadBump(r.ctx, r.id)
	if err != nil {
		return err
	}
	r.thread = thread
	compaction, err := r.storage.CompactionGet(r.ctx, r.id, api.NoRoundID)
	if err != nil {
		return err
	}
	r.messages = newMessages(compaction, r.config, r.model, r.factor)
	last := lastRound{
		id: api.NoRoundID,
	}
	for round, err := range r.storage.RoundRange(r.ctx, r.id, compaction.Target, api.NoRoundID) {
		if err != nil {
			r.messages = nil
			return err
		}
		r.messages.appendRound(round)
		last = lastRound{
			id:      round.ID,
			created: round.Created,
			input:   round.Input,
			state:   round.State,
		}
	}
	r.lastRound = last
	records, err := r.storage.RecordList(r.ctx, r.id)
	if err != nil {
		return err
	}
	r.messages.stashRecords(records...)
	return nil
}

func (r Replay) Create() (bool, error) {
	if r.IsLoaded() {
		return false, nil
	}
	thread, err := r.storage.ThreadCreate(r.ctx, r.id)
	if err != nil {
		return false, err
	}
	r.thread = thread
	r.messages = newMessages(api.NoCompaction, r.config, r.model, r.factor)
	return true, nil
}

func (r Replay) IsLoaded() bool {
	return r.messages != nil
}

func (r Replay) seq() api.Seq {
	return r.thread.Seq
}

func (r Replay) updateSeq(seq api.Seq) {
	r.thread.Seq = seq
}
