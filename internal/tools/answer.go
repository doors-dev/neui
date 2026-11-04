package tools

import (
	"context"

	"neui.dev/api"
)

func newAnswer[O any](ctx context.Context) answer[O] {
	ch := make(chan replyResult[O])
	ctx, cancel := context.WithCancel(ctx)
	return answer[O]{
		ch:        ch,
		ctx:       ctx,
		ctxCancel: cancel,
	}
}

type replyResult[O any] struct {
	ctx    context.Context
	output O
	err    error
}

type answer[O any] struct {
	ch        chan replyResult[O]
	ctx       context.Context
	ctxCancel context.CancelFunc
}

func (r answer[O]) Fail(ctx context.Context, err error) {
	select {
	case r.ch <- replyResult[O]{
		ctx: ctx,
		err: err,
	}:
	case <-r.ctx.Done():
	}
}

func (r answer[O]) Submit(ctx context.Context, output O) {
	select {
	case r.ch <- replyResult[O]{
		ctx:    ctx,
		output: output,
	}:
	case <-r.ctx.Done():
	}
}

func (r answer[O]) cancel(ctx context.Context) {
	select {
	case r.ch <- replyResult[O]{
		ctx: ctx,
		err: api.ErrCanceledByUser,
	}:
	case <-r.ctx.Done():
	}
}

func (r answer[O]) receive() (O, context.Context, error) {
	defer r.ctxCancel()
	select {
	case <-r.ctx.Done():
		var o O
		return o, context.Background(), context.Cause(r.ctx)
	case r := <-r.ch:
		return r.output, r.ctx, r.err
	}
}
