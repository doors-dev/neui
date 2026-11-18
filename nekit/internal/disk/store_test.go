package disk

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"neui.dev/api"
)

func open(t *testing.T, path string) *Store {
	t.Helper()
	s, err := New(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func read(t *testing.T, s *Store, origin api.Origin, name string) string {
	t.Helper()
	rc, err := s.FileRead(context.Background(), "t1", origin, name, 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

type gateReader struct {
	started chan struct{}
	release chan struct{}
	once    sync.Once
	body    io.Reader
}

func newGate(body string) *gateReader {
	return &gateReader{started: make(chan struct{}), release: make(chan struct{}), body: strings.NewReader(body)}
}

func (g *gateReader) Read(p []byte) (int, error) {
	g.once.Do(func() {
		close(g.started)
		<-g.release
	})
	return g.body.Read(p)
}

func TestNewRefusesUnwritablePaths(t *testing.T) {
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := New(file); err == nil {
		t.Fatal("New on a file succeeded")
	}
	if os.Geteuid() == 0 {
		return
	}
	locked := filepath.Join(t.TempDir(), "locked")
	if err := os.Mkdir(locked, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(locked, 0o755) })
	if _, err := New(locked); err == nil {
		t.Fatal("New on a read-only directory succeeded")
	}
}

func TestReopenClearsStaging(t *testing.T) {
	path := filepath.Join(t.TempDir(), "uploads")
	open(t, path)
	staging := filepath.Join(path, ".tmp")
	if err := os.MkdirAll(filepath.Join(staging, "left", "over"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(staging, "stray"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	open(t, path)
	if entries, err := os.ReadDir(staging); err != nil || len(entries) != 0 {
		t.Fatalf("staging = %v, %v, want empty", entries, err)
	}
}

func TestReopenKeepsFiles(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploads")
	s := open(t, path)
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "kept.txt", "text/plain", strings.NewReader("kept"), true); err != nil {
		t.Fatal(err)
	}
	if err := s.FileCommit(ctx, "t1", api.OriginUser, "kept.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "draft.txt", "text/plain", strings.NewReader("draft"), true); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginAgent, "out.txt", "text/plain", strings.NewReader("out"), false); err != nil {
		t.Fatal(err)
	}
	want := map[api.Origin][]api.File{}
	for _, origin := range []api.Origin{api.OriginUser, api.OriginAgent} {
		for f, err := range s.FileList(ctx, "t1", origin) {
			if err != nil {
				t.Fatal(err)
			}
			want[origin] = append(want[origin], f)
		}
	}
	r := open(t, path)
	for origin, files := range want {
		var got []api.File
		for f, err := range r.FileList(ctx, "t1", origin) {
			if err != nil {
				t.Fatal(err)
			}
			got = append(got, f)
		}
		if len(got) != len(files) {
			t.Fatalf("%s after reopen = %+v, want %+v", origin, got, files)
		}
		for i := range got {
			if !got[i].CreatedAt.Equal(files[i].CreatedAt) {
				t.Fatalf("%s CreatedAt = %v, want %v", got[i].Name, got[i].CreatedAt, files[i].CreatedAt)
			}
			got[i].CreatedAt = files[i].CreatedAt
			if got[i] != files[i] {
				t.Fatalf("after reopen %+v, want %+v", got[i], files[i])
			}
		}
	}
	if f, err := r.FileStat(ctx, "t1", api.OriginUser, "draft.txt"); err != nil || !f.Draft {
		t.Fatalf("draft after reopen = %+v, %v", f, err)
	}
	if f, err := r.FileStat(ctx, "t1", api.OriginUser, "kept.txt"); err != nil || f.Draft {
		t.Fatalf("committed after reopen = %+v, %v", f, err)
	}
	if got := read(t, r, api.OriginUser, "kept.txt"); got != "kept" {
		t.Fatalf("kept.txt = %q", got)
	}
	if got := read(t, r, api.OriginAgent, "out.txt"); got != "out" {
		t.Fatalf("out.txt = %q", got)
	}
}

func TestTwoStoresRacePut(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploads")
	stores := []*Store{open(t, path), open(t, path)}
	gates := []*gateReader{newGate("first"), newGate("second")}
	done := make(chan error, 2)
	for i, s := range stores {
		go func() {
			_, err := s.FilePut(ctx, "t1", api.OriginUser, "f.txt", "text/plain", gates[i], false)
			done <- err
		}()
	}
	for _, g := range gates {
		<-g.started
	}
	for _, g := range gates {
		close(g.release)
	}
	var won, lost int
	for range stores {
		switch err := <-done; {
		case err == nil:
			won++
		case errors.Is(err, api.ErrExists):
			lost++
		default:
			t.Fatalf("put = %v", err)
		}
	}
	if won != 1 || lost != 1 {
		t.Fatalf("won %d, lost %d, want one each", won, lost)
	}
	if got := read(t, stores[0], api.OriginUser, "f.txt"); got != "first" && got != "second" {
		t.Fatalf("body = %q", got)
	}
	if entries, err := os.ReadDir(filepath.Join(path, ".tmp")); err != nil || len(entries) != 0 {
		t.Fatalf("staging = %v, %v, want empty", entries, err)
	}
}

func TestCrashMidPutLeavesNameFree(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "uploads")
	crashed := open(t, path)
	gate := newGate("lost")
	done := make(chan error, 1)
	go func() {
		_, err := crashed.FilePut(ctx, "t1", api.OriginUser, "f.txt", "text/plain", gate, false)
		done <- err
	}()
	<-gate.started
	if entries, err := os.ReadDir(filepath.Join(path, ".tmp")); err != nil || len(entries) != 1 {
		t.Fatalf("staging mid-put = %v, %v, want one dir", entries, err)
	}
	s := open(t, path)
	if _, err := s.FileStat(ctx, "t1", api.OriginUser, "f.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat = %v, want ErrNotFound", err)
	}
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f.txt", "text/plain", strings.NewReader("again"), false); err != nil {
		t.Fatal(err)
	}
	close(gate.release)
	if err := <-done; err == nil {
		t.Fatal("put whose staging was cleared succeeded")
	}
	if got := read(t, s, api.OriginUser, "f.txt"); got != "again" {
		t.Fatalf("body = %q", got)
	}
}
