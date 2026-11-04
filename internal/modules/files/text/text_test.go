package text

import (
	"context"
	"io"
	"strings"
	"testing"

	"neui.dev/api"
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
