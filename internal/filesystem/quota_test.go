package filesystem

import (
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
)

func newQuotaFS(t *testing.T, config api.FilesConfig) (*workFS, *local.FileStorage) {
	t.Helper()
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	ws, err := store.WorkspaceOpen(testThread)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	w := New(testThread, ws, store, config)
	return w, store
}

func TestQuotaFileCountCap(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 2})
	writeFile(t, w, "a.txt", "1")
	writeFile(t, w, "b.txt", "2")
	if _, err := w.Create("c.txt"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("third create: %v", err)
	}
	if _, err := w.Stat(context.Background(), "c.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("refused file exists: %v", err)
	}
	writeFile(t, w, "a.txt", "rewrite")
	if got := readAll(t, w, "a.txt"); got != "rewrite" {
		t.Fatalf("got %q", got)
	}
}

func TestQuotaByteCapMidWrite(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxBytes: 10})
	f, err := w.Create("big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, "123456"); err != nil {
		t.Fatalf("first write: %v", err)
	}
	if _, err := io.WriteString(f, "789012"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("crossing write: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	e, err := w.Stat(context.Background(), "big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if e.Size != 6 {
		t.Fatalf("size %d, want 6", e.Size)
	}
}

func TestQuotaTruncateReleasesOldSize(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxBytes: 10})
	writeFile(t, w, "a.txt", "12345678")
	writeFile(t, w, "a.txt", "123456789")
	if got := readAll(t, w, "a.txt"); got != "123456789" {
		t.Fatalf("got %q", got)
	}
}

func TestQuotaAppendAccumulates(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxBytes: 10})
	writeFile(t, w, "log.txt", "123456")
	f, err := w.Append("log.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, "789"); err != nil {
		t.Fatalf("append under cap: %v", err)
	}
	if _, err := io.WriteString(f, "012"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("append over cap: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, w, "log.txt"); got != "123456789" {
		t.Fatalf("got %q", got)
	}
}

func TestQuotaRemoveFreesBudget(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 1, MaxBytes: 5})
	writeFile(t, w, "a.txt", "12345")
	if _, err := w.Create("b.txt"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("second create: %v", err)
	}
	if err := w.Remove(context.Background(), "a.txt"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w, "b.txt", "12345")
	if got := readAll(t, w, "b.txt"); got != "12345" {
		t.Fatalf("got %q", got)
	}
}

func TestQuotaRemoveDirFreesSubtree(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 3, MaxBytes: 10})
	writeFile(t, w, "dir/a.txt", "1234")
	writeFile(t, w, "dir/sub/b.txt", "1234")
	f, err := w.Create("big.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, "1234567890"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("write over cap: %v", err)
	}
	f.Close()
	if err := w.Remove(context.Background(), "big.txt"); err != nil {
		t.Fatal(err)
	}
	if err := w.Remove(context.Background(), "dir"); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w, "big.txt", "1234567890")
	writeFile(t, w, "x.txt", "")
	writeFile(t, w, "y.txt", "")
}

func TestQuotaRenameOverwriteFreesDst(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 2, MaxBytes: 10})
	writeFile(t, w, "a.txt", "123456")
	writeFile(t, w, "b.txt", "1234")
	if _, err := w.Create("c.txt"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("create at cap: %v", err)
	}
	if err := w.Rename("a.txt", "b.txt"); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, w, "b.txt"); got != "123456" {
		t.Fatalf("got %q", got)
	}
	writeFile(t, w, "c.txt", "1234")
}

func TestQuotaZeroLimitsEnforceNothing(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{})
	for _, name := range []string{"a.txt", "b.txt", "c.txt", "d.txt"} {
		writeFile(t, w, name, strings.Repeat("x", 4096))
	}
	if got := readAll(t, w, "d.txt"); got != strings.Repeat("x", 4096) {
		t.Fatalf("got %d bytes", len(got))
	}
}

func TestQuotaNegativeByteLimitOff(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 2, MaxBytes: -1})
	writeFile(t, w, "a.txt", strings.Repeat("x", 4096))
	writeFile(t, w, "b.txt", strings.Repeat("y", 4096))
	if got := readAll(t, w, "b.txt"); got != strings.Repeat("y", 4096) {
		t.Fatalf("got %d bytes", len(got))
	}
	if _, err := w.Create("c.txt"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("file count no longer enforced: %v", err)
	}
}

func TestQuotaIgnoresUploads(t *testing.T) {
	w, store := newQuotaFS(t, api.FilesConfig{MaxFiles: 1, MaxBytes: 5})
	putUpload(t, store, "doc.txt", "text/plain", strings.Repeat("u", 100))
	if _, err := w.Create("user_uploads/new.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Create in uploads: %v", err)
	}
	writeFile(t, w, "local.txt", "12345")
	if got := readAll(t, w, "user_uploads/doc.txt"); got != strings.Repeat("u", 100) {
		t.Fatalf("upload read: %d bytes", len(got))
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "user_uploads", "doc.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized: %v", err)
	}
}

func TestQuotaLazyScanCountsExistingFiles(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 2, MaxBytes: 10})
	if err := os.WriteFile(filepath.Join(w.workspace.Dir, "pre1.txt"), []byte("12345"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.workspace.Dir, "pre2.txt"), []byte("123"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Create("new.txt"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("create past preexisting file count: %v", err)
	}
	f, err := w.Create("pre2.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, "123456"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("write past preexisting bytes: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestQuotaCopyFailureReleasesBudget(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 2, MaxBytes: 10})
	writeFile(t, w, "src.txt", "12345678")
	if err := w.Copy(context.Background(), "src.txt", "dst.txt"); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("copy over cap: %v", err)
	}
	if _, err := w.Stat(context.Background(), "dst.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("dst left behind: %v", err)
	}
	writeFile(t, w, "ok.txt", "12")
	if got := readAll(t, w, "ok.txt"); got != "12" {
		t.Fatalf("got %q", got)
	}
}

func TestQuotaIgnoresTempDir(t *testing.T) {
	w, _ := newQuotaFS(t, api.FilesConfig{MaxFiles: 2, MaxBytes: 10})
	dir, err := w.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"a.bin", "b.bin", "c.bin"} {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(strings.Repeat("s", 100)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	writeFile(t, w, "local.txt", "12345")
	if got := readAll(t, w, "local.txt"); got != "12345" {
		t.Fatalf("got %q", got)
	}
}
