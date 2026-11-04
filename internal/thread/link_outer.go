package thread

import (
	"neui.dev/api"
)

type Link struct {
	inner *link
	state api.InstanceThread
}

func (l Link) Valid() bool {
	return l.inner != nil
}

func (l Link) ID() api.ThreadID {
	return l.inner.thread.id
}

func (l Link) Thread() api.InstanceThread {
	return l.state
}

func (l Link) Receiver() (receive <-chan Message, done <-chan struct{}) {
	return l.inner.ch, l.inner.linkCtx.Done()
}

func (l Link) Sender() chan<- Command {
	return l.inner.thread.ch
}

func (l Link) Close() {
	if l.inner == nil {
		return
	}
	l.inner.linkCancel()
	select {
	case <-l.inner.thread.close:
	case l.inner.thread.ch <- commandUnlink{l.inner}:
	}
	l.inner = nil
}

func (l Link) Input(op Operation, input api.Input) Command {
	return commandInput{l.inner, input, op}
}

func (l Link) Record(op Operation, kind string, attrs map[string]any, data any) Command {
	return commandRecord{l.inner, api.NoRoundID, kind, attrs, data, op}
}

func (l Link) Stop(op Operation) Command {
	return commandStop{l.inner, op}
}

func (l Link) Send(op Operation, input api.Input) Command {
	return commandSend{l.inner, op, input}
}

func (l Link) Unsend(op Operation) Command {
	return commandUnsend{l.inner, op}
}

func (l Link) Retry(op Operation) Command {
	return commandRetry{l.inner, op}
}

func (l Link) Compact(op Operation) Command {
	return commandCompact{l.inner, op}
}

func (l Link) Rewind(op Operation, round api.RoundID, input api.Input) Command {
	return commandRewind{l.inner, round, input, op}
}

func (l Link) Init() Command {
	return commandInit{l.inner}
}
