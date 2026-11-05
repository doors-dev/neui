package text

import (
	"context"
	"errors"
	"io"
	"iter"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/filesystem"
	"neui.dev/internal/storage/local"
	"neui.dev/internal/tools"
)

func newFS(t *testing.T) filesystem.FS {
	t.Helper()
	store, err := local.NewFileStorage("")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Remove() })
	ws, err := store.WorkspaceOpen("thread")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ws.Close() })
	return filesystem.New("thread", ws, store, api.FilesConfig{})
}

type testThread struct {
	fs    api.FS
	files api.FilesConfig
}

func (testThread) Context() context.Context { return context.Background() }

func (testThread) ID() api.ThreadID { return "thread" }

func (testThread) PageContext() context.Context { return context.Background() }

func (testThread) Task(string, map[string]any, context.CancelCauseFunc) (common.Task, error) {
	return nil, errors.New("no tasks in tests")
}

func (testThread) Ask(api.ChatAsk) (func(context.Context), error) {
	return nil, errors.New("no asks in tests")
}

func (testThread) Rename(context.Context, string) error { return nil }

func (testThread) Modules() iter.Seq[common.Module] {
	return func(func(common.Module) bool) {}
}

func (t testThread) FS() api.FS               { return t.fs }
func (t testThread) Model() api.Model         { return api.Model{} }
func (t testThread) Config() api.ThreadConfig { return api.ThreadConfig{} }

func (t testThread) FilesConfig() api.FilesConfig {
	return common.FilesConfigDefaults(t.files)
}

func threadCtx(fs api.FS) context.Context {
	return context.WithValue(context.Background(), common.KeyController, testThread{fs: fs})
}

func content(t *testing.T, fs filesystem.FS, name string) string {
	t.Helper()
	rc, err := fs.ReadAll(context.Background(), name)
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

func TestRegister(t *testing.T) {
	m := tools.NewModule("files", "")
	Register(m)
	count := 0
	for range m.List() {
		count++
	}
	if count != 5 {
		t.Fatalf("tools = %d, want 5", count)
	}
}

func TestWriteAppend(t *testing.T) {
	fs := newFS(t)
	res, err := writeTool(threadCtx(fs), writeArg{Path: "notes/a.txt", Content: "one\n"})
	if err != nil || res.Size != 4 || res.Path != "notes/a.txt" {
		t.Fatalf("%+v, %v", res, err)
	}
	res, err = appendTool(threadCtx(fs), writeArg{Path: "notes/a.txt", Content: "two\n"})
	if err != nil || res.Size != 8 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "notes/a.txt"); got != "one\ntwo\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEdit(t *testing.T) {
	fs := newFS(t)
	edit := editTool
	writeTool(threadCtx(fs), writeArg{Path: "a.txt", Content: "aa bb aa\n"})
	res, err := edit(threadCtx(fs), editArg{Path: "a.txt", Old: "bb", New: "XX"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "a.txt"); got != "aa XX aa\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := edit(threadCtx(fs), editArg{Path: "a.txt", Old: "aa", New: "y"}); err == nil || !strings.Contains(err.Error(), "2 places") {
		t.Fatalf("ambiguous: %v", err)
	}
	res, err = edit(threadCtx(fs), editArg{Path: "a.txt", Old: "aa", New: "y", ReplaceAll: true})
	if err != nil || res.Replacements != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if _, err := edit(threadCtx(fs), editArg{Path: "a.txt", Old: "gone", New: "y"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := edit(threadCtx(fs), editArg{Path: "a.txt", Old: "", New: "y"}); err == nil {
		t.Fatal("empty old accepted")
	}
	if _, err := edit(threadCtx(fs), editArg{Path: "a.txt", Old: "y", New: "y"}); err == nil {
		t.Fatal("identical accepted")
	}
	if _, err := edit(threadCtx(fs), editArg{Path: "user_uploads/x.txt", Old: "a", New: "b"}); err == nil {
		t.Fatal("uploads path accepted")
	}
}

func TestReplace(t *testing.T) {
	fs := newFS(t)
	replace := replaceTool
	writeTool(threadCtx(fs), writeArg{Path: "r.txt", Content: "aa bb aa\n"})
	res, err := replace(threadCtx(fs), replaceArg{Path: "r.txt", Pattern: "b+", Replacement: "XX"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "r.txt"); got != "aa XX aa\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := replace(threadCtx(fs), replaceArg{Path: "r.txt", Pattern: "aa", Replacement: "y"}); err == nil || !strings.Contains(err.Error(), "2 places") {
		t.Fatalf("ambiguous: %v", err)
	}
	res, err = replace(threadCtx(fs), replaceArg{Path: "r.txt", Pattern: "aa", Replacement: "y", All: true})
	if err != nil || res.Replacements != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "r.txt"); got != "y XX y\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := replace(threadCtx(fs), replaceArg{Path: "r.txt", Pattern: "zzz", Replacement: "y"}); err == nil || !strings.Contains(err.Error(), "matches nothing") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := replace(threadCtx(fs), replaceArg{Path: "r.txt", Pattern: "(", Replacement: "y"}); err == nil || !strings.Contains(err.Error(), "bad pattern") {
		t.Fatalf("invalid: %v", err)
	}
	writeTool(threadCtx(fs), writeArg{Path: "g.txt", Content: "name=alex\n"})
	res, err = replace(threadCtx(fs), replaceArg{Path: "g.txt", Pattern: `name=(\w+)`, Replacement: "user=$1"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "g.txt"); got != "user=alex\n" {
		t.Fatalf("got %q", got)
	}
	writeTool(threadCtx(fs), writeArg{Path: "s.txt", Content: "foo\nmid\nbar tail\n"})
	res, err = replace(threadCtx(fs), replaceArg{Path: "s.txt", Pattern: "(?s)foo.*?bar", Replacement: "span"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "s.txt"); got != "span tail\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEditsWithTheCapTurnedOff(t *testing.T) {
	fs := newFS(t)
	ctx := context.WithValue(context.Background(), common.KeyController, testThread{fs: fs, files: api.FilesConfig{EditLimit: -1}})
	writeTool(ctx, writeArg{Path: "a.txt", Content: "hello\nworld\n"})
	res, err := insertTool(ctx, insertArg{Path: "a.txt", Line: 0, Text: "first\n"})
	if err != nil {
		t.Fatalf("insert with the edit cap off: %v", err)
	}
	if got := content(t, fs, "a.txt"); got != "first\nhello\nworld\n" {
		t.Fatalf("insert with the edit cap off left %q: the file read as empty and the rewrite destroyed it", got)
	}
	if res.Size != int64(len("first\nhello\nworld\n")) {
		t.Fatalf("insert reported %d bytes, want %d", res.Size, len("first\nhello\nworld\n"))
	}
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "hello", New: "goodbye"}); err != nil {
		t.Fatalf("edit with the edit cap off: %v", err)
	}
	if got := content(t, fs, "a.txt"); got != "first\ngoodbye\nworld\n" {
		t.Fatalf("edit with the edit cap off left %q", got)
	}
	if _, err := replaceTool(ctx, replaceArg{Path: "a.txt", Pattern: "world", Replacement: "earth"}); err != nil {
		t.Fatalf("replace with the edit cap off: %v", err)
	}
	if got := content(t, fs, "a.txt"); got != "first\ngoodbye\nearth\n" {
		t.Fatalf("replace with the edit cap off left %q", got)
	}
}

func TestInsert(t *testing.T) {
	fs := newFS(t)
	insert := insertTool
	writeTool(threadCtx(fs), writeArg{Path: "i.txt", Content: "one\ntwo\nthree\n"})
	res, err := insert(threadCtx(fs), insertArg{Path: "i.txt", Line: 0, Text: "zero"})
	if err != nil || res.Size != 19 || res.Path != "i.txt" {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "i.txt"); got != "zero\none\ntwo\nthree\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := insert(threadCtx(fs), insertArg{Path: "i.txt", Line: 2, Text: "mid\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "i.txt"); got != "zero\none\nmid\ntwo\nthree\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := insert(threadCtx(fs), insertArg{Path: "i.txt", Line: 5, Text: "end"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "i.txt"); got != "zero\none\nmid\ntwo\nthree\nend\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := insert(threadCtx(fs), insertArg{Path: "i.txt", Line: 9, Text: "x"}); err == nil || !strings.Contains(err.Error(), "6 lines") {
		t.Fatalf("beyond: %v", err)
	}
	writeTool(threadCtx(fs), writeArg{Path: "n.txt", Content: "tail"})
	if _, err := insert(threadCtx(fs), insertArg{Path: "n.txt", Line: 0, Text: "head\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "n.txt"); got != "head\ntail" {
		t.Fatalf("got %q", got)
	}
	writeTool(threadCtx(fs), writeArg{Path: "e.txt", Content: ""})
	if _, err := insert(threadCtx(fs), insertArg{Path: "e.txt", Line: 0, Text: "first\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "e.txt"); got != "first\n" {
		t.Fatalf("got %q", got)
	}
}
