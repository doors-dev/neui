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

package schema

import (
	"container/heap"
	"sync"
	"time"
)

type entry struct {
	hash   uint64
	schema Schema[any]
	expire time.Time
	index  int
}

type entries []*entry

func (h entries) Head() *entry {
	if len(h) == 0 {
		return nil
	}
	return h[0]
}

func (h entries) Len() int {
	return len(h)
}

func (h entries) Less(i, j int) bool {
	return h[i].expire.Before(h[j].expire)
}

func (h entries) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
	h[i].index = i
	h[j].index = j
}

func (h *entries) Push(x any) {
	e := x.(*entry)
	e.index = len(*h)
	*h = append(*h, e)
}

func (h *entries) Pop() any {
	old := *h
	n := len(old)
	x := old[n-1]
	old[n-1] = nil
	*h = old[:n-1]
	return x
}

func newCache(ttl time.Duration) *cache {
	return &cache{
		ttl:    ttl,
		lookup: make(map[uint64]*entry),
	}
}

type cache struct {
	mu     sync.Mutex
	ttl    time.Duration
	lookup map[uint64]*entry
	heap   entries
	timer  *time.Timer
}

func (c *cache) get(hash uint64) (Schema[any], bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.lookup[hash]
	if !ok {
		return Schema[any]{}, false
	}
	c.touch(e)
	return e.schema, true
}

func (c *cache) put(hash uint64, schema Schema[any]) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if e, ok := c.lookup[hash]; ok {
		c.touch(e)
		return
	}
	e := &entry{
		hash:   hash,
		schema: schema,
		expire: time.Now().Add(c.ttl),
	}
	c.lookup[hash] = e
	heap.Push(&c.heap, e)
	if e.index == 0 {
		c.newHead()
	}
}

func (c *cache) touch(e *entry) {
	e.expire = time.Now().Add(c.ttl)
	first := e.index == 0
	heap.Fix(&c.heap, e.index)
	if first {
		c.newHead()
	}
}

func (c *cache) newHead() {
	if c.timer != nil && !c.timer.Stop() {
		return
	}
	d := time.Until(c.heap.Head().expire)
	if c.timer == nil {
		c.timer = time.AfterFunc(d, c.expire)
		return
	}
	c.timer.Reset(d)
}

func (c *cache) expire() {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	for e := c.heap.Head(); e != nil && !e.expire.After(now); e = c.heap.Head() {
		heap.Pop(&c.heap)
		delete(c.lookup, e.hash)
	}
	e := c.heap.Head()
	if e == nil {
		c.timer = nil
		return
	}
	c.timer.Reset(time.Until(e.expire))
}
