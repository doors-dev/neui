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
	"testing"
	"time"
)

func TestCacheExpires(t *testing.T) {
	c := newCache(40 * time.Millisecond)
	c.put(1, Schema[any]{raw: []byte("1")})
	c.put(2, Schema[any]{raw: []byte("2")})
	if _, ok := c.get(1); !ok {
		t.Fatal("fresh entry missing")
	}
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.get(1); ok {
		t.Fatal("entry survived its ttl")
	}
	if _, ok := c.get(2); ok {
		t.Fatal("entry survived its ttl")
	}
	c.mu.Lock()
	timer, n := c.timer, len(c.heap)
	c.mu.Unlock()
	if timer != nil || n != 0 {
		t.Fatalf("timer %v, heap %d after draining", timer != nil, n)
	}
	c.put(3, Schema[any]{raw: []byte("3")})
	time.Sleep(60 * time.Millisecond)
	if _, ok := c.get(3); ok {
		t.Fatal("entry added after drain survived its ttl")
	}
}

func TestCacheTouchSlides(t *testing.T) {
	c := newCache(60 * time.Millisecond)
	c.put(1, Schema[any]{raw: []byte("1")})
	c.put(2, Schema[any]{raw: []byte("2")})
	for range 4 {
		time.Sleep(25 * time.Millisecond)
		if _, ok := c.get(1); !ok {
			t.Fatal("touched entry expired")
		}
	}
	if _, ok := c.get(2); ok {
		t.Fatal("untouched entry survived its ttl")
	}
	c.mu.Lock()
	head := c.heap.Head()
	c.mu.Unlock()
	if head == nil || head.hash != 1 {
		t.Fatal("touched entry is not the remaining head")
	}
}
