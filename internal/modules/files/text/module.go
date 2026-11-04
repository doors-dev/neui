package text

import (
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const writeDesc = `Write a whole text file, replacing existing content, creating parent directories. Binary goes through writeBytes.`
const appendDesc = `Append to the end of a text file, creating it if missing.`
const editDesc = `Replace text in a file; check content with read first. old must match the current content byte for byte, including whitespace and indentation; the "N: " line prefixes shown by the read tool are not part of the content.`
const replaceDesc = `Replace text matching an RE2 regular expression. Without all the pattern must match exactly once; capture groups work in the replacement, e.g. $1.`
const insertDesc = `Insert text at a line position; 0 is the top of the file, N goes after line N from the read tool.`

func Register(m tools.Module) {
	add(m, "write", writeDesc, writeTool)
	add(m, "append", appendDesc, appendTool)
	add(m, "edit", editDesc, editTool)
	add(m, "replace", replaceDesc, replaceTool)
	add(m, "insert", insertDesc, insertTool)
}

func add[I any, O any](m tools.Module, name string, desc string, f func(ctx context.Context, input I) (O, error)) {
	if _, err := m.Add(name, desc, f); err != nil {
		panic(err)
	}
}

type writeArg struct {
	Path    string `json:"path" required:"true"`
	Content string `json:"content" required:"true"`
}

type writeResult struct {
	Path string `json:"path"`
	Size int64  `json:"size" description:"file size after the write"`
}

type editArg struct {
	Path       string `json:"path" required:"true"`
	Old        string `json:"old" required:"true" description:"exact text to replace"`
	New        string `json:"new" required:"true"`
	ReplaceAll bool   `json:"replaceAll" description:"replace every occurrence; without it old must match exactly once"`
}

type editResult struct {
	Replacements int `json:"replacements"`
}

type replaceArg struct {
	Path        string `json:"path" required:"true"`
	Pattern     string `json:"pattern" required:"true" description:"RE2 regular expression matched against the file content; (?s) lets . cross lines, (?m) makes ^ and $ match line boundaries"`
	Replacement string `json:"replacement" description:"may reference capture groups as $1, $2; $$ is a literal dollar"`
	All         bool   `json:"all" description:"replace every match; without it the pattern must match exactly once"`
}

type insertArg struct {
	Path string `json:"path" required:"true"`
	Line uint   `json:"line" description:"0 inserts before the first line, N inserts after line N as shown by the read tool"`
	Text string `json:"text" required:"true"`
}

func writeTool(ctx context.Context, arg writeArg) (writeResult, error) {
	fs := common.GetController(ctx).FS()
	return put(ctx, fs, fs.Create, arg)
}

func appendTool(ctx context.Context, arg writeArg) (writeResult, error) {
	fs := common.GetController(ctx).FS()
	return put(ctx, fs, fs.Append, arg)
}

func put(ctx context.Context, fs api.FS, open func(name string) (io.WriteCloser, error), arg writeArg) (writeResult, error) {
	w, err := open(arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	_, err = io.WriteString(w, arg.Content)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return writeResult{}, err
	}
	e, err := fs.Stat(ctx, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	return writeResult{Path: e.Name, Size: e.Size}, nil
}

func editTool(ctx context.Context, arg editArg) (editResult, error) {
	thread := common.GetController(ctx)
	fs := thread.FS()
	limit := thread.FilesConfig().EditLimit
	if arg.Old == "" {
		return editResult{}, fmt.Errorf("old is empty: use write to create or replace a whole file")
	}
	if arg.Old == arg.New {
		return editResult{}, fmt.Errorf("old and new are identical")
	}
	rc, err := fs.ReadAll(ctx, arg.Path)
	if err != nil {
		return editResult{}, err
	}
	var data []byte
	if limit < 0 {
		data, err = io.ReadAll(rc)
	} else {
		data, err = io.ReadAll(io.LimitReader(rc, int64(limit)+1))
	}
	if cerr := rc.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return editResult{}, err
	}
	if limit >= 0 && len(data) > limit {
		return editResult{}, fmt.Errorf("%q is over the %d-byte edit cap", arg.Path, limit)
	}
	content := string(data)
	count := strings.Count(content, arg.Old)
	switch {
	case count == 0:
		return editResult{}, fmt.Errorf("old not found in %q: it must match the file byte for byte, read the file and copy the exact text", arg.Path)
	case count > 1 && !arg.ReplaceAll:
		return editResult{}, fmt.Errorf("old matches %d places in %q: add surrounding lines to make it unique, or set replaceAll", count, arg.Path)
	}
	if !arg.ReplaceAll {
		count = 1
	}
	content = strings.Replace(content, arg.Old, arg.New, count)
	w, err := fs.Create(arg.Path)
	if err != nil {
		return editResult{}, err
	}
	_, err = io.WriteString(w, content)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return editResult{}, err
	}
	return editResult{Replacements: count}, nil
}

func replaceTool(ctx context.Context, arg replaceArg) (editResult, error) {
	fs := common.GetController(ctx).FS()
	re, err := regexp.Compile(arg.Pattern)
	if err != nil {
		return editResult{}, fmt.Errorf("bad pattern: %w", err)
	}
	content, err := readCapped(ctx, fs, arg.Path)
	if err != nil {
		return editResult{}, err
	}
	count := len(re.FindAllStringIndex(content, -1))
	switch {
	case count == 0:
		return editResult{}, fmt.Errorf("pattern matches nothing in %q: check it against the file with the read tool", arg.Path)
	case count > 1 && !arg.All:
		return editResult{}, fmt.Errorf("pattern matches %d places in %q: tighten it or set all", count, arg.Path)
	}
	content = re.ReplaceAllString(content, arg.Replacement)
	w, err := fs.Create(arg.Path)
	if err != nil {
		return editResult{}, err
	}
	_, err = io.WriteString(w, content)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return editResult{}, err
	}
	return editResult{Replacements: count}, nil
}

func insertTool(ctx context.Context, arg insertArg) (writeResult, error) {
	fs := common.GetController(ctx).FS()
	content, err := readCapped(ctx, fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	lines := splitLines(content)
	if arg.Line > uint(len(lines)) {
		return writeResult{}, fmt.Errorf("line %d is beyond %q, %d lines", arg.Line, arg.Path, len(lines))
	}
	endsNL := strings.HasSuffix(content, "\n")
	if content == "" {
		endsNL = strings.HasSuffix(arg.Text, "\n")
	}
	spliced := make([]string, 0, len(lines)+1)
	spliced = append(spliced, lines[:arg.Line]...)
	spliced = append(spliced, splitLines(arg.Text)...)
	spliced = append(spliced, lines[arg.Line:]...)
	out := strings.Join(spliced, "\n")
	if endsNL {
		out += "\n"
	}
	return put(ctx, fs, fs.Create, writeArg{Path: arg.Path, Content: out})
}

func readCapped(ctx context.Context, fs api.FS, path string) (string, error) {
	limit := common.GetController(ctx).FilesConfig().EditLimit
	rc, err := fs.ReadAll(ctx, path)
	if err != nil {
		return "", err
	}
	var data []byte
	if limit < 0 {
		data, err = io.ReadAll(rc)
	} else {
		data, err = io.ReadAll(io.LimitReader(rc, int64(limit)+1))
	}
	if cerr := rc.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return "", err
	}
	if limit >= 0 && len(data) > limit {
		return "", fmt.Errorf("%q is over the %d-byte edit cap", path, limit)
	}
	return string(data), nil
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
