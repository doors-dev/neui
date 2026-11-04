// Managed by GoX v0.3.3

//line controller.gox:1
package controller

import (
	"context"
	"slices"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
)

type InputState struct {
	Seq int
	Input api.Input
	Pending bool
	Rewind bool
	Ask *api.ChatAsk
}

// should come from conf in constructor
const inputLimit = 256 * 1024

func NewComposerController(thread api.ChatThread, input api.ChatInput, active bool, settled api.EventUsage, tasks []api.Task, compaction bool, ask *api.ChatAsk) ComposerController {
	limit := inputLimit
	origin := doors.NewSourceEqual(state{
		input: input.Input,
		pending: input.Pending,
		rewind: invalidRewind,
		active: active,
		settled: settled,
		tasks: tasks,
		compaction: compaction,
		ask: ask,
	}, func(old, new state) bool {
		if old.inputSeq != new.inputSeq {
			return false
		}
		if old.pending != new.pending {
			return false
		}
		if old.active != new.active {
			return false
		}
		if old.uploading != new.uploading {
			return false
		}
		if old.inputBytes != new.inputBytes {
			return false
		}
		if old.settled != new.settled || old.unsettled != new.unsettled {
			return false
		}
		if old.compaction != new.compaction {
			return false
		}
		if !sameTasks(old.tasks, new.tasks) {
			return false
		}
		if old.input.Text != new.input.Text {
			return false
		}
		if !slices.Equal(old.input.Attachments, new.input.Attachments) {
			return false
		}
		if old.rewind.round != new.rewind.round {
			return false
		}
		if old.rewind.input.Text != new.rewind.input.Text {
			return false
		}
		if !slices.Equal(old.rewind.input.Attachments, new.rewind.input.Attachments) {
			return false
		}
		return true
	})
	reset := doors.DeriveBeam(origin, func(d state) int {
		return d.inputSeq
	})
	pending := doors.DeriveBeam(origin, func(d state) bool {
		return d.pending
	})
	enabled := doors.DeriveBeam(origin, func(d state) bool {
		if d.inputBytes > limit {
			return false
		}
		if d.rewind.isValid() {
			return d.uploading == 0 && d.rewind.input.Len() != 0
		}
		return !d.pending && d.uploading == 0 && d.input.Len() != 0
	})
	activity := doors.DeriveBeam(origin, func(d state) bool {
		return d.active
	})
	bar := doors.DeriveBeam(origin, func(d state) Bar {
		return Bar{d.active, d.settled.Total(), d.unsettled, d.compaction}
	})
	hasContext := doors.DeriveBeam(origin, func(d state) bool {
		return d.settled.Total() != 0
	})
	usage := doors.DeriveBeam(origin, func(d state) Context {
		return Context{d.settled, d.compaction}
	})
	hasTasks := doors.DeriveBeam(origin, func(d state) bool {
		return len(d.tasks) != 0
	})
	taskCount := doors.DeriveBeam(origin, func(d state) int {
		return len(d.tasks)
	})
	taskList := doors.DeriveBeamEqual(origin, func(d state) []api.Task {
		return d.tasks
	}, sameTasks)
	rewinding := doors.DeriveBeam(origin, func(d state) api.RoundID {
		return d.rewind.round
	})
	inputState := doors.DeriveBeamEqual(origin, func(d state) InputState {
		if d.rewind.isValid() {
			return InputState{
				Seq: d.inputSeq,
				Rewind: true,
				Input: d.rewind.input,
				Pending: false,
			}
		}
		return InputState{
			Seq: d.inputSeq,
			Input: d.input,
			Pending: d.pending,
			Ask: d.ask,
		}
	}, func(old, new InputState) bool {
		return old.Seq == new.Seq
	})
	excessBytes := doors.DeriveBeam(origin, func(d state) int {
		return max(d.inputBytes - limit, 0)
	})
	return ComposerController{
		thread: thread,
		origin: origin,
		input: inputState,
		rewinding: rewinding,
		reset: reset,
		pending: pending,
		enabled: enabled,
		active: activity,
		bar: bar,
		hasContext: hasContext,
		context: usage,
		hasTasks: hasTasks,
		taskCount: taskCount,
		tasks: taskList,
		excessBytes: excessBytes,
		inputLimit: limit,
	}
}

func sameTasks(a, b []api.Task) bool {
	return slices.EqualFunc(a, b, func(x, y api.Task) bool {
		return x.ID == y.ID
	})
}

type rewind struct {
	round api.RoundID
	input api.Input
}

var invalidRewind = rewind{
	round: api.NoRoundID,
}

func (r rewind) isValid() bool {
	return r.round != api.NoRoundID
}

type state struct {
	input api.Input
	rewind rewind
	pending bool
	active bool
	inputSeq int
	inputBytes int
	uploading int
	settled api.EventUsage
	unsettled int
	reported int
	tasks []api.Task
	compaction bool
	ask *api.ChatAsk
}

type Bar struct {
	Active bool
	Settled int
	Unsettled int
	Compaction bool
}

type Context struct {
	Usage api.EventUsage
	Compaction bool
}

type ComposerController struct {
	thread api.ChatThread
	origin doors.Source[state]
	reset doors.Beam[int]
	input doors.Beam[InputState]
	rewinding doors.Beam[api.RoundID]
	pending doors.Beam[bool]
	enabled doors.Beam[bool]
	active doors.Beam[bool]
	bar doors.Beam[Bar]
	hasContext doors.Beam[bool]
	context doors.Beam[Context]
	hasTasks doors.Beam[bool]
	taskCount doors.Beam[int]
	excessBytes doors.Beam[int]
	tasks doors.Beam[[]api.Task]
	inputLimit int
}

func (s ComposerController) FS() api.FS {
	return s.thread.FS()
}

func (s ComposerController) FlushInput(ctx context.Context) {
	s.origin.Mutate(ctx, func(d state) state {
		d.pending = false
		d.rewind = invalidRewind
		d.inputSeq += 1
		d.uploading = 0
		d.input = api.Input{}
		return d
	})
}

func (s ComposerController) CancelRewind(ctx context.Context) {
	s.origin.Mutate(ctx, func(s state) state {
		if !s.rewind.isValid() {
			return s
		}
		s.rewind = invalidRewind
		s.inputSeq += 1
		s.uploading = 0
		return s
	})
}

func (s ComposerController) InitRewind(ctx context.Context, round api.RoundID, input api.Input) {
	s.origin.Mutate(ctx, func(s state) state {
		s.rewind = rewind{
			round: round,
			input: input,
		}
		s.inputSeq += 1
		s.uploading = 0
		return s
	})
}

func (s ComposerController) Ask(ctx context.Context, ask api.ChatAsk) {
	s.origin.Mutate(ctx, func(s state) state {
		s.ask = &ask
		s.inputSeq += 1
		s.uploading = 0
		return s
	})
}

func (s ComposerController) AskDone(ctx context.Context) {
	s.origin.Mutate(ctx, func(s state) state {
		s.ask = nil
		s.inputSeq += 1
		return s
	})
}

func (s ComposerController) UpdatePending(ctx context.Context, value bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.pending = value
		return d
	})
}

func (s ComposerController) UpdateCompaction(ctx context.Context, active bool, usage api.EventUsage) {
	s.origin.Mutate(ctx, func(d state) state {
		d.compaction = active
		if usage.Total() != 0 {
			d.settled = usage
		}
		return d
	})
}

func (s ComposerController) UpdateActive(ctx context.Context, value bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.active = value
		if !value {
			d.unsettled = 0
			d.reported = 0
		}
		return d
	})
}

func (s ComposerController) UpdateUsage(ctx context.Context, usage api.EventUsage) {
	s.origin.Mutate(ctx, func(d state) state {
		if !d.active {
			return d
		}
		out := usage.Output + usage.Reasoning
		d.unsettled += out - d.reported
		d.reported = out
		return d
	})
}

func (s ComposerController) RoundDone(ctx context.Context, roundState api.RoundState, usage api.EventUsage) {
	s.origin.Mutate(ctx, func(d state) state {
		d.reported = 0
		if roundState == api.RoundOk {
			d.settled = usage
		}
		return d
	})
}

func (s ComposerController) UpdateTasks(ctx context.Context, tasks []api.Task) {
	s.origin.Mutate(ctx, func(d state) state {
		d.tasks = tasks
		return d
	})
}

func (s ComposerController) CancelTask(id uint) {
	for _, t := range s.origin.Get().tasks {
		if t.ID == id {
			t.Cancel(api.ErrCanceledByUser)
			return
		}
	}
}

func (s ComposerController) CancelTasks() {
	for _, t := range s.origin.Get().tasks {
		t.Cancel(api.ErrCanceledByUser)
	}
}

func (s ComposerController) Stop(ctx context.Context) {
	s.thread.Stop(ctx)
}

func (s ComposerController) Escape(ctx context.Context) {
	d := s.origin.Get()
	if d.rewind.isValid() {
		s.CancelRewind(ctx)
		return
	}
	if d.ask != nil {
		d.ask.Cancel(ctx)
		return
	}
	if d.pending {
		s.thread.Unsend(ctx)
		return
	}
	if d.active {
		s.thread.Stop(ctx)
		return
	}
	if len(d.tasks) != 0 {
		s.CancelTasks()
	}
}

func (s ComposerController) Send(ctx context.Context) {
	state := s.origin.Get()
	if state.uploading != 0 {
		return
	}
	if state.rewind.isValid() {
		if state.rewind.input.Len() == 0 {
			return
		}
		if state.inputBytes > s.inputLimit {
			return
		}
		s.thread.Rewind(ctx, state.rewind.round, state.rewind.input)
		return
	}
	if state.input.Len() == 0 || state.pending || state.inputBytes > s.inputLimit {
		return
	}
	s.thread.Send(ctx, state.input)
}

func (s ComposerController) UploadStarted(ctx context.Context, seq int) {
	s.origin.Mutate(ctx, func(d state) state {
		if d.inputSeq == seq {
			d.uploading++
		}
		return d
	})
}

func (s ComposerController) UploadFinished(ctx context.Context, seq int) {
	s.origin.Mutate(ctx, func(d state) state {
		if d.inputSeq == seq && d.uploading > 0 {
			d.uploading--
		}
		return d
	})
}

func (s ComposerController) Unsend(ctx context.Context) {
	s.thread.Unsend(ctx)
}

func (s ComposerController) Compact(ctx context.Context) {
	s.thread.Compact(ctx)
}

func (s ComposerController) UpdateAttachments(ctx context.Context, seq int, files []string) {
	send := false
	var input api.Input
	s.origin.Mutate(ctx, func(d state) state {
		if d.inputSeq != seq {
			return d
		}
		if d.rewind.isValid() {
			d.rewind.input.Attachments = files
		} else {
			d.input.Attachments = files
		}
		text := d.input.Text
		if d.rewind.isValid() {
			text = d.rewind.input.Text
		}
		d.inputBytes = 0
		for _, c := range (api.Input{Text: text, Attachments: files}).Content() {
			if t, ok := c.(api.ContentText); ok {
				d.inputBytes += len(t.Text)
			}
		}
		if d.rewind.isValid() || d.inputBytes > s.inputLimit {
			return d
		}
		input = d.input
		send = true
		return d
	})
	if !send {
		return
	}
	s.thread.Input(ctx, input)
}

func (s ComposerController) UpdateInputText(ctx context.Context, text string) {
	send := false
	var input api.Input
	s.origin.Mutate(ctx, func(d state) state {
		attachments := d.input.Attachments
		if d.rewind.isValid() {
			attachments = d.rewind.input.Attachments
		}
		d.inputBytes = 0
		for _, c := range (api.Input{Text: text, Attachments: attachments}).Content() {
			if t, ok := c.(api.ContentText); ok {
				d.inputBytes += len(t.Text)
			}
		}
		if d.inputBytes > s.inputLimit {
			return d
		}
		if d.rewind.isValid() {
			d.rewind.input.Text = text
			return d
		}
		d.input.Text = text
		input = d.input
		send = true
		return d
	})
	if !send {
		return
	}
	s.thread.Input(ctx, input)
}

func (s ComposerController) Pending() doors.Beam[bool] {
	return s.pending
}

func (s ComposerController) Active() doors.Beam[bool] {
	return s.active
}

func (s ComposerController) Bar() doors.Beam[Bar] {
	return s.bar
}

func (s ComposerController) ExcessBytes() doors.Beam[int] {
	return s.excessBytes
}

func (s ComposerController) HasContext() doors.Beam[bool] {
	return s.hasContext
}

func (s ComposerController) Context() doors.Beam[Context] {
	return s.context
}

func (s ComposerController) HasTasks() doors.Beam[bool] {
	return s.hasTasks
}

func (s ComposerController) TaskCount() doors.Beam[int] {
	return s.taskCount
}

func (s ComposerController) Tasks() doors.Beam[[]api.Task] {
	return s.tasks
}

func (s ComposerController) Model() api.Model {
	return s.thread.Model()
}

func (s ComposerController) Config() api.ThreadConfig {
	return s.thread.Config()
}

func (s ComposerController) BindAttach(el func(enabled bool) gox.Elem) gox.Elem {
	return s.pending.Bind(func(b bool) gox.Elem {
		return el(!b)
	})
}

func (s ComposerController) BindSend(el func(enabled bool) gox.Elem) gox.Elem {
	return s.enabled.Bind(el)
}

func (s ComposerController) Input() doors.Beam[InputState] {
	return s.input
}

func (s ComposerController) Rewind() doors.Beam[api.RoundID] {
	return s.rewinding
}
