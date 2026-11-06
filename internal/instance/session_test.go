package instance

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestEnableRetriesLocalFileStorage(t *testing.T) {
	t.Setenv("TMPDIR", filepath.Join(t.TempDir(), "missing"))
	s := &session{ctx: context.Background(), instances: make(map[*instance]struct{})}
	if err := s.Enable(nil); err == nil {
		t.Fatal("Enable succeeded without a temp dir")
	}
	if s.backend != nil || s.localFileStorage != nil {
		t.Fatal("a failed Enable kept a backend or a file storage")
	}
	t.Setenv("TMPDIR", t.TempDir())
	s.Enable(nil)
	if s.localFileStorage == nil {
		t.Fatal("a later Enable did not create the file storage")
	}
}

func TestSessionEndRemovesTheLocalFileStorage(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("TMPDIR", dir)
	ctx, cancel := context.WithCancel(context.Background())
	s := &session{ctx: ctx, instances: make(map[*instance]struct{})}
	s.Enable(nil)
	if entries, _ := os.ReadDir(dir); s.localFileStorage == nil || len(entries) == 0 {
		t.Fatal("Enable did not create the file storage")
	}
	cancel()
	deadline := time.Now().Add(5 * time.Second)
	for {
		entries, err := os.ReadDir(dir)
		if err != nil {
			t.Fatalf("ReadDir: %v", err)
		}
		if len(entries) == 0 {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("the session ended but its file storage is still there: %v", entries)
		}
		time.Sleep(10 * time.Millisecond)
	}
}
