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
	b.mu.Unlock()
	select {
	case <-ctx.Done():
		return false
	case <-time.After(delay + rand.N(delay/2)):
		b.mu.Lock()
		b.delay = min(b.delay*2, 30*time.Second)
		b.mu.Unlock()
		return true
	}
}

func (b *backoff) reset() {
	b.mu.Lock()
	b.delay = 0
	b.mu.Unlock()
}
