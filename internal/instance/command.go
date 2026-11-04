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
