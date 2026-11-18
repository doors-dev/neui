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

package ui

import (
	"context"
	"time"

	"github.com/doors-dev/doors"
	"neui.dev/api"
)

type State struct {
	state   doors.Source[api.InstanceState]
	enabled doors.Beam[bool]
}

func (s State) Beam() doors.Beam[api.InstanceState] {
	return s.state
}

func (s State) Enabled() doors.Beam[bool] {
	return s.enabled
}

func (s State) Disable(ctx context.Context) {
	s.state.Update(ctx, api.InstanceState{Status: api.InstanceDisabled})
}

func (s State) Enable(ctx context.Context) {
	s.state.Update(ctx, api.InstanceState{Status: api.InstanceLoading})
}

func (s State) Open(ctx context.Context, thread api.InstanceThread) {
	s.state.Update(ctx, api.InstanceState{
		Status: api.InstanceEnabled,
		Thread: thread,
	})
}

func (s State) Create(ctx context.Context) {
	s.state.Mutate(ctx, func(s api.InstanceState) api.InstanceState {
		s.Thread.New = false
		return s
	})
}

func (s State) Touch(ctx context.Context, at time.Time) {
	s.state.Mutate(ctx, func(s api.InstanceState) api.InstanceState {
		s.Thread.Updated = at
		return s
	})
}

func (s State) Rename(ctx context.Context, name string) {
	s.state.Mutate(ctx, func(s api.InstanceState) api.InstanceState {
		s.Thread.Name = name
		return s
	})
}
