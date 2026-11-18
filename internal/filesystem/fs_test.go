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

package filesystem

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/storage/local"
)

const testThread = "thread"

func newTestFS(t *testing.T) (*workFS, *local.FileStorage) {
	t.Helper()
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	w := Open(testThread, store, store, api.FilesConfig{})
	t.Cleanup(func() { w.Dispose() })
	if _, err := w.root(); err != nil {
		t.Fatal(err)
	}
	return w, store
}

func putUpload(t *testing.T, store *local.FileStorage, name, mime, content string) api.File {
	t.Helper()
	f, err := store.FilePut(context.Background(), testThread, api.OriginUser, name, mime, strings.NewReader(content), false)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func putAgentFile(t *testing.T, store *local.FileStorage, name, mime, content string) api.File {
	t.Helper()
	f, err := store.FilePut(context.Background(), testThread, api.OriginAgent, name, mime, strings.NewReader(content), false)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func commitUpload(t *testing.T, store *local.FileStorage, name string) {
	t.Helper()
	if err := store.FileCommit(context.Background(), testThread, api.OriginUser, name); err != nil {
		t.Fatal(err)
	}
}

func writeFile(t *testing.T, w *workFS, name, content string) {
	t.Helper()
	f, err := w.Create(name)
	if err != nil {
		t.Fatalf("Create(%q): %v", name, err)
	}
	if _, err := io.WriteString(f, content); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}

func readAll(t *testing.T, w *workFS, name string) string {
	t.Helper()
	rc, err := w.ReadAll(context.Background(), name)
	if err != nil {
		t.Fatalf("ReadAll(%q): %v", name, err)
	}
	defer rc.Close()
	b, err := io.ReadAll(rc)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestCreateReadAllRoundTrip(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "notes/plan.txt", "hello")
	if got := readAll(t, w, "notes/plan.txt"); got != "hello" {
		t.Fatalf("got %q", got)
	}
}

func TestAppend(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "log.txt", "hello")
	f, err := w.Append("log.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(f, " world"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, w, "log.txt"); got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestUploadsReadOnly(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "data")
	writeFile(t, w, "local.txt", "x")
	if _, err := w.Create("user_uploads/new.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Append("user_uploads/doc.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Rename("user_uploads/doc.txt", "out.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Rename from uploads: %v", err)
	}
	if err := w.Rename("local.txt", "user_uploads/local.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Rename into uploads: %v", err)
	}
	if err := w.Remove(context.Background(), "user_uploads/doc.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Remove: %v", err)
	}
	if err := w.Copy(context.Background(), "local.txt", "user_uploads/copy.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Copy: %v", err)
	}
}

func TestReadDirMergedView(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "report.pdf", "application/pdf", "pdfbytes")
	writeFile(t, w, "local.txt", "x")
	assertRoot := func(stage string) {
		t.Helper()
		entries, err := w.ReadDir(context.Background(), ".")
		if err != nil {
			t.Fatal(err)
		}
		uploads := 0
		for _, e := range entries {
			if e.Name == "user_uploads" {
				uploads++
				if !e.Dir {
					t.Fatalf("%s: user_uploads not a dir", stage)
				}
			}
		}
		if uploads != 1 {
			t.Fatalf("%s: %d user_uploads entries, want 1", stage, uploads)
		}
	}
	assertRoot("before read")
	entries, err := w.ReadDir(context.Background(), "user_uploads")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "user_uploads/report.pdf" || entries[0].MIME != "application/pdf" || !entries[0].Virtual {
		t.Fatalf("uploads listing: %+v", entries)
	}
	if got := readAll(t, w, "user_uploads/report.pdf"); got != "pdfbytes" {
		t.Fatalf("got %q", got)
	}
	assertRoot("after read")
	if err := os.MkdirAll(filepath.Join(w.workspace.Dir, "user_uploads"), 0o755); err != nil {
		t.Fatal(err)
	}
	assertRoot("with stale physical dir")
}

func TestReadAllUploadStreams(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "content")
	if got := readAll(t, w, "user_uploads/doc.txt"); got != "content" {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "user_uploads", "doc.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized: %v", err)
	}
	if got := readAll(t, w, "user_uploads/doc.txt"); got != "content" {
		t.Fatalf("second read got %q", got)
	}
}

func TestReadRangeLocal(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "data.txt", "0123456789")
	readRange := func(off, n int64) (string, error) {
		rc, err := w.ReadRange(context.Background(), "data.txt", off, n)
		if err != nil {
			return "", err
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if got, err := readRange(2, 3); err != nil || got != "234" {
		t.Fatalf("mid: %q %v", got, err)
	}
	if got, err := readRange(5, -1); err != nil || got != "56789" {
		t.Fatalf("tail: %q %v", got, err)
	}
	if got, err := readRange(8, 100); err != nil || got != "89" {
		t.Fatalf("truncated: %q %v", got, err)
	}
	if _, err := readRange(10, 1); !errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("at EOF: %v", err)
	}
	if _, err := readRange(42, -1); !errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("past EOF: %v", err)
	}
	if _, err := readRange(-1, 5); err == nil {
		t.Fatal("negative off succeeded")
	}
	writeFile(t, w, "empty.txt", "")
	rc, err := w.ReadRange(context.Background(), "empty.txt", 0, -1)
	if err != nil {
		t.Fatalf("empty full read: %v", err)
	}
	b, err := io.ReadAll(rc)
	rc.Close()
	if err != nil || len(b) != 0 {
		t.Fatalf("empty read: %q %v", b, err)
	}
}

func TestReadRangeUpload(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "abcdef")
	readRange := func(off, n int64) (string, error) {
		rc, err := w.ReadRange(context.Background(), "user_uploads/doc.txt", off, n)
		if err != nil {
			return "", err
		}
		defer rc.Close()
		b, err := io.ReadAll(rc)
		if err != nil {
			return "", err
		}
		return string(b), nil
	}
	if got, err := readRange(1, 3); err != nil || got != "bcd" {
		t.Fatalf("mid: %q %v", got, err)
	}
	if got, err := readRange(3, -1); err != nil || got != "def" {
		t.Fatalf("tail: %q %v", got, err)
	}
	if got, err := readRange(4, 100); err != nil || got != "ef" {
		t.Fatalf("truncated: %q %v", got, err)
	}
	if _, err := readRange(6, 1); !errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("at EOF: %v", err)
	}
	if _, err := readRange(42, -1); !errors.Is(err, api.ErrOutOfRange) {
		t.Fatalf("past EOF: %v", err)
	}
	if _, err := readRange(-1, 5); err == nil {
		t.Fatal("negative off succeeded")
	}
	if got, err := readRange(2, 0); err != nil || got != "" {
		t.Fatalf("zero n: %q %v", got, err)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "user_uploads", "doc.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized: %v", err)
	}
}

func TestLocalPath(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "blob")
	writeFile(t, w, "local.txt", "here")
	got, err := w.LocalPath(context.Background(), "local.txt")
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(w.workspace.Dir, "local.txt"); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	if _, err := w.LocalPath(context.Background(), "missing.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing local: %v", err)
	}
	for _, name := range []string{"user_uploads/doc.txt", "user_uploads/missing.txt", "user_uploads", "agent_uploads"} {
		if _, err := w.LocalPath(context.Background(), name); !errors.Is(err, api.ErrVirtual) {
			t.Fatalf("LocalPath(%q) = %v, want api.ErrVirtual", name, err)
		}
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "user_uploads")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("physical uploads dir appeared: %v", err)
	}
}

func TestStat(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "12345")
	writeFile(t, w, "local.txt", "abc")
	e, err := w.Stat(context.Background(), "user_uploads/doc.txt")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "user_uploads/doc.txt" || e.Size != 5 || e.MIME != "text/plain" || e.Dir || !e.Virtual {
		t.Fatalf("upload entry: %+v", e)
	}
	e, err = w.Stat(context.Background(), "local.txt")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "local.txt" || e.Size != 3 || e.MIME != "text/plain" || e.Dir || e.Virtual {
		t.Fatalf("local entry: %+v", e)
	}
	e, err = w.Stat(context.Background(), "user_uploads")
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "user_uploads" || !e.Dir || !e.Virtual {
		t.Fatalf("dir entry: %+v", e)
	}
	if _, err := w.Stat(context.Background(), "missing.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing local: %v", err)
	}
	if _, err := w.Stat(context.Background(), "user_uploads/missing.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing upload: %v", err)
	}
}

func TestRenameLocal(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "old.txt", "content")
	if err := w.Rename("old.txt", "new.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Stat(context.Background(), "old.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("old still visible: %v", err)
	}
	if got := readAll(t, w, "new.txt"); got != "content" {
		t.Fatalf("got %q", got)
	}
}

func TestRemove(t *testing.T) {
	w, _ := newTestFS(t)
	if err := w.Remove(context.Background(), "missing.txt"); err != nil {
		t.Fatalf("ensure-absent: %v", err)
	}
	writeFile(t, w, "dir/a.txt", "1")
	writeFile(t, w, "dir/sub/b.txt", "2")
	if err := w.Remove(context.Background(), "dir"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Stat(context.Background(), "dir"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("dir survived: %v", err)
	}
}

func TestCopy(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "upload bytes")
	if err := w.Copy(context.Background(), "user_uploads/doc.txt", "edit/doc.txt"); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, w, "edit/doc.txt"); got != "upload bytes" {
		t.Fatalf("got %q", got)
	}
	if err := w.Copy(context.Background(), "missing.txt", "dst.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing src: %v", err)
	}
	if _, err := w.Stat(context.Background(), "dst.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("dst created despite missing src: %v", err)
	}
}

func TestMalformedUploadSubpath(t *testing.T) {
	w, _ := newTestFS(t)
	if _, err := w.ReadDir(context.Background(), "user_uploads/a/b"); err == nil {
		t.Fatal("ReadDir: no error for nested uploads path")
	}
	if _, err := w.Stat(context.Background(), "user_uploads/a/b"); err == nil {
		t.Fatal("Stat: no error for nested uploads path")
	}
	if err := w.Rename("user_uploads/a/b", "x"); err == nil || errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Rename should surface the resolve error, got %v", err)
	}
}

func TestGhostTmpInvisible(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "real.txt", "text/plain", "real")
	if err := os.MkdirAll(filepath.Join(w.workspace.Dir, "user_uploads"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(w.workspace.Dir, "user_uploads", "ghost.tmp"), []byte("junk"), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{".", "user_uploads", "./user_uploads"} {
		entries, err := w.ReadDir(context.Background(), dir)
		if err != nil {
			t.Fatalf("ReadDir(%q): %v", dir, err)
		}
		for _, e := range entries {
			if strings.Contains(e.Name, "ghost.tmp") {
				t.Fatalf("ReadDir(%q) leaked %q", dir, e.Name)
			}
		}
	}
	if _, err := w.Stat(context.Background(), "user_uploads/ghost.tmp"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Stat ghost: %v", err)
	}
	if _, err := w.ReadAll(context.Background(), "user_uploads/ghost.tmp"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ReadAll ghost: %v", err)
	}
	if _, err := w.ReadRange(context.Background(), "user_uploads/ghost.tmp", 0, 1); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ReadRange ghost: %v", err)
	}
	if _, err := w.LocalPath(context.Background(), "user_uploads/ghost.tmp"); !errors.Is(err, api.ErrVirtual) {
		t.Fatalf("LocalPath ghost: %v", err)
	}
}

func TestCopySelfPreservesContent(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "a.txt", "data")
	if err := w.Copy(context.Background(), "a.txt", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, w, "a.txt"); got != "data" {
		t.Fatalf("self copy: %q", got)
	}
	if err := w.Copy(context.Background(), "./a.txt", "a.txt"); err != nil {
		t.Fatal(err)
	}
	if got := readAll(t, w, "a.txt"); got != "data" {
		t.Fatalf("alias copy: %q", got)
	}
}

func TestCreateTrailingSlashLeavesNoJunk(t *testing.T) {
	w, _ := newTestFS(t)
	f, err := w.Create("logs/out.txt/")
	if err != nil {
		t.Fatalf("Create with trailing slash: %v", err)
	}
	if _, err := io.WriteString(f, "ok"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	e, err := w.Stat(context.Background(), "logs/out.txt")
	if err != nil || e.Dir {
		t.Fatalf("stat: %+v %v", e, err)
	}
	writeFile(t, w, "logs/out.txt", "again")
	if got := readAll(t, w, "logs/out.txt"); got != "again" {
		t.Fatalf("got %q", got)
	}
}

func TestRenameMissingDestDirBlamesDest(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "a.txt", "content")
	err := w.Rename("a.txt", "nodir/b.txt")
	if !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("err: %v", err)
	}
	if !strings.Contains(err.Error(), "nodir/b.txt") || strings.Contains(err.Error(), `"a.txt"`) {
		t.Fatalf("blames wrong name: %v", err)
	}
	if got := readAll(t, w, "a.txt"); got != "content" {
		t.Fatalf("src damaged: %q", got)
	}
}

func TestUploadsDirThroughFileMethods(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "x")
	readAllErr := func() string {
		t.Helper()
		_, err := w.ReadAll(context.Background(), "user_uploads")
		if err == nil {
			t.Fatal("ReadAll(user_uploads) succeeded")
		}
		return err.Error()
	}
	before := readAllErr()
	if got := readAll(t, w, "user_uploads/doc.txt"); got != "x" {
		t.Fatalf("got %q", got)
	}
	after := readAllErr()
	if before != after {
		t.Fatalf("state-dependent: %q vs %q", before, after)
	}
	if _, err := w.ReadRange(context.Background(), "user_uploads", 0, -1); err == nil {
		t.Fatal("ReadRange(user_uploads) succeeded")
	}
}

func TestUploadNameWithSeparators(t *testing.T) {
	w, store := newTestFS(t)
	e, err := w.UserUpload(context.Background(), "a/b.txt", strings.NewReader("nested"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "user_uploads/b.txt" {
		t.Fatalf("entry: %+v", e)
	}
	e, err = w.UserUpload(context.Background(), "..", strings.NewReader("dots"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "user_uploads/unnamed" {
		t.Fatalf("entry: %+v", e)
	}
	commitUpload(t, store, "b.txt")
	commitUpload(t, store, "unnamed")
	if got := readAll(t, w, "user_uploads/b.txt"); got != "nested" {
		t.Fatalf("b.txt: %q", got)
	}
	if got := readAll(t, w, "user_uploads/unnamed"); got != "dots" {
		t.Fatalf("unnamed: %q", got)
	}
}

func TestAgentUploadsMirrorsUserUploads(t *testing.T) {
	w, store := newTestFS(t)
	putAgentFile(t, store, "out.txt", "text/plain", "agent")
	putUpload(t, store, "in.txt", "text/plain", "user")
	entries, err := w.ReadDir(context.Background(), "agent_uploads")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "agent_uploads/out.txt" || entries[0].MIME != "text/plain" {
		t.Fatalf("agent listing: %+v", entries)
	}
	entries, err = w.ReadDir(context.Background(), "user_uploads")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name != "user_uploads/in.txt" {
		t.Fatalf("user listing: %+v", entries)
	}
	if got := readAll(t, w, "agent_uploads/out.txt"); got != "agent" {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "agent_uploads", "out.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized: %v", err)
	}
	e, err := w.Stat(context.Background(), "agent_uploads")
	if err != nil || e.Name != "agent_uploads" || !e.Dir {
		t.Fatalf("Stat: %+v, %v", e, err)
	}
	if _, err := w.Create("agent_uploads/x.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Create: %v", err)
	}
	if _, err := w.Append("agent_uploads/out.txt"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Append: %v", err)
	}
	if err := w.Remove(context.Background(), "agent_uploads"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Remove dir: %v", err)
	}
	if err := w.Remove(context.Background(), "agent_uploads/out.txt"); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	if _, err := w.Stat(context.Background(), "agent_uploads/out.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Stat after Remove: %v", err)
	}
	if err := w.Rename("agent_uploads/out.txt", "x"); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("Rename: %v", err)
	}
}

func TestReadDirRootShowsBothUploadsDirs(t *testing.T) {
	w, _ := newTestFS(t)
	entries, err := w.ReadDir(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, e := range entries {
		if e.Name == "user_uploads" || e.Name == "agent_uploads" {
			if !e.Dir || !e.Virtual || seen[e.Name] {
				t.Fatalf("entry %+v, seen %v", e, seen)
			}
			seen[e.Name] = true
		}
	}
	if !seen["user_uploads"] || !seen["agent_uploads"] {
		t.Fatalf("seen %v", seen)
	}
}

func TestCopyToAgentUploads(t *testing.T) {
	w, store := newTestFS(t)
	writeFile(t, w, "report.txt", "data")
	if err := w.Copy(context.Background(), "report.txt", "agent_uploads/report.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "agent_uploads", "report.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized early: %v", err)
	}
	var stored api.File
	for f, err := range store.FileList(context.Background(), testThread, api.OriginAgent) {
		if err != nil {
			t.Fatal(err)
		}
		stored = f
	}
	if stored.Origin != api.OriginAgent || stored.Name != "report.txt" || stored.Size != 4 {
		t.Fatalf("stored: %+v", stored)
	}
	if got := readAll(t, w, "agent_uploads/report.txt"); got != "data" {
		t.Fatalf("got %q", got)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "agent_uploads", "report.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized after read: %v", err)
	}
	if err := w.Copy(context.Background(), "report.txt", "agent_uploads/report.txt"); !errors.Is(err, api.ErrExists) {
		t.Fatalf("collision: %v", err)
	}
}

func TestCopyToAgentUploadsDetectsMIME(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.pdf", "application/pdf", "pdf")
	if err := w.Copy(context.Background(), "user_uploads/doc.pdf", "agent_uploads/doc.pdf"); err != nil {
		t.Fatal(err)
	}
	e, err := w.Stat(context.Background(), "agent_uploads/doc.pdf")
	if err != nil || e.MIME != "application/pdf" || e.Size != 3 {
		t.Fatalf("Stat: %+v, %v", e, err)
	}
	if got := readAll(t, w, "agent_uploads/doc.pdf"); got != "pdf" {
		t.Fatalf("got %q", got)
	}
	if err := w.Copy(context.Background(), "user_uploads/doc.pdf", "agent_uploads/noext"); err != nil {
		t.Fatal(err)
	}
	e, err = w.Stat(context.Background(), "agent_uploads/noext")
	if err != nil || e.MIME != "" {
		t.Fatalf("noext Stat: %+v, %v", e, err)
	}
}

func TestLocalEntriesCarryMIME(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "docs/a.pdf", "x")
	entries, err := w.ReadDir(context.Background(), "docs")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].MIME != "application/pdf" {
		t.Fatalf("entries: %+v", entries)
	}
	e, err := w.Stat(context.Background(), "docs")
	if err != nil || !e.Dir || e.MIME != "" {
		t.Fatalf("dir Stat: %+v, %v", e, err)
	}
}

func TestCopyToAgentUploadsErrors(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "x.txt", "x")
	if err := w.Copy(context.Background(), "x.txt", "agent_uploads"); err == nil || errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("dir dst: %v", err)
	}
	if err := w.Copy(context.Background(), "x.txt", "agent_uploads/a/b"); err == nil {
		t.Fatal("deep dst succeeded")
	}
	if err := w.Copy(context.Background(), "missing.txt", "agent_uploads/y.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing src: %v", err)
	}
}

func TestSameNameInBothUploadsDirs(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "user")
	putAgentFile(t, store, "doc.txt", "text/plain", "agent")
	if got := readAll(t, w, "user_uploads/doc.txt"); got != "user" {
		t.Fatalf("user: %q", got)
	}
	if got := readAll(t, w, "agent_uploads/doc.txt"); got != "agent" {
		t.Fatalf("agent: %q", got)
	}
}

func TestUserUpload(t *testing.T) {
	w, store := newTestFS(t)
	e, err := w.UserUpload(context.Background(), "notes.txt", strings.NewReader("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "user_uploads/notes.txt" || e.Size != 11 || e.MIME != "text/plain" || e.Dir {
		t.Fatalf("entry: %+v", e)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "user_uploads", "notes.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("materialized early: %v", err)
	}
	commitUpload(t, store, "notes.txt")
	if got := readAll(t, w, e.Name); got != "hello world" {
		t.Fatalf("got %q", got)
	}
}

func TestUserUploadSniffsMIME(t *testing.T) {
	w, _ := newTestFS(t)
	png := "\x89PNG\r\n\x1a\n" + strings.Repeat("\x00", 16)
	e, err := w.UserUpload(context.Background(), "pic.dat", strings.NewReader(png))
	if err != nil {
		t.Fatal(err)
	}
	if e.MIME != "image/png" {
		t.Fatalf("MIME: %+v", e)
	}
}

func TestUserUploadDedupsName(t *testing.T) {
	w, store := newTestFS(t)
	if _, err := w.UserUpload(context.Background(), "x.txt", strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	e, err := w.UserUpload(context.Background(), "x.txt", strings.NewReader("second"))
	if err != nil {
		t.Fatal(err)
	}
	if e.Name != "user_uploads/x(1).txt" {
		t.Fatalf("entry: %+v", e)
	}
	commitUpload(t, store, "x(1).txt")
	if got := readAll(t, w, "user_uploads/x(1).txt"); got != "second" {
		t.Fatalf("got %q", got)
	}
}

func TestUserUploadTrimsLongNamesToFit(t *testing.T) {
	w, store := newTestFS(t)
	long := strings.Repeat("界", 90) + ".txt"
	first, err := w.UserUpload(context.Background(), long, strings.NewReader("first"))
	if err != nil {
		t.Fatal(err)
	}
	stem := strings.Repeat("界", 83)
	if first.Name != "user_uploads/"+stem+".txt" {
		t.Fatalf("first: %q", first.Name)
	}
	commitUpload(t, store, stem+".txt")
	second, err := w.UserUpload(context.Background(), long, strings.NewReader("second"))
	if err != nil {
		t.Fatal(err)
	}
	if second.Name != "user_uploads/"+strings.Repeat("界", 82)+"(1).txt" {
		t.Fatalf("second: %q", second.Name)
	}
}

func TestDeleteUpload(t *testing.T) {
	w, store := newTestFS(t)
	putUpload(t, store, "doc.txt", "text/plain", "old")
	if got := readAll(t, w, "user_uploads/doc.txt"); got != "old" {
		t.Fatalf("got %q", got)
	}
	if err := w.DeleteUpload(context.Background(), "user_uploads/doc.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Stat(context.Background(), "user_uploads/doc.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Stat after delete: %v", err)
	}
	if _, err := os.Stat(filepath.Join(w.workspace.Dir, "user_uploads", "doc.txt")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("local copy survived: %v", err)
	}
	if err := w.DeleteUpload(context.Background(), "user_uploads/doc.txt"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
	e, err := w.UserUpload(context.Background(), "doc.txt", strings.NewReader("new"))
	if err != nil || e.Name != "user_uploads/doc.txt" {
		t.Fatalf("re-upload: %+v, %v", e, err)
	}
	commitUpload(t, store, "doc.txt")
	if got := readAll(t, w, "user_uploads/doc.txt"); got != "new" {
		t.Fatalf("after re-upload got %q", got)
	}
}

func TestDeleteAgentUpload(t *testing.T) {
	w, store := newTestFS(t)
	putAgentFile(t, store, "out.txt", "text/plain", "agent")
	if err := w.DeleteUpload(context.Background(), "agent_uploads/out.txt"); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Stat(context.Background(), "agent_uploads/out.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Stat after delete: %v", err)
	}
	if err := w.DeleteUpload(context.Background(), "agent_uploads/out.txt"); err != nil {
		t.Fatalf("second delete: %v", err)
	}
}

func TestDeleteUploadRejectsOtherPaths(t *testing.T) {
	w, store := newTestFS(t)
	putAgentFile(t, store, "out.txt", "text/plain", "agent")
	writeFile(t, w, "local.txt", "x")
	for _, name := range []string{"local.txt", "user_uploads", "agent_uploads"} {
		if err := w.DeleteUpload(context.Background(), name); err == nil {
			t.Fatalf("DeleteUpload(%q) succeeded", name)
		}
	}
	if got := readAll(t, w, "agent_uploads/out.txt"); got != "agent" {
		t.Fatalf("agent file: %q", got)
	}
	if got := readAll(t, w, "local.txt"); got != "x" {
		t.Fatalf("local file: %q", got)
	}
}

func TestUserUploadConcurrentSameName(t *testing.T) {
	w, _ := newTestFS(t)
	const n = 6
	names := make(chan string, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			e, err := w.UserUpload(context.Background(), "same.txt", strings.NewReader(fmt.Sprintf("body %d", i)))
			if err != nil {
				t.Errorf("upload %d: %v", i, err)
				return
			}
			names <- e.Name
		}()
	}
	wg.Wait()
	close(names)
	seen := map[string]bool{}
	for name := range names {
		if seen[name] {
			t.Fatalf("duplicate stored name %q", name)
		}
		seen[name] = true
	}
	if len(seen) != n {
		t.Fatalf("stored %d distinct names, want %d", len(seen), n)
	}
}

func TestDraftUnreadableUntilCommitted(t *testing.T) {
	w, store := newTestFS(t)
	ctx := context.Background()
	e, err := w.UserUpload(ctx, "draft.txt", strings.NewReader("secret"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.ReadAll(ctx, e.Name); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ReadAll draft: %v", err)
	}
	if _, err := w.ReadRange(ctx, e.Name, 0, 2); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("ReadRange draft: %v", err)
	}
	if err := w.Copy(ctx, e.Name, "copy.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Copy draft: %v", err)
	}
	if err := w.Copy(ctx, e.Name, "agent_uploads/copy.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("Copy draft to agent_uploads: %v", err)
	}
	got, err := w.UploadStat(ctx, e.Name)
	if err != nil || got.Size != 6 {
		t.Fatalf("UploadStat draft: %+v, %v", got, err)
	}
	commitUpload(t, store, "draft.txt")
	if got := readAll(t, w, e.Name); got != "secret" {
		t.Fatalf("after commit got %q", got)
	}
}

type clashStorage struct {
	api.UploadStorage
	calls  int
	drains int
}

func (c *clashStorage) FilePut(ctx context.Context, threadID api.ThreadID, origin api.Origin, name string, mime string, r io.Reader, draft bool) (api.File, error) {
	c.calls++
	if c.calls <= c.drains {
		io.Copy(io.Discard, r)
		return api.File{}, api.ErrExists
	}
	return c.UploadStorage.FilePut(ctx, threadID, origin, name, mime, r, draft)
}

func TestUserUploadRetriesOnlyUnreadBody(t *testing.T) {
	ctx := context.Background()
	for _, tc := range []struct {
		name   string
		body   string
		drains int
		ok     bool
	}{
		{"short body drained", "tiny", 2, true},
		{"long body drained", strings.Repeat("x", 2000), 1, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, store := newTestFS(t)
			clash := &clashStorage{UploadStorage: store, drains: tc.drains}
			w := Open(testThread, store, clash, api.FilesConfig{})
			t.Cleanup(func() { w.Dispose() })
			e, err := w.UserUpload(ctx, "f.txt", strings.NewReader(tc.body))
			if !tc.ok {
				if !errors.Is(err, api.ErrExists) || !strings.Contains(err.Error(), "upload it again") {
					t.Fatalf("got %+v, %v", e, err)
				}
				if clash.calls != 1 {
					t.Fatalf("FilePut calls %d, want 1", clash.calls)
				}
				return
			}
			if err != nil || e.Name != "user_uploads/f(2).txt" || e.Size != int64(len(tc.body)) {
				t.Fatalf("got %+v, %v", e, err)
			}
		})
	}
}

func TestDirtyNameResolvesTheSameEverywhere(t *testing.T) {
	w, _ := newTestFS(t)
	const dirty = "a/../b.txt"
	writeFile(t, w, dirty, "hello")
	if got := readAll(t, w, "b.txt"); got != "hello" {
		t.Fatalf("cleaned write landed elsewhere: %q", got)
	}
	e, err := w.Stat(context.Background(), dirty)
	if err != nil {
		t.Fatalf("Stat(%q): %v", dirty, err)
	}
	if e.Name != "b.txt" {
		t.Fatalf("Stat name %q, want b.txt", e.Name)
	}
	if got := readAll(t, w, dirty); got != "hello" {
		t.Fatalf("ReadAll(%q): %q", dirty, got)
	}
	rc, err := w.ReadRange(context.Background(), dirty, 0, 2)
	if err != nil {
		t.Fatalf("ReadRange(%q): %v", dirty, err)
	}
	rc.Close()
	got, err := w.LocalPath(context.Background(), dirty)
	if err != nil {
		t.Fatalf("LocalPath(%q): %v", dirty, err)
	}
	if want := filepath.Join(w.workspace.Dir, "b.txt"); got != want {
		t.Fatalf("LocalPath %q, want %q", got, want)
	}
	if err := w.Rename(dirty, "c/../d.txt"); err != nil {
		t.Fatalf("Rename(%q): %v", dirty, err)
	}
	if got := readAll(t, w, "d.txt"); got != "hello" {
		t.Fatalf("renamed content: %q", got)
	}
	if _, err := w.Stat(context.Background(), "x/../missing.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("missing dirty name: %v", err)
	} else if !strings.Contains(err.Error(), `"missing.txt"`) {
		t.Fatalf("error quotes uncleaned name: %v", err)
	}
}

func TestTempDirReservedEverywhere(t *testing.T) {
	w, _ := newTestFS(t)
	ctx := context.Background()
	dir, err := w.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unzipped.xml"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w, "local.txt", "x")
	for _, name := range []string{".tmp-thread", ".tmp-thread/", "./.tmp-thread", ".tmp-thread/unzipped.xml", ".tmp-thread/deep/x.bin"} {
		if _, err := w.ReadRange(ctx, name, 0, -1); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("ReadRange(%q): %v", name, err)
		}
		if _, err := w.ReadAll(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("ReadAll(%q): %v", name, err)
		}
		if _, err := w.LocalPath(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("LocalPath(%q): %v", name, err)
		}
		if _, err := w.Create(name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Create(%q): %v", name, err)
		}
		if _, err := w.Append(name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Append(%q): %v", name, err)
		}
		if err := w.Copy(ctx, "local.txt", name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Copy to %q: %v", name, err)
		}
		if err := w.Copy(ctx, name, "out.txt"); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Copy from %q: %v", name, err)
		}
		if err := w.Rename("local.txt", name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Rename to %q: %v", name, err)
		}
		if err := w.Rename(name, "out.txt"); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Rename from %q: %v", name, err)
		}
		if err := w.Remove(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Remove(%q): %v", name, err)
		}
		if _, err := w.Stat(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Stat(%q): %v", name, err)
		}
		if _, err := w.ReadDir(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("ReadDir(%q): %v", name, err)
		}
		if _, err := w.UploadStat(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("UploadStat(%q): %v", name, err)
		}
		if err := w.DeleteUpload(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("DeleteUpload(%q): %v", name, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "unzipped.xml")); err != nil || string(b) != "scratch" {
		t.Fatalf("scratch file: %q, %v", b, err)
	}
	if got := readAll(t, w, "local.txt"); got != "x" {
		t.Fatalf("local.txt: %q", got)
	}
	if _, err := w.Stat(ctx, "out.txt"); !errors.Is(err, api.ErrNotFound) {
		t.Fatalf("out.txt materialized: %v", err)
	}
}

func TestTempDirReservedCaseInsensitively(t *testing.T) {
	w, _ := newTestFS(t)
	ctx := context.Background()
	dir, err := w.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "unzipped.xml"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w, "local.txt", "x")
	for _, name := range []string{".Tmp-Thread/leak.txt", "./.TMP-THREAD/deep/x.bin", ".TMP-thread"} {
		if f, err := w.Create(name); !errors.Is(err, api.ErrReserved) {
			if err == nil {
				f.Close()
			}
			t.Fatalf("Create(%q): %v", name, err)
		}
		if _, err := w.Append(name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Append(%q): %v", name, err)
		}
		if err := w.Remove(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Remove(%q): %v", name, err)
		}
		if _, err := w.Stat(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Stat(%q): %v", name, err)
		}
		if _, err := w.ReadAll(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("ReadAll(%q): %v", name, err)
		}
		if _, err := w.LocalPath(ctx, name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("LocalPath(%q): %v", name, err)
		}
		if err := w.Copy(ctx, "local.txt", name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Copy to %q: %v", name, err)
		}
		if err := w.Rename("local.txt", name); !errors.Is(err, api.ErrReserved) {
			t.Fatalf("Rename to %q: %v", name, err)
		}
	}
	if b, err := os.ReadFile(filepath.Join(dir, "unzipped.xml")); err != nil || string(b) != "scratch" {
		t.Fatalf("scratch file: %q, %v", b, err)
	}
	entries, err := w.ReadDir(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.EqualFold(e.Name, tempDirPrefix+testThread) {
			t.Fatalf("ReadDir leaked %q", e.Name)
		}
	}
}

func TestTempDirHiddenFromReadDir(t *testing.T) {
	w, _ := newTestFS(t)
	dir, err := w.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "x.bin"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w, "visible.txt", "x")
	entries, err := w.ReadDir(context.Background(), ".")
	if err != nil {
		t.Fatal(err)
	}
	seen := false
	for _, e := range entries {
		if strings.Contains(e.Name, tempDirPrefix) {
			t.Fatalf("ReadDir leaked %q", e.Name)
		}
		if e.Name == "visible.txt" {
			seen = true
		}
	}
	if !seen {
		t.Fatalf("entries: %+v", entries)
	}
}

func TestTempDirCreatesUsablePath(t *testing.T) {
	w, _ := newTestFS(t)
	got, err := w.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("not absolute: %q", got)
	}
	if want := filepath.Join(w.workspace.Dir, tempDirPrefix+testThread); got != want {
		t.Fatalf("got %q want %q", got, want)
	}
	info, err := os.Stat(got)
	if err != nil || !info.IsDir() {
		t.Fatalf("stat: %+v, %v", info, err)
	}
	f, err := os.Create(filepath.Join(got, "unzip.tmp"))
	if err != nil {
		t.Fatalf("direct create: %v", err)
	}
	if _, err := io.WriteString(f, "excelize"); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := w.TempDir()
	if err != nil || again != got {
		t.Fatalf("second call: %q, %v", again, err)
	}
	if b, err := os.ReadFile(filepath.Join(got, "unzip.tmp")); err != nil || string(b) != "excelize" {
		t.Fatalf("second call disturbed the scratch: %q, %v", b, err)
	}
}

func TestTempWipeEmptiesScratch(t *testing.T) {
	w, _ := newTestFS(t)
	dir, err := w.TempDir()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "nested"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "nested", "a.bin"), []byte("scratch"), 0o644); err != nil {
		t.Fatal(err)
	}
	writeFile(t, w, "keep.txt", "keep")
	if err := w.TempWipe(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("scratch survived: %v", err)
	}
	if got := readAll(t, w, "keep.txt"); got != "keep" {
		t.Fatalf("keep.txt: %q", got)
	}
	if err := w.TempWipe(); err != nil {
		t.Fatalf("wipe of an absent scratch: %v", err)
	}
	again, err := w.TempDir()
	if err != nil || again != dir {
		t.Fatalf("TempDir after wipe: %q, %v", again, err)
	}
	left, err := os.ReadDir(again)
	if err != nil || len(left) != 0 {
		t.Fatalf("fresh scratch holds %+v, %v", left, err)
	}
}

func TestCopyDirectorySourceKeepsDestination(t *testing.T) {
	w, _ := newTestFS(t)
	writeFile(t, w, "keep.txt", "precious")
	if err := w.workspace.Root.Mkdir("folder", 0o755); err != nil {
		t.Fatal(err)
	}
	err := w.Copy(context.Background(), "folder", "keep.txt")
	if err == nil || !strings.Contains(err.Error(), "is a directory") {
		t.Fatalf("Copy from a directory = %v, want an is-a-directory error", err)
	}
	if strings.Contains(err.Error(), w.workspace.Dir) {
		t.Fatalf("error leaks the host path: %v", err)
	}
	if got := readAll(t, w, "keep.txt"); got != "precious" {
		t.Fatalf("destination changed to %q", got)
	}
	err = w.Copy(context.Background(), "folder", "agent_uploads/folder")
	if err == nil || strings.Contains(err.Error(), w.workspace.Dir) {
		t.Fatalf("Copy from a directory to agent_uploads = %v", err)
	}
}

type flakyStorage struct {
	dir     string
	fails   int
	opens   int
	deletes int
}

func (s *flakyStorage) WorkspaceOpen(threadID api.ThreadID) (api.Workspace, error) {
	s.opens++
	if s.fails > 0 {
		s.fails--
		return api.Workspace{}, errors.New("disk")
	}
	dir := filepath.Join(s.dir, string(threadID))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return api.Workspace{}, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return api.Workspace{}, err
	}
	return api.Workspace{Dir: dir, Root: root}, nil
}

func (s *flakyStorage) WorkspaceDelete(threadID api.ThreadID) error {
	s.deletes++
	return os.RemoveAll(filepath.Join(s.dir, string(threadID)))
}

func newLazyFS(t *testing.T, fails int) (*workFS, *flakyStorage, *local.FileStorage) {
	t.Helper()
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	storage := &flakyStorage{dir: t.TempDir(), fails: fails}
	w := Open(testThread, storage, store, api.FilesConfig{})
	t.Cleanup(func() { w.Dispose() })
	return w, storage, store
}

func TestOpenRetriesAFailedWorkspace(t *testing.T) {
	w, storage, _ := newLazyFS(t, 2)
	for i := range 2 {
		if _, err := w.Create("a.txt"); err == nil || err.Error() != "disk" {
			t.Fatalf("Create %d with a broken workspace = %v, want the open error", i, err)
		}
	}
	writeFile(t, w, "a.txt", "ok")
	if got := readAll(t, w, "a.txt"); got != "ok" {
		t.Fatalf("read back %q", got)
	}
	if storage.opens != 3 {
		t.Fatalf("workspace opened %d times, want 3", storage.opens)
	}
}

func TestUploadPathsNeverOpenTheWorkspace(t *testing.T) {
	w, storage, store := newLazyFS(t, 0)
	ctx := context.Background()
	if _, err := w.UserUpload(ctx, "a.txt", strings.NewReader("a")); err != nil {
		t.Fatalf("UserUpload: %v", err)
	}
	putAgentFile(t, store, "b.txt", "text/plain", "b")
	if _, err := w.ReadDir(ctx, agentUploadsDirName); err != nil {
		t.Fatalf("ReadDir(agent_uploads): %v", err)
	}
	if err := w.TempWipe(); err != nil {
		t.Fatalf("TempWipe: %v", err)
	}
	if storage.opens != 0 {
		t.Fatalf("upload-only calls opened the workspace %d times", storage.opens)
	}
	if left, _ := os.ReadDir(storage.dir); len(left) != 0 {
		t.Fatalf("upload-only calls left %v on disk", left)
	}
}

func TestDisposeOfANeverOpenedFSTouchesNothing(t *testing.T) {
	w, storage, _ := newLazyFS(t, 0)
	w.Dispose()
	if storage.opens != 0 || storage.deletes != 0 {
		t.Fatalf("Dispose opened %d and deleted %d workspaces", storage.opens, storage.deletes)
	}
}

func TestDisposeDeletesAnOpenedWorkspace(t *testing.T) {
	w, storage, _ := newLazyFS(t, 0)
	writeFile(t, w, "a.txt", "a")
	w.Dispose()
	if storage.deletes != 1 {
		t.Fatalf("Dispose deleted %d workspaces, want 1", storage.deletes)
	}
	if left, _ := os.ReadDir(storage.dir); len(left) != 0 {
		t.Fatalf("Dispose left %v on disk", left)
	}
}

func TestCreateAfterDisposeIsClosed(t *testing.T) {
	w, storage, _ := newLazyFS(t, 0)
	w.Dispose()
	if _, err := w.Create("a.txt"); !errors.Is(err, os.ErrClosed) {
		t.Fatalf("Create after Dispose = %v, want os.ErrClosed", err)
	}
	if storage.opens != 0 {
		t.Fatalf("Create after Dispose opened the workspace")
	}
	if left, _ := os.ReadDir(storage.dir); len(left) != 0 {
		t.Fatalf("Create after Dispose left %v on disk", left)
	}
}
