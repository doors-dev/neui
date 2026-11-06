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
	fs := filesystem.Open("thread", store, store, api.FilesConfig{})
	t.Cleanup(fs.Dispose)
	return fs
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

func raw(t *testing.T, fs filesystem.FS, name, s string) {
	t.Helper()
	w, err := fs.Create(name)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := io.WriteString(w, s); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestEditKeepsCRLF(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "a.txt", "one\r\ntwo\r\nthree\r\n")
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "one\ntwo\n", New: "1\n2\n2.5\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "1\r\n2\r\n2.5\r\nthree\r\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "1\r\n2", New: "x\r\ny"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "x\r\ny\r\n2.5\r\nthree\r\n" {
		t.Fatalf("CRLF in old/new: got %q", got)
	}
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "\nthree", New: " three"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "x\r\ny\r\n2.5 three\r\n" {
		t.Fatalf("old starting with a line break: got %q", got)
	}
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "y", New: "y\rz"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "x\r\ny\rz\r\n2.5 three\r\n" {
		t.Fatalf("lone CR in new: got %q", got)
	}
}

func TestEditMixedFileTakesItsDominantEnding(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "c.txt", "a\r\nb\r\nc\nd\r\n")
	if _, err := editTool(ctx, editArg{Path: "c.txt", Old: "b", New: "B"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "c.txt"); got != "a\r\nB\r\nc\r\nd\r\n" {
		t.Fatalf("CRLF majority: got %q", got)
	}
	raw(t, fs, "l.txt", "a\nb\r\nc\n")
	if _, err := editTool(ctx, editArg{Path: "l.txt", Old: "a", New: "A"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "l.txt"); got != "A\nb\nc\n" {
		t.Fatalf("LF majority: got %q", got)
	}
	raw(t, fs, "t.txt", "a\r\nb\n")
	if _, err := editTool(ctx, editArg{Path: "t.txt", Old: "a", New: "A"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "t.txt"); got != "A\nb\n" {
		t.Fatalf("tie: got %q", got)
	}
}

func TestReplaceOnCRLF(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "r.txt", "foo\r\nbar foo\r\nbaz\r\n")
	res, err := replaceTool(ctx, replaceArg{Path: "r.txt", Pattern: `(?m)foo$`, Replacement: "end", All: true})
	if err != nil || res.Replacements != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "r.txt"); got != "end\r\nbar end\r\nbaz\r\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := replaceTool(ctx, replaceArg{Path: "r.txt", Pattern: `end\nbar`, Replacement: "x\ny\nz"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "r.txt"); got != "x\r\ny\r\nz end\r\nbaz\r\n" {
		t.Fatalf("multi-line: got %q", got)
	}
}

func TestInsertOnCRLF(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "i.txt", "\ufeffone\r\ntwo\r\n")
	for _, step := range []struct {
		line uint
		text string
		want string
	}{
		{1, "mid\n", "\ufeffone\r\nmid\r\ntwo\r\n"},
		{3, "end", "\ufeffone\r\nmid\r\ntwo\r\nend\r\n"},
		{0, "top\n", "\ufefftop\r\none\r\nmid\r\ntwo\r\nend\r\n"},
	} {
		if _, err := insertTool(ctx, insertArg{Path: "i.txt", Line: step.line, Text: step.text}); err != nil {
			t.Fatal(err)
		}
		if got := content(t, fs, "i.txt"); got != step.want {
			t.Fatalf("line %d: got %q, want %q", step.line, got, step.want)
		}
	}
	raw(t, fs, "n.txt", "a\r\nb")
	if _, err := insertTool(ctx, insertArg{Path: "n.txt", Line: 2, Text: "c"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "n.txt"); got != "a\r\nb\r\nc" {
		t.Fatalf("no final newline: got %q", got)
	}
}

func TestWriteKeepsTheFileEnding(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "w.txt", "\ufeffold\r\nlines\r\n")
	if _, err := writeTool(ctx, writeArg{Path: "w.txt", Content: "new\ntext\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "w.txt"); got != "\ufeffnew\r\ntext\r\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := writeTool(ctx, writeArg{Path: "w.txt", Content: "as\r\nis\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "w.txt"); got != "as\r\nis\n" {
		t.Fatalf("content with CR: got %q", got)
	}
	if _, err := writeTool(ctx, writeArg{Path: "fresh.txt", Content: "a\nb\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "fresh.txt"); got != "a\nb\n" {
		t.Fatalf("new file: got %q", got)
	}
	raw(t, fs, "empty.txt", "")
	if _, err := writeTool(ctx, writeArg{Path: "empty.txt", Content: "a\nb\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "empty.txt"); got != "a\nb\n" {
		t.Fatalf("empty file: got %q", got)
	}
}

func TestAppendKeepsTheFileEnding(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "a.txt", "\ufeffone\r\n")
	if _, err := appendTool(ctx, writeArg{Path: "a.txt", Content: "two\nthree\r\nfour"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "\ufeffone\r\ntwo\r\nthree\r\nfour" {
		t.Fatalf("got %q", got)
	}
}

func TestLoneCRSurvives(t *testing.T) {
	fs := newFS(t)
	ctx := threadCtx(fs)
	raw(t, fs, "cr.txt", "a\rb\r\nc\r\n")
	if _, err := editTool(ctx, editArg{Path: "cr.txt", Old: "c", New: "C"}); err != nil {
		t.Fatal(err)
	}
	if _, err := replaceTool(ctx, replaceArg{Path: "cr.txt", Pattern: "C", Replacement: "D"}); err != nil {
		t.Fatal(err)
	}
	if _, err := insertTool(ctx, insertArg{Path: "cr.txt", Line: 2, Text: "e\n"}); err != nil {
		t.Fatal(err)
	}
	if _, err := appendTool(ctx, writeArg{Path: "cr.txt", Content: "f\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "cr.txt"); got != "a\rb\r\nD\r\ne\r\nf\r\n" {
		t.Fatalf("got %q", got)
	}
}
