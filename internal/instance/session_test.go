package instance

import (
	"context"
	"errors"
	"testing"
)

func TestEnableFailsWithoutLocalFileStorage(t *testing.T) {
	cause := errors.New("no temp dir")
	s := &session{ctx: context.Background(), fileStorageErr: cause, instances: make(map[*instance]struct{})}
	err := s.Enable(nil)
	if !errors.Is(err, cause) {
		t.Fatalf("Enable returned %v, want the file storage error", err)
	}
	if s.backend != nil {
		t.Fatal("Enable built a backend without file storage")
	}
}
