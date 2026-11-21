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

package text

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"testing"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/testkit"
	"neui.dev/internal/tools"
)

func content(t *testing.T, fs api.FS, name string) string {
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
	full := []string{"append", "edit", "insert", "replace", "write"}
	for tier, want := range map[common.ModelTier][]string{common.TierDeveloper: full, common.TierAgent: full, common.TierAssistant: {"edit", "write"}} {
		m := tools.NewModule("files", "")
		Register(m, tier)
		var names []string
		for _, tool := range m.List() {
			names = append(names, tool.Name())
		}
		if !slices.Equal(names, want) {
			t.Fatalf("tier %d registers %v, want %v", tier, names, want)
		}
	}
}

func TestWriteAppend(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	res, err := writeTool(c.Ctx(), writeArg{Path: "notes/a.txt", Content: "one\n"})
	if err != nil || res.Size != 4 || res.Path != "notes/a.txt" {
		t.Fatalf("%+v, %v", res, err)
	}
	res, err = appendTool(c.Ctx(), writeArg{Path: "notes/a.txt", Content: "two\n"})
	if err != nil || res.Size != 8 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "notes/a.txt"); got != "one\ntwo\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEdit(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	edit := editTool
	writeTool(c.Ctx(), writeArg{Path: "a.txt", Content: "aa bb aa\n"})
	res, err := edit(c.Ctx(), editArg{Path: "a.txt", Old: "bb", New: "XX"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "a.txt"); got != "aa XX aa\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := edit(c.Ctx(), editArg{Path: "a.txt", Old: "aa", New: "y"}); err == nil || !strings.Contains(err.Error(), "2 places") {
		t.Fatalf("ambiguous: %v", err)
	}
	res, err = edit(c.Ctx(), editArg{Path: "a.txt", Old: "aa", New: "y", ReplaceAll: true})
	if err != nil || res.Replacements != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if _, err := edit(c.Ctx(), editArg{Path: "a.txt", Old: "gone", New: "y"}); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := edit(c.Ctx(), editArg{Path: "a.txt", Old: "", New: "y"}); err == nil {
		t.Fatal("empty old accepted")
	}
	if _, err := edit(c.Ctx(), editArg{Path: "a.txt", Old: "y", New: "y"}); err == nil {
		t.Fatal("identical accepted")
	}
	if _, err := edit(c.Ctx(), editArg{Path: "user_uploads/x.txt", Old: "a", New: "b"}); err == nil {
		t.Fatal("uploads path accepted")
	}
}

func TestReplace(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	replace := replaceTool
	writeTool(c.Ctx(), writeArg{Path: "r.txt", Content: "aa bb aa\n"})
	res, err := replace(c.Ctx(), replaceArg{Path: "r.txt", Pattern: "b+", Replacement: "XX"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "r.txt"); got != "aa XX aa\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := replace(c.Ctx(), replaceArg{Path: "r.txt", Pattern: "aa", Replacement: "y"}); err == nil || !strings.Contains(err.Error(), "2 places") {
		t.Fatalf("ambiguous: %v", err)
	}
	res, err = replace(c.Ctx(), replaceArg{Path: "r.txt", Pattern: "aa", Replacement: "y", All: true})
	if err != nil || res.Replacements != 2 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "r.txt"); got != "y XX y\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := replace(c.Ctx(), replaceArg{Path: "r.txt", Pattern: "zzz", Replacement: "y"}); err == nil || !strings.Contains(err.Error(), "matches nothing") {
		t.Fatalf("missing: %v", err)
	}
	if _, err := replace(c.Ctx(), replaceArg{Path: "r.txt", Pattern: "(", Replacement: "y"}); err == nil || !strings.Contains(err.Error(), "bad pattern") {
		t.Fatalf("invalid: %v", err)
	}
	writeTool(c.Ctx(), writeArg{Path: "g.txt", Content: "name=alex\n"})
	res, err = replace(c.Ctx(), replaceArg{Path: "g.txt", Pattern: `name=(\w+)`, Replacement: "user=$1"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "g.txt"); got != "user=alex\n" {
		t.Fatalf("got %q", got)
	}
	writeTool(c.Ctx(), writeArg{Path: "s.txt", Content: "foo\nmid\nbar tail\n"})
	res, err = replace(c.Ctx(), replaceArg{Path: "s.txt", Pattern: "(?s)foo.*?bar", Replacement: "span"})
	if err != nil || res.Replacements != 1 {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "s.txt"); got != "span tail\n" {
		t.Fatalf("got %q", got)
	}
}

func TestEditsWithTheCapTurnedOff(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{EditLimit: -1}})
	fs := c.FS()
	ctx := c.Ctx()
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	insert := insertTool
	writeTool(c.Ctx(), writeArg{Path: "i.txt", Content: "one\ntwo\nthree\n"})
	res, err := insert(c.Ctx(), insertArg{Path: "i.txt", Line: 0, Text: "zero"})
	if err != nil || res.Size != 19 || res.Path != "i.txt" {
		t.Fatalf("%+v, %v", res, err)
	}
	if got := content(t, fs, "i.txt"); got != "zero\none\ntwo\nthree\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := insert(c.Ctx(), insertArg{Path: "i.txt", Line: 2, Text: "mid\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "i.txt"); got != "zero\none\nmid\ntwo\nthree\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := insert(c.Ctx(), insertArg{Path: "i.txt", Line: 5, Text: "end"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "i.txt"); got != "zero\none\nmid\ntwo\nthree\nend\n" {
		t.Fatalf("got %q", got)
	}
	if _, err := insert(c.Ctx(), insertArg{Path: "i.txt", Line: 9, Text: "x"}); err == nil || !strings.Contains(err.Error(), "6 lines") {
		t.Fatalf("beyond: %v", err)
	}
	writeTool(c.Ctx(), writeArg{Path: "n.txt", Content: "tail"})
	if _, err := insert(c.Ctx(), insertArg{Path: "n.txt", Line: 0, Text: "head\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "n.txt"); got != "head\ntail" {
		t.Fatalf("got %q", got)
	}
	writeTool(c.Ctx(), writeArg{Path: "e.txt", Content: ""})
	if _, err := insert(c.Ctx(), insertArg{Path: "e.txt", Line: 0, Text: "first\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "e.txt"); got != "first\n" {
		t.Fatalf("got %q", got)
	}
}

func raw(t *testing.T, fs api.FS, name, s string) {
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
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
	if got := content(t, fs, "w.txt"); got != "\ufeffas\r\nis\n" {
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
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
	raw(t, fs, "a.txt", "\ufeffone\r\n")
	if _, err := appendTool(ctx, writeArg{Path: "a.txt", Content: "two\nthree\r\nfour"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "\ufeffone\r\ntwo\r\nthree\r\nfour" {
		t.Fatalf("got %q", got)
	}
}

func TestLoneCRSurvives(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
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

func TestStyleComesFromTheHead(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
	head := strings.Repeat("crlf line\r\n", 70<<10/11)
	raw(t, fs, "log.txt", head+strings.Repeat("lf\n", 100<<10))
	if _, err := appendTool(ctx, writeArg{Path: "log.txt", Content: "x\ny\n"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "log.txt"); !strings.HasSuffix(got, "lf\nx\r\ny\r\n") {
		t.Fatalf("tail %q", got[len(got)-20:])
	}
}

func TestWriteToAnUploadIsReadOnly(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	store := c.Store()
	if _, err := store.FilePut(context.Background(), "thread", api.OriginUser, "up.txt", "text/plain", strings.NewReader("a\r\n"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := writeTool(c.Ctx(), writeArg{Path: "user_uploads/up.txt", Content: "b\n"}); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("write: %v", err)
	}
	if _, err := appendTool(c.Ctx(), writeArg{Path: "user_uploads/up.txt", Content: "b\n"}); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("append: %v", err)
	}
}

func TestRewriteErrorsNameThePath(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{MaxFiles: 1}})
	fs := c.FS()
	ctx := c.Ctx()
	if _, err := writeTool(ctx, writeArg{Path: "agent_uploads/r.md", Content: "b\n"}); !errors.Is(err, api.ErrReadOnly) || !strings.HasPrefix(err.Error(), `"agent_uploads/r.md": `) {
		t.Fatalf("upload: %v", err)
	}
	raw(t, fs, "a.txt", "0123456789\n")
	if _, err := writeTool(ctx, writeArg{Path: "a.txt", Content: "y\n"}); !errors.Is(err, api.ErrQuota) || !strings.HasPrefix(err.Error(), `"a.txt": `) || strings.Contains(err.Error(), ".tmp-") {
		t.Fatalf("quota: %v", err)
	}
}

func TestFailedRewriteKeepsTheOriginal(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{MaxBytes: 30}})
	fs := c.FS()
	ctx := c.Ctx()
	raw(t, fs, "a.txt", "0123456789\n")
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "0123456789", New: strings.Repeat("x", 25)}); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("edit: %v", err)
	}
	if _, err := writeTool(ctx, writeArg{Path: "a.txt", Content: strings.Repeat("y", 25)}); !errors.Is(err, api.ErrQuota) {
		t.Fatalf("write: %v", err)
	}
	if got := content(t, fs, "a.txt"); got != "0123456789\n" {
		t.Fatalf("got %q", got)
	}
	entries, err := fs.ReadDir(ctx, ".")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !e.Dir && e.Name != "a.txt" {
			t.Fatalf("left behind %q", e.Name)
		}
	}
	if _, err := editTool(ctx, editArg{Path: "a.txt", Old: "0123", New: "abcd"}); err != nil {
		t.Fatal(err)
	}
	if got := content(t, fs, "a.txt"); got != "abcd456789\n" {
		t.Fatalf("got %q", got)
	}
}

func TestConcurrentEditsOnOnePathSerialize(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{})
	fs := c.FS()
	ctx := c.Ctx()
	raw(t, fs, "a.txt", "end\n")
	const n = 30
	errs := make(chan error, n)
	for i := range n {
		go func() {
			_, err := insertTool(ctx, insertArg{Path: []string{"a.txt", "/a.txt", "file:///a.txt"}[i%3], Line: 0, Text: "line\n"})
			errs <- err
		}()
	}
	for range n {
		if err := <-errs; err != nil {
			t.Fatal(err)
		}
	}
	if got := strings.Count(content(t, fs, "a.txt"), "line\n"); got != n {
		t.Fatalf("got %d lines, want %d", got, n)
	}
}

func TestEditOfAnUploadIsReadOnlyBeforeReading(t *testing.T) {
	c := testkit.NewController(t, testkit.Setup{Files: api.FilesConfig{EditLimit: 4}})
	store, ctx := c.Store(), c.Ctx()
	if _, err := store.FilePut(context.Background(), "thread", api.OriginUser, "up.txt", "text/plain", strings.NewReader("a long upload\n"), false); err != nil {
		t.Fatal(err)
	}
	if _, err := editTool(ctx, editArg{Path: "user_uploads/up.txt", Old: "long", New: "short"}); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("edit: %v", err)
	}
	if _, err := insertTool(ctx, insertArg{Path: "user_uploads/up.txt", Text: "x\n"}); !errors.Is(err, api.ErrReadOnly) {
		t.Fatalf("insert: %v", err)
	}
}
