// Managed by GoX v0.4.0

//line controller.gox:1
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

package controller

import (
	"context"
	"slices"
	"unicode/utf8"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/internal/replay"
)

type InputState struct {
	Seq int
	Input api.Input
	Pending bool
	Rewind bool
	Ask *api.ChatAsk
}

func New(thread api.ChatThread, input api.ChatInput, active bool, settled api.EventUsage, tasks []api.Task, compaction bool, canCompact bool, ask *api.ChatAsk) Controller {
	limit := replay.RoundItemBudget(thread.Config(), thread.Model())
	origin := doors.NewSourceEqual(state{
		input: input.Input,
		textBytes: len(input.Input.Text),
		attachBytes: attachBytes(input.Input.Attachments),
		pending: input.Pending,
		rewind: invalidRewind,
		active: active,
		settled: settled,
		tasks: tasks,
		compaction: compaction,
		canCompact: canCompact,
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
		if old.textBytes != new.textBytes || old.attachBytes != new.attachBytes {
			return false
		}
		if old.settled != new.settled || old.unsettled != new.unsettled {
			return false
		}
		if old.compaction != new.compaction || old.canCompact != new.canCompact {
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
		if d.textBytes + d.attachBytes > limit {
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
		return Context{d.settled, d.compaction, d.canCompact}
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
		return max(d.textBytes + d.attachBytes - limit, 0)
	})
	return Controller{
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

func attachBytes(files []string) int {
	n := 0
	for _, c := range (api.Input{Attachments: files}).Content() {
		if t, ok := c.(api.ContentText); ok {
			n += len(t.Text)
		}
	}
	return n
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
	textBytes int
	attachBytes int
	uploading int
	settled api.EventUsage
	unsettled int
	reported int
	tasks []api.Task
	compaction bool
	canCompact bool
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
	CanCompact bool
}

type Controller struct {
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

func (s Controller) FS() api.FS {
	return s.thread.FS()
}

func (s Controller) FlushInput(ctx context.Context) {
	s.origin.Mutate(ctx, func(d state) state {
		d.pending = false
		d.input = api.Input{}
		if d.rewind.isValid() {
			return d
		}
		d.inputSeq += 1
		d.uploading = 0
		d.textBytes = 0
		d.attachBytes = 0
		return d
	})
}

func (s Controller) CancelRewind(ctx context.Context) {
	s.origin.Mutate(ctx, func(s state) state {
		if !s.rewind.isValid() {
			return s
		}
		s.rewind = invalidRewind
		s.inputSeq += 1
		s.uploading = 0
		s.textBytes = len(s.input.Text)
		s.attachBytes = attachBytes(s.input.Attachments)
		return s
	})
}

func (s Controller) InitRewind(ctx context.Context, round api.RoundID, input api.Input) {
	s.origin.Mutate(ctx, func(s state) state {
		s.rewind = rewind{
			round: round,
			input: input,
		}
		s.inputSeq += 1
		s.uploading = 0
		s.textBytes = len(input.Text)
		s.attachBytes = attachBytes(input.Attachments)
		return s
	})
}

func (s Controller) Ask(ctx context.Context, ask api.ChatAsk) {
	s.origin.Mutate(ctx, func(s state) state {
		s.ask = &ask
		s.inputSeq += 1
		s.uploading = 0
		return s
	})
}

func (s Controller) AskDone(ctx context.Context) {
	s.origin.Mutate(ctx, func(s state) state {
		s.ask = nil
		s.inputSeq += 1
		return s
	})
}

func (s Controller) UpdatePending(ctx context.Context, value bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.pending = value
		return d
	})
}

func (s Controller) UpdateCompaction(ctx context.Context, active bool, usage api.EventUsage, canCompact bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.compaction = active
		d.canCompact = canCompact
		if usage.Total() != 0 {
			d.settled = usage
		}
		return d
	})
}

func (s Controller) UpdateActive(ctx context.Context, value bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.active = value
		if !value {
			d.unsettled = 0
			d.reported = 0
		}
		return d
	})
}

func (s Controller) RoundStart(ctx context.Context, usage api.EventUsage, canCompact bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.active = true
		d.settled = usage
		d.canCompact = canCompact
		return d
	})
}

func (s Controller) UpdateUsage(ctx context.Context, usage api.EventUsage) {
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

func (s Controller) RoundDone(ctx context.Context, roundState api.RoundState, usage api.EventUsage, canCompact bool) {
	s.origin.Mutate(ctx, func(d state) state {
		d.reported = 0
		d.canCompact = canCompact
		if roundState == api.RoundOk {
			d.settled = usage
		}
		return d
	})
}

func (s Controller) UpdateTasks(ctx context.Context, tasks []api.Task) {
	s.origin.Mutate(ctx, func(d state) state {
		d.tasks = tasks
		return d
	})
}

func (s Controller) CancelTask(id uint) {
	for _, t := range s.origin.Get().tasks {
		if t.ID == id {
			t.Cancel(api.ErrCanceledByUser)
			return
		}
	}
}

func (s Controller) CancelTasks() {
	for _, t := range s.origin.Get().tasks {
		t.Cancel(api.ErrCanceledByUser)
	}
}

func (s Controller) Stop(ctx context.Context) {
	s.thread.Stop(ctx)
}

func (s Controller) Escape(ctx context.Context, fromComposer bool) {
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
	if !fromComposer {
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

func (s Controller) Send(ctx context.Context) {
	state := s.origin.Get()
	if state.uploading != 0 {
		return
	}
	if state.rewind.isValid() {
		if state.rewind.input.Len() == 0 {
			return
		}
		if state.textBytes + state.attachBytes > s.inputLimit {
			return
		}
		s.thread.Rewind(ctx, state.rewind.round, state.rewind.input)
		return
	}
	if state.input.Len() == 0 || state.pending || state.textBytes + state.attachBytes > s.inputLimit {
		return
	}
	s.thread.Send(ctx, state.input)
}

func (s Controller) UploadStarted(ctx context.Context, seq int) {
	s.origin.Mutate(ctx, func(d state) state {
		if d.inputSeq == seq {
			d.uploading++
		}
		return d
	})
}

func (s Controller) UploadFinished(ctx context.Context, seq int) {
	s.origin.Mutate(ctx, func(d state) state {
		if d.inputSeq == seq && d.uploading > 0 {
			d.uploading--
		}
		return d
	})
}

func (s Controller) Unsend(ctx context.Context) {
	s.thread.Unsend(ctx)
}

func (s Controller) Compact(ctx context.Context) {
	s.thread.Compact(ctx)
}

func (s Controller) UpdateAttachments(ctx context.Context, seq int, files []string) {
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
		d.attachBytes = attachBytes(files)
		if d.rewind.isValid() || d.textBytes + d.attachBytes > s.inputLimit {
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

func (s Controller) UpdateInputText(ctx context.Context, text string) {
	send := false
	var input api.Input
	s.origin.Mutate(ctx, func(d state) state {
		d.textBytes = len(text)
		cut := text
		if len(cut) > s.inputLimit {
			n := s.inputLimit
			for n > 0 && !utf8.RuneStart(cut[n]) {
				n--
			}
			cut = cut[:n]
		}
		if d.rewind.isValid() {
			d.rewind.input.Text = cut
			return d
		}
		d.input.Text = cut
		if d.textBytes + d.attachBytes > s.inputLimit {
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

func (s Controller) Pending() doors.Beam[bool] {
	return s.pending
}

func (s Controller) Active() doors.Beam[bool] {
	return s.active
}

func (s Controller) Bar() doors.Beam[Bar] {
	return s.bar
}

func (s Controller) ExcessBytes() doors.Beam[int] {
	return s.excessBytes
}

func (s Controller) HasContext() doors.Beam[bool] {
	return s.hasContext
}

func (s Controller) Context() doors.Beam[Context] {
	return s.context
}

func (s Controller) HasTasks() doors.Beam[bool] {
	return s.hasTasks
}

func (s Controller) TaskCount() doors.Beam[int] {
	return s.taskCount
}

func (s Controller) Tasks() doors.Beam[[]api.Task] {
	return s.tasks
}

func (s Controller) Model() api.Model {
	return s.thread.Model()
}

func (s Controller) Config() api.ThreadConfig {
	return s.thread.Config()
}

func (s Controller) BindSend(el func(enabled bool) gox.Elem) gox.Elem {
	return s.enabled.Bind(el)
}

func (s Controller) Input() doors.Beam[InputState] {
	return s.input
}

func (s Controller) Rewind() doors.Beam[api.RoundID] {
	return s.rewinding
}
