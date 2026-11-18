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
	"maps"

	"github.com/doors-dev/doors"
	"neui.dev/api"
)

func NewChats() Chats {
	chats := doors.NewSourceEqual[map[string]api.InstanceState](nil, func(old, new map[string]api.InstanceState) bool {
		return maps.Equal(old, new)
	})
	return Chats{
		chats: chats,
		all: doors.DeriveBeamEqual(chats, func(m map[string]api.InstanceState) map[api.ThreadID]api.InstanceChat {
			out := make(map[api.ThreadID]api.InstanceChat, len(m))
			for instance, state := range m {
				if state.Thread.ID == "" {
					continue
				}
				out[state.Thread.ID] = api.InstanceChat{
					InstanceID: instance,
					State:      state,
				}
			}
			return out
		}, func(old, new map[api.ThreadID]api.InstanceChat) bool {
			return maps.Equal(old, new)
		}),
	}
}

type Chats struct {
	chats doors.Source[map[string]api.InstanceState]
	all   doors.Beam[map[api.ThreadID]api.InstanceChat]
}

func (c Chats) Beam() doors.Beam[map[api.ThreadID]api.InstanceChat] {
	return c.all
}

func (c Chats) Add(ctx context.Context, instance string, state api.InstanceState) State {
	c.chats.Mutate(ctx, func(m map[string]api.InstanceState) map[string]api.InstanceState {
		if _, ok := m[instance]; ok {
			return m
		}
		out := make(map[string]api.InstanceState, len(m)+1)
		maps.Copy(out, m)
		out[instance] = state
		return out
	})
	source := doors.DeriveSource(c.chats, func(m map[string]api.InstanceState) api.InstanceState {
		return m[instance]
	}, func(m map[string]api.InstanceState, state api.InstanceState) map[string]api.InstanceState {
		if _, ok := m[instance]; !ok {
			return m
		}
		out := make(map[string]api.InstanceState, len(m))
		maps.Copy(out, m)
		out[instance] = state
		return out
	})
	return State{
		state: source,
		enabled: doors.DeriveBeam(source, func(s api.InstanceState) bool {
			return s.Status != api.InstanceDisabled
		}),
	}
}

func (c Chats) Remove(ctx context.Context, instance string) {
	c.chats.Mutate(ctx, func(m map[string]api.InstanceState) map[string]api.InstanceState {
		if _, ok := m[instance]; !ok {
			return m
		}
		out := make(map[string]api.InstanceState, len(m)-1)
		for id, s := range m {
			if id == instance {
				continue
			}
			out[id] = s
		}
		return out
	})
}
