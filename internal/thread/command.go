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

package thread

import (
	"context"
	"encoding/json"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/page"
	"neui.dev/internal/thread/response"
)

type Command interface {
	command()
}

func (c commandInit) command()            {}
func (c commandUnlink) command()          {}
func (c commandLink) command()            {}
func (c commandInput) command()           {}
func (c commandRecord) command()          {}
func (c commandRename) command()          {}
func (c commandThreadRename) command()    {}
func (c commandThreadDelete) command()    {}
func (c commandStop) command()            {}
func (c commandSend) command()            {}
func (c commandUnsend) command()          {}
func (c commandTask) command()            {}
func (c commandAsk) command()             {}
func (c commandAskDone) command()         {}
func (c commandRetry) command()           {}
func (c commandRewind) command()          {}
func (c commandTaskResult) command()      {}
func (c commandTaskNotify) command()      {}
func (c commandApplyCompaction) command() {}
func (c commandCompact) command()         {}

type commandLink struct {
	backend *common.Backend
	page    page.Page
	load    bool
	meta    any
	op      Operation
	ResultResponse[Link]
}

type commandUnlink struct {
	link *link
}

type commandInit struct {
	link *link
}

type Errorable interface {
	Err(error)
}

type commandInput struct {
	link  *link
	input api.Input
	Operation
}

type commandRecord struct {
	link   *link
	origin api.RoundID
	kind   string
	attrs  map[string]any
	data   any
	Operation
}

type commandRename struct {
	link *link
	name string
	Operation
}

type commandThreadRename struct {
	ctx     context.Context
	storage api.MessageStorage
	name    string
	Operation
}

type commandThreadDelete struct {
	ctx     context.Context
	storage api.MessageStorage
	Operation
}

type commandSend struct {
	link *link
	Operation
	input api.Input
}

type commandUnsend struct {
	link *link
	Operation
}

type commandStop struct {
	link *link
	Operation
}

type commandTask struct {
	linkTasks *linkTasks
	origin    api.RoundID
	module    string
	name      string
	attrs     map[string]any
	comment   string
	cancel    context.CancelCauseFunc
	ResultResponse[common.Task]
}

type commandAsk struct {
	link *link
	ask  *api.ChatAsk
	ResultResponse[struct{}]
}

type commandAskDone struct {
	ask *api.ChatAsk
	Operation
}

type commandTaskResult struct {
	linkTasks *linkTasks
	id        uint
	output    any
	err       error
}

type commandTaskNotify struct {
	linkTasks *linkTasks
	id        uint
	data      json.RawMessage
	meta      map[string]any
}

type commandRetry struct {
	link *link
	Operation
}

type commandRewind struct {
	link  *link
	round api.RoundID
	input api.Input
	Operation
}

type commandCompact struct {
	link *link
	Operation
}

type commandApplyCompaction struct {
	resp response.Response
	seq  int
}
