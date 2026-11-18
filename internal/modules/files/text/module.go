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
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"path"
	"regexp"
	"strings"
	"sync"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const writeDesc = `Write a whole text file, replacing existing content, creating parent directories. Content with \r keeps its line breaks as given.`
const appendDesc = `Append to the end of a text file, creating it if missing.`
const editDesc = `Replace text in a file; read the file first.`
const replaceDesc = `Replace text matching a regular expression.`
const insertDesc = `Insert lines into a text file.`

const bom = "\ufeff"

type pathLock struct {
	mu   sync.Mutex
	refs int
}

var (
	locksMu sync.Mutex
	locks   = map[string]*pathLock{}
)

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
	Old        string `json:"old" required:"true" description:"text to replace, matching the current content exactly, including whitespace and indentation"`
	New        string `json:"new" required:"true"`
	ReplaceAll bool   `json:"replaceAll" description:"replace every occurrence; without it old must match exactly once"`
}

type editResult struct {
	Replacements int `json:"replacements"`
}

type replaceArg struct {
	Path        string `json:"path" required:"true"`
	Pattern     string `json:"pattern" required:"true" description:"RE2 regular expression matched against the whole file, which it sees with lines ending in \n; (?s) lets . cross lines, (?m) makes ^ and $ match line boundaries"`
	Replacement string `json:"replacement" description:"may reference capture groups as $1, $2; write ${1} when a letter, digit or _ follows, e.g. ${1}_old; $$ is a literal dollar"`
	All         bool   `json:"all" description:"replace every match; without it the pattern must match exactly once"`
}

type insertArg struct {
	Path string `json:"path" required:"true"`
	Line uint   `json:"line" description:"0 inserts before the first line, N inserts after line N as shown by the read tool"`
	Text string `json:"text" required:"true"`
}

func lock(fs api.FS, name string) (func(), error) {
	tmp, err := fs.TempDir()
	if err != nil {
		return nil, err
	}
	key := tmp + "\x00" + path.Clean(name)
	locksMu.Lock()
	l := locks[key]
	if l == nil {
		l = &pathLock{}
		locks[key] = l
	}
	l.refs++
	locksMu.Unlock()
	l.mu.Lock()
	return func() {
		l.mu.Unlock()
		locksMu.Lock()
		if l.refs--; l.refs == 0 {
			delete(locks, key)
		}
		locksMu.Unlock()
	}, nil
}

func writeTool(ctx context.Context, arg writeArg) (writeResult, error) {
	fs := common.GetController(ctx).FS()
	unlock, err := lock(fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	defer unlock()
	s, exists, err := existingStyle(ctx, fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	switch {
	case exists && !strings.Contains(arg.Content, "\r"):
		arg.Content = s.format(arg.Content)
	case s.bom && !strings.HasPrefix(arg.Content, bom):
		arg.Content = bom + arg.Content
	}
	return put(ctx, fs, arg, true)
}

func appendTool(ctx context.Context, arg writeArg) (writeResult, error) {
	fs := common.GetController(ctx).FS()
	unlock, err := lock(fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	defer unlock()
	s, exists, err := existingStyle(ctx, fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	if exists {
		arg.Content = toLF(arg.Content)
		if s.crlf {
			arg.Content = strings.ReplaceAll(arg.Content, "\n", "\r\n")
		}
	}
	return put(ctx, fs, arg, false)
}

func put(ctx context.Context, fs api.FS, arg writeArg, rewrite bool) (writeResult, error) {
	var w io.WriteCloser
	var err error
	tmp := ""
	if rewrite {
		tmp = fmt.Sprintf("%s.tmp-%016x~", arg.Path, rand.Uint64())
		w, err = fs.Create(tmp)
		if errors.Is(err, api.ErrReadOnly) {
			err = fmt.Errorf("%q: %w", arg.Path, api.ErrReadOnly)
		} else if errors.Is(err, api.ErrQuota) {
			err = fmt.Errorf("%q: %w", arg.Path, api.ErrQuota)
		}
	} else {
		w, err = fs.Append(arg.Path)
	}
	if err != nil {
		return writeResult{}, err
	}
	_, err = io.WriteString(w, arg.Content)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil && rewrite {
		err = fs.Rename(tmp, arg.Path)
	}
	if err != nil {
		if rewrite {
			err = errors.Join(err, fs.Remove(context.WithoutCancel(ctx), tmp))
		}
		return writeResult{}, err
	}
	e, err := fs.Stat(ctx, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	return writeResult{Path: e.Name, Size: e.Size}, nil
}

func editTool(ctx context.Context, arg editArg) (editResult, error) {
	fs := common.GetController(ctx).FS()
	arg.Old, arg.New = toLF(arg.Old), toLF(arg.New)
	if arg.Old == "" {
		return editResult{}, fmt.Errorf("old is empty: use write to create or replace a whole file")
	}
	if arg.Old == arg.New {
		return editResult{}, fmt.Errorf("old and new are identical")
	}
	unlock, err := lock(fs, arg.Path)
	if err != nil {
		return editResult{}, err
	}
	defer unlock()
	content, s, err := readCapped(ctx, fs, arg.Path)
	if err != nil {
		return editResult{}, err
	}
	count := strings.Count(content, arg.Old)
	switch {
	case count == 0:
		return editResult{}, fmt.Errorf("old not found in %q: read the file and copy the exact text", arg.Path)
	case count > 1 && !arg.ReplaceAll:
		return editResult{}, fmt.Errorf("old matches %d places in %q: add surrounding lines to make it unique, or set replaceAll", count, arg.Path)
	}
	if !arg.ReplaceAll {
		count = 1
	}
	content = strings.Replace(content, arg.Old, arg.New, count)
	if _, err := put(ctx, fs, writeArg{Path: arg.Path, Content: s.format(content)}, true); err != nil {
		return editResult{}, err
	}
	return editResult{Replacements: count}, nil
}

func replaceTool(ctx context.Context, arg replaceArg) (editResult, error) {
	fs := common.GetController(ctx).FS()
	re, err := regexp.Compile(toLF(arg.Pattern))
	if err != nil {
		return editResult{}, fmt.Errorf("bad pattern: %w", err)
	}
	unlock, err := lock(fs, arg.Path)
	if err != nil {
		return editResult{}, err
	}
	defer unlock()
	content, s, err := readCapped(ctx, fs, arg.Path)
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
	content = re.ReplaceAllString(content, toLF(arg.Replacement))
	if _, err := put(ctx, fs, writeArg{Path: arg.Path, Content: s.format(content)}, true); err != nil {
		return editResult{}, err
	}
	return editResult{Replacements: count}, nil
}

func insertTool(ctx context.Context, arg insertArg) (writeResult, error) {
	fs := common.GetController(ctx).FS()
	unlock, err := lock(fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	defer unlock()
	content, s, err := readCapped(ctx, fs, arg.Path)
	if err != nil {
		return writeResult{}, err
	}
	arg.Text = toLF(arg.Text)
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
	return put(ctx, fs, writeArg{Path: arg.Path, Content: s.format(out)}, true)
}

func readCapped(ctx context.Context, fs api.FS, path string) (string, style, error) {
	limit := common.GetController(ctx).FilesConfig().EditLimit
	e, err := fs.Stat(ctx, path)
	if err != nil {
		return "", style{}, err
	}
	if e.Virtual {
		return "", style{}, api.ErrReadOnly
	}
	rc, err := fs.ReadAll(ctx, path)
	if err != nil {
		return "", style{}, err
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
		return "", style{}, err
	}
	if limit >= 0 && len(data) > limit {
		return "", style{}, fmt.Errorf("%q is over the %d-byte edit cap", path, limit)
	}
	raw := string(data)
	s := style{bom: strings.HasPrefix(raw, bom)}
	raw = strings.TrimPrefix(raw, bom)
	crlf := strings.Count(raw, "\r\n")
	s.crlf = crlf > strings.Count(raw, "\n")-crlf
	return toLF(raw), s, nil
}

func existingStyle(ctx context.Context, fs api.FS, path string) (style, bool, error) {
	e, err := fs.Stat(ctx, path)
	if errors.Is(err, api.ErrNotFound) {
		return style{}, false, nil
	}
	if err != nil {
		return style{}, false, err
	}
	if e.Virtual {
		return style{}, false, api.ErrReadOnly
	}
	if e.Size == 0 {
		return style{}, true, nil
	}
	rc, err := fs.ReadRange(ctx, path, 0, 64<<10)
	if err != nil {
		return style{}, false, err
	}
	head, err := io.ReadAll(rc)
	if cerr := rc.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return style{}, false, err
	}
	crlf := bytes.Count(head, []byte("\r\n"))
	return style{bom: bytes.HasPrefix(head, []byte(bom)), crlf: crlf > bytes.Count(head, []byte("\n"))-crlf}, true, nil
}

type style struct {
	bom  bool
	crlf bool
}

func (s style) format(lf string) string {
	if s.crlf {
		lf = strings.ReplaceAll(lf, "\n", "\r\n")
	}
	if s.bom && !strings.HasPrefix(lf, bom) {
		lf = bom + lf
	}
	return lf
}

func toLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

func splitLines(s string) []string {
	if s == "" {
		return nil
	}
	return strings.Split(strings.TrimSuffix(s, "\n"), "\n")
}
