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

package mcp

import (
	"context"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type command interface {
	command()
}

func (connectCommand) command()    {}
func (disconnectCommand) command() {}
func (toolsCommand) command()      {}
func (getSessionCommand) command() {}
func (callDoneCommand) command()   {}
func (apiCommand) command()        {}
func (connectErrCommand) command() {}

type connectCommand struct {
	client *mcp.ClientSession
	desc   string
	tools  []*mcp.Tool
	seq    int
}

type disconnectCommand struct {
	seq int
}

type connectErrCommand struct {
	err error
	seq int
}

type toolsCommand struct {
	tools []*mcp.Tool
	err   error
	seq   int
}

type getSessionCommand struct {
	ctx context.Context
	ch  chan *mcp.ClientSession
}

func (c getSessionCommand) reply(client *mcp.ClientSession) bool {
	select {
	case c.ch <- client:
		return true
	case <-c.ctx.Done():
		return false
	}
}

type callDoneCommand struct{}

type apiCommand struct {
	f func(m *module)
}
