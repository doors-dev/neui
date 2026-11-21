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

package files

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"strings"
	"unicode/utf8"

	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const (
	readMaxLines = 2000
	readLineMax  = 2000
	bytesCap     = 8 << 20
)

func readDesc(light bool) string {
	desc := `Read a text file as numbered lines; everything after "N: " is file content.`
	if light {
		return desc + ` A long file comes in parts, and the last line says where to continue.`
	}
	return desc + ` Binary files are refused; use readBytes. Output stops at the line limit or the configured byte limit, whichever comes first, and its last line says where to continue. From a script it returns the file's text as it is, up to 8MB: no line numbers, no closing line, no truncated lines, and the whole file when limit is unset.`
}

const readBytesDesc = `Read raw bytes of a file, base64 in the result. Reads over 8MB must be ranged.`
const writeBytesDesc = `Write raw bytes to a file, replacing existing content, creating parent directories. Plain text goes through write.`

type readArg struct {
	Path   string `json:"path" required:"true"`
	Offset uint   `json:"offset" description:"1-based line to start from, for continuing a long file"`
	Limit  uint   `json:"limit" description:"max lines to return, default 2000"`
}

type readBytesArg struct {
	Path   string `json:"path" required:"true"`
	Offset int64  `json:"offset" description:"byte offset to start from"`
	N      int64  `json:"n" description:"bytes to read, 0 reads to the end of the file"`
}

type writeBytesArg struct {
	Path string `json:"path" required:"true"`
	Data []byte `json:"data" required:"true" description:"base64 in direct calls"`
}

func readTool(ctx context.Context, arg readArg) (string, error) {
	thread := common.GetController(ctx)
	rc, err := thread.FS().ReadAll(ctx, arg.Path)
	if err != nil {
		return "", err
	}
	defer rc.Close()
	br := bufio.NewReaderSize(ctxReader{ctx, rc}, 64<<10)
	head, _ := br.Peek(4096)
	if bytes.IndexByte(head, 0) >= 0 {
		if common.LightMode(ctx) {
			return "", fmt.Errorf("%q is not a text file: look at an image with readImage, read a PDF or Office file with readDocument", arg.Path)
		}
		return "", fmt.Errorf("%q is a binary file, use readBytes", arg.Path)
	}
	if bytes.HasPrefix(head, []byte("\ufeff")) {
		br.Discard(3)
	}
	if tools.FromJS(ctx) {
		data, err := io.ReadAll(io.LimitReader(br, bytesCap+1))
		if err != nil {
			return "", err
		}
		if len(data) > bytesCap {
			return "", fmt.Errorf("%q is larger than 8MB, read it in ranges with readBytes", arg.Path)
		}
		if arg.Offset <= 1 && arg.Limit == 0 {
			return string(data), nil
		}
		lines := strings.SplitAfter(string(data), "\n")
		if lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		start := max(arg.Offset, 1)
		if start > uint(len(lines)) {
			return "", fmt.Errorf("offset %d is beyond the end of %q, %d lines", start, arg.Path, len(lines))
		}
		end := uint(len(lines))
		if arg.Limit > 0 {
			end = min(end, start-1+arg.Limit)
		}
		return strings.Join(lines[start-1:end], ""), nil
	}
	offset := max(arg.Offset, 1)
	limit := arg.Limit
	if limit == 0 {
		limit = readMaxLines
	}
	var b strings.Builder
	var total, shown, last uint
	budget := thread.FilesConfig().ReadLimit
	capped := false
	for {
		raw, cut, err := readLine(br, readLineMax)
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", err
		}
		total++
		if total < offset || shown == limit || capped {
			continue
		}
		line := string(raw)
		if cut {
			line += "... (line truncated)"
		}
		fmt.Fprintf(&b, "%d: %s\n", total, line)
		if budget >= 0 {
			budget -= len(line)
			if budget <= 0 {
				capped = true
			}
		}
		shown++
		last = total
	}
	if total == 0 {
		return "(empty file)", nil
	}
	if offset > total {
		return "", fmt.Errorf("offset %d is beyond the end of %q, %d lines", offset, arg.Path, total)
	}
	if last == total {
		fmt.Fprintf(&b, "(end of file, %d lines)", total)
	} else {
		fmt.Fprintf(&b, "(showing lines %d-%d of %d, continue with offset=%d)", offset, last, total, last+1)
	}
	return b.String(), nil
}

func readLine(br *bufio.Reader, keep int) ([]byte, bool, error) {
	var line []byte
	cut := false
	started := false
	for {
		chunk, more, err := br.ReadLine()
		if err != nil {
			if err == io.EOF && started {
				return line, cut, nil
			}
			return nil, false, err
		}
		started = true
		if cut {
			chunk = nil
		}
		if room := keep - len(line); len(chunk) > room {
			for room > 0 && !utf8.RuneStart(chunk[room]) {
				room--
			}
			chunk = chunk[:room]
			cut = true
		}
		line = append(line, chunk...)
		if !more {
			return line, cut, nil
		}
	}
}

func readBytesTool(ctx context.Context, arg readBytesArg) ([]byte, error) {
	fs := common.GetController(ctx).FS()
	if arg.N > bytesCap {
		return nil, fmt.Errorf("n %d exceeds the %dMB cap, read in ranges", arg.N, bytesCap>>20)
	}
	n := arg.N
	if n <= 0 {
		n = -1
	}
	rc, err := fs.ReadRange(ctx, arg.Path, arg.Offset, n)
	if err != nil {
		return nil, err
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, bytesCap+1))
	if err != nil {
		return nil, err
	}
	if len(data) > bytesCap {
		return nil, fmt.Errorf("%q is over the %dMB cap, read in ranges with offset and n", arg.Path, bytesCap>>20)
	}
	return data, nil
}

func writeBytesTool(ctx context.Context, arg writeBytesArg) (Entry, error) {
	fs := common.GetController(ctx).FS()
	tmp := fmt.Sprintf("%s.tmp-%016x~", arg.Path, rand.Uint64())
	w, err := fs.Create(tmp)
	if errors.Is(err, api.ErrReadOnly) {
		return Entry{}, fmt.Errorf("%q: %w", arg.Path, api.ErrReadOnly)
	}
	if errors.Is(err, api.ErrQuota) {
		return Entry{}, fmt.Errorf("%q: %w", arg.Path, api.ErrQuota)
	}
	if err != nil {
		return Entry{}, err
	}
	_, err = w.Write(arg.Data)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err == nil {
		err = fs.Rename(tmp, arg.Path)
	}
	if err != nil {
		return Entry{}, errors.Join(err, fs.Remove(context.WithoutCancel(ctx), tmp))
	}
	e, err := fs.Stat(ctx, arg.Path)
	if err != nil {
		return Entry{}, err
	}
	return entry(e), nil
}
