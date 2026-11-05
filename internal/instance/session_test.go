package instance

import (
	"context"
	"path/filepath"
	"testing"
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
