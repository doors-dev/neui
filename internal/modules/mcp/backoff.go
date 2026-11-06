package mcp

import (
	"context"
	"math/rand/v2"
	"sync"
	"time"
)

type backoff struct {
	mu    sync.Mutex
	delay time.Duration
}

func (b *backoff) wait(ctx context.Context) bool {
	b.mu.Lock()
	if b.delay == 0 {
		b.delay = time.Second
	}
	delay := b.delay
	b.delay = min(b.delay*2, 30*time.Second)
	b.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay + rand.N(delay/2)):
		return true
	}
}

func (b *backoff) reset() {
	b.mu.Lock()
	b.delay = 0
	b.mu.Unlock()
}
