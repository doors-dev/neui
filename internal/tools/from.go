package tools

import (
	"context"
	"sync/atomic"
)

type fromKey struct{}

type runKey struct{}

var runs atomic.Uint64

func WithRun(ctx context.Context) context.Context {
	return context.WithValue(ctx, runKey{}, runs.Add(1))
}

func RunID(ctx context.Context) uint64 {
	id, _ := ctx.Value(runKey{}).(uint64)
	return id
}

type from int

const (
	fromAgent from = iota
	fromJS
)

func FromJS(ctx context.Context) bool {
	from, ok := ctx.Value(fromKey{}).(from)
	if !ok {
		return false
	}
	return from == fromJS
}

func FromAgent(ctx context.Context) bool {
	from, ok := ctx.Value(fromKey{}).(from)
	if !ok {
		return false
	}
	return from == fromAgent
}

func Attach(ctx context.Context, source context.Context) context.Context {
	f, ok := source.Value(fromKey{}).(from)
	if !ok {
		return ctx
	}
	return context.WithValue(ctx, fromKey{}, f)
}

func makeFromJS(ctx context.Context) context.Context {
	from, ok := ctx.Value(fromKey{}).(from)
	if ok && from == fromJS {
		return ctx
	}
	return context.WithValue(ctx, fromKey{}, fromJS)
}

func makeFromAgent(ctx context.Context) context.Context {
	from, ok := ctx.Value(fromKey{}).(from)
	if ok && from == fromAgent {
		return ctx
	}
	return context.WithValue(ctx, fromKey{}, fromAgent)
}
