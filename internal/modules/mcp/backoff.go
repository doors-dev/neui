package mcp

import (
	"context"
	"math/rand/v2"
	"time"
)

type backoff struct {
	ctx   context.Context
	base  time.Duration
	limit time.Duration
	delay time.Duration
}

func newBackoff(ctx context.Context, base time.Duration, limit time.Duration) *backoff {
	return &backoff{ctx: ctx, base: base, limit: limit}
}

func (b *backoff) wait() bool {
	if b.delay == 0 {
		b.delay = b.base
	}
	select {
	case <-b.ctx.Done():
		return false
	case <-time.After(b.delay + rand.N(b.delay/2)):
	}
	b.delay = min(b.delay*2, b.limit)
	return true
}

func (b *backoff) reset() {
	b.delay = 0
}
