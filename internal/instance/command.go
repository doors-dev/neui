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

package instance

import (
	"context"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/thread"
)

type command interface {
	command()
}

func (c commandSwitch) command()    {}
func (c commandLink) command()      {}
func (c commandUnlink) command()    {}
func (c commandSetUplink) command() {}
func (c commandUplink) command()    {}
func (c commandDrop) command()      {}
func (c commandRecord) command()    {}

type commandSwitch struct {
	backend *common.Backend
}

type commandSetUplink struct {
	ctx     context.Context
	backend *common.Backend
	uplink  thread.Link
	thread.Operation
}

type commandDrop struct {
	ctx context.Context
	id  api.ThreadID
	thread.Operation
}

type commandLink struct {
	link *link
}

type commandUnlink struct {
	link *link
}

type commandUplink struct {
	link *link
	thread.Operation
	make func(uplink thread.Link, op thread.Operation) thread.Command
}

func (c commandUplink) makeCommand(uplink thread.Link) thread.Command {
	return c.make(uplink, c.Operation)
}

type commandRecord struct {
	kind  string
	attrs map[string]any
	data  any
	thread.Operation
}
