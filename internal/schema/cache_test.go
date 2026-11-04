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
