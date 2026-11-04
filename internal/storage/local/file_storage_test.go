package local

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neui.dev/api"
)

func newFileStore(t *testing.T) *FileStorage {
	t.Helper()
	s, err := NewFileStorage()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func putText(t *testing.T, s api.FileStorage, thread api.ThreadID, name, body string) api.File {
	t.Helper()
	f, err := s.FilePut(context.Background(), thread, api.OriginUser, name, "text/plain", strings.NewReader(body), false)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func readAll(t *testing.T, rc io.ReadCloser) string {
	t.Helper()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	if err := rc.Close(); err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestFilePutRoundTrip(t *testing.T) {
	s := newFileStore(t)
	f := putText(t, s, "t1", "f1.txt", "hello")
	if f.Size != 5 || f.CreatedAt.IsZero() {
		t.Fatalf("file = %+v", f)
	}
	got, err := s.FileStat(context.Background(), "t1", api.OriginUser, "f1.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got.ThreadID != "t1" || got.Name != "f1.txt" || got.MIME != "text/plain" || got.Origin != api.OriginUser || got.Size != 5 {
		t.Fatalf("stat = %+v", got)
	}
	if !got.CreatedAt.Equal(f.CreatedAt) {
		t.Fatalf("CreatedAt = %v, want %v", got.CreatedAt, f.CreatedAt)
	}
	rc, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if body := readAll(t, rc); body != "hello" {
		t.Fatalf("body = %q", body)
	}
}

func TestFilePutDuplicate(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "f1.txt", "original")
	_, err := s.FilePut(context.Background(), "t1", api.OriginUser, "f1.txt", "", strings.NewReader("clobber"), false)
	if !errors.Is(err, api.ErrExists) {
		t.Fatalf("err = %v, want ErrExists", err)
	}
	rc, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if body := readAll(t, rc); body != "original" {
		t.Fatalf("body = %q, want original untouched", body)
	}
}

func TestFileOriginsSeparate(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "f1.txt", "user side")
	if _, err := s.FilePut(context.Background(), "t1", api.OriginAgent, "f1.txt", "", strings.NewReader("agent side"), false); err != nil {
		t.Fatal(err)
	}
	rc, err := s.FileRead(context.Background(), "t1", api.OriginAgent, "f1.txt", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if body := readAll(t, rc); body != "agent side" {
		t.Fatalf("body = %q", body)
	}
	if err := s.FileDelete(context.Background(), "t1", api.OriginAgent, "f1.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatalf("user side gone with agent delete: %v", err)
	}
}

func TestFilePutInvalid(t *testing.T) {
	s := newFileStore(t)
	cases := []struct {
		thread api.ThreadID
		origin api.Origin
		name   string
	}{
		{"", api.OriginUser, "f1.txt"},
		{"t1", api.OriginUser, ""},
		{"t1", api.OriginUser, "a/b.txt"},
		{"t1", api.OriginUser, "."},
		{"t1", api.OriginUser, ".."},
		{"t1", api.Origin("smuggled"), "f1.txt"},
		{"t1", api.Origin(""), "f1.txt"},
	}
	for _, c := range cases {
		_, err := s.FilePut(context.Background(), c.thread, c.origin, c.name, "", strings.NewReader("x"), false)
		if err == nil {
			t.Fatalf("FilePut(%+v) succeeded", c)
		}
		if errors.Is(err, api.ErrExists) || errors.Is(err, api.ErrNotFound) {
			t.Fatalf("FilePut(%+v) = %v, want a plain error", c, err)
		}
	}
}

func TestFileZeroByte(t *testing.T) {
	s := newFileStore(t)
	f := putText(t, s, "t1", "empty", "")
	if f.Size != 0 {
		t.Fatalf("Size = %d", f.Size)
	}
	rc, err := s.FileRead(context.Background(), "t1", api.OriginUser, "empty", 0, -1)
	if err != nil {
		t.Fatal(err)
	}
	if body := readAll(t, rc); body != "" {
		t.Fatalf("body = %q", body)
	}
}

func TestFileReadRange(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "f1.txt", "hello world")
	cases := []struct {
		off, n int64
		want   string
	}{
		{0, -1, "hello world"},
		{6, -1, "world"},
		{0, 5, "hello"},
		{6, 5, "world"},
		{6, 100, "world"},
		{10, 1, "d"},
	}
	for _, c := range cases {
		rc, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", c.off, c.n)
		if err != nil {
			t.Fatalf("FileRead(%d, %d) = %v", c.off, c.n, err)
		}
		if body := readAll(t, rc); body != c.want {
			t.Fatalf("FileRead(%d, %d) = %q, want %q", c.off, c.n, body, c.want)
		}
	}
	for _, c := range []struct{ off, n int64 }{{11, -1}, {11, 1}, {100, 5}} {
		if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", c.off, c.n); !errors.Is(err, api.ErrOutOfRange) {
			t.Fatalf("FileRead(%d, %d) = %v, want ErrOutOfRange", c.off, c.n, err)
		}
	}
	if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", -3, 5); err == nil || errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("negative off = %v, want a plain error", err)
	}
	rc0, err := s.FileRead(context.Background(), "t1", api.OriginUser, "f1.txt", 3, 0)
	if err != nil {
		t.Fatalf("zero n = %v", err)
	}
	if body := readAll(t, rc0); body != "" {
		t.Fatalf("zero n body = %q", body)
	}
	putText(t, s, "t1", "empty", "")
	rc, err := s.FileRead(context.Background(), "t1", api.OriginUser, "empty", 0, -1)
	if err != nil {
		t.Fatalf("empty whole read = %v", err)
	}
	if body := readAll(t, rc); body != "" {
		t.Fatalf("empty body = %q", body)
	}
	if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "empty", 0, 1); !errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("empty ranged read = %v, want ErrOutOfRange", err)
	}
}

func TestFileMissing(t *testing.T) {
	s := newFileStore(t)
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "ghost"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat = %v, want ErrNotFound", err)
	}
	if _, err := s.FileRead(context.Background(), "t1", api.OriginUser, "ghost", 0, -1); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileRead = %v, want ErrNotFound", err)
	}
	if err := s.FileDelete(context.Background(), "t1", api.OriginUser, "ghost"); err != nil {
		t.Fatalf("FileDelete = %v, want nil", err)
	}
	if err := s.FileDeleteAll(context.Background(), "ghost-thread"); err != nil {
		t.Fatalf("FileDeleteAll = %v, want nil", err)
	}
}

func TestFileDelete(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "f1.txt", "hello")
	if err := s.FileDelete(context.Background(), "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "f1.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat after FileDelete = %v, want ErrNotFound", err)
	}
	putText(t, s, "t2", "f1.txt", "hello")
	if err := s.FileDeleteAll(context.Background(), "t2"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.FileStat(context.Background(), "t2", api.OriginUser, "f1.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat after FileDeleteAll = %v, want ErrNotFound", err)
	}
}

func collect(t *testing.T, s api.FileStorage, thread api.ThreadID, origin api.Origin) ([]api.File, []error) {
	t.Helper()
	var files []api.File
	var errs []error
	for f, err := range s.FileList(context.Background(), thread, origin) {
		if err != nil {
			errs = append(errs, err)
			continue
		}
		files = append(files, f)
	}
	return files, errs
}

func TestFileListSorted(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "mango", "3")
	putText(t, s, "t1", "zebra", "1")
	putText(t, s, "t1", "apple", "2")
	files, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 0 {
		t.Fatalf("errs = %v", errs)
	}
	if len(files) != 3 || files[0].Name != "apple" || files[1].Name != "mango" || files[2].Name != "zebra" {
		t.Fatalf("files = %+v, want lexical name order", files)
	}
}

func TestFileListEmptyAndMissing(t *testing.T) {
	s := newFileStore(t)
	if files, errs := collect(t, s, "never-seen", api.OriginUser); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("missing thread: files = %v, errs = %v", files, errs)
	}
	putText(t, s, "t1", "f1.txt", "x")
	if files, errs := collect(t, s, "t1", api.OriginAgent); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("missing origin: files = %v, errs = %v", files, errs)
	}
	if err := s.FileDelete(context.Background(), "t1", api.OriginUser, "f1.txt"); err != nil {
		t.Fatal(err)
	}
	if files, errs := collect(t, s, "t1", api.OriginUser); len(files) != 0 || len(errs) != 0 {
		t.Fatalf("emptied origin: files = %v, errs = %v", files, errs)
	}
}

func TestFileListCorruptMeta(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "good1", "a")
	putText(t, s, "t1", "good2", "b")
	putText(t, s, "t1", "bad", "c")
	base := s.base
	if err := os.WriteFile(filepath.Join(base, "files", "t1", "user", "bad", ".meta"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 1 {
		t.Fatalf("errs = %v, want exactly one", errs)
	}
	if len(files) != 2 || files[0].Name != "good1" || files[1].Name != "good2" {
		t.Fatalf("files = %+v, want good1, good2", files)
	}
}

func TestFileMetalessDirInvisible(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "f1.txt", "x")
	base := s.base
	if err := os.MkdirAll(filepath.Join(base, "files", "t1", "user", "ghost"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(base, "files", "t1", "user", "ghost", "data"), []byte("stray"), 0o644); err != nil {
		t.Fatal(err)
	}
	files, errs := collect(t, s, "t1", api.OriginUser)
	if len(errs) != 0 || len(files) != 1 || files[0].Name != "f1.txt" {
		t.Fatalf("files = %+v, errs = %v, want only f1.txt", files, errs)
	}
	if _, err := s.FileStat(context.Background(), "t1", api.OriginUser, "ghost"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("FileStat ghost = %v, want ErrNotFound", err)
	}
}

func TestFileCancelledContext(t *testing.T) {
	s := newFileStore(t)
	putText(t, s, "t1", "f1.txt", "x")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := s.FilePut(ctx, "t1", api.OriginUser, "f2.txt", "", strings.NewReader("x"), false); err == nil {
		t.Fatal("FilePut with cancelled ctx succeeded")
	}
	if _, err := s.FileStat(ctx, "t1", api.OriginUser, "f1.txt"); err == nil {
		t.Fatal("FileStat with cancelled ctx succeeded")
	}
	if _, err := s.FileRead(ctx, "t1", api.OriginUser, "f1.txt", 0, -1); err == nil {
		t.Fatal("FileRead with cancelled ctx succeeded")
	}
	if err := s.FileDelete(ctx, "t1", api.OriginUser, "f1.txt"); err == nil {
		t.Fatal("FileDelete with cancelled ctx succeeded")
	}
	if err := s.FileDeleteAll(ctx, "t1"); err == nil {
		t.Fatal("FileDeleteAll with cancelled ctx succeeded")
	}
	var errs []error
	for _, err := range s.FileList(ctx, "t1", api.OriginUser) {
		if err != nil {
			errs = append(errs, err)
		}
	}
	if len(errs) == 0 {
		t.Fatal("FileList with cancelled ctx yielded no error")
	}
}
