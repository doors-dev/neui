package files

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"strings"

	"neui.dev/internal/common"
)

const (
	readMaxLines = 2000
	readLineMax  = 2000
	bytesCap     = 8 << 20
)

const readDesc = `Read a text file as numbered lines; everything after "N: " is file content. Output stops at the line limit or at the configured byte limit, whichever comes first, and the last line of the result says where to continue from. Binary files are refused, use readBytes.`
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
	br := bufio.NewReaderSize(rc, 64<<10)
	head, _ := br.Peek(4096)
	if bytes.IndexByte(head, 0) >= 0 {
		return "", fmt.Errorf("%q is a binary file, use readBytes", arg.Path)
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
		if room := keep - len(line); len(chunk) > room {
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
	w, err := fs.Create(arg.Path)
	if err != nil {
		return Entry{}, err
	}
	_, err = w.Write(arg.Data)
	if cerr := w.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return Entry{}, err
	}
	e, err := fs.Stat(ctx, arg.Path)
	if err != nil {
		return Entry{}, err
	}
	return entry(e), nil
}
