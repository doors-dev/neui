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
