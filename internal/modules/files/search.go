package files

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"regexp"
	"strings"

	"neui.dev/api"
	"neui.dev/internal/common"
)

const (
	grepDefaultLimit = 100
	grepMaxLimit     = 1000
	grepLineMax      = 400
	grepMatchMax     = 1 << 20
	globCap          = 100
)

const grepDesc = `Search file contents line by line with a regular expression. The walk skips the uploads dirs; point path at one of them or a file inside to search there — every file scanned in an uploads dir is downloaded whole.`
const globDesc = `Find files by path pattern.`

type grepArg struct {
	Pattern string `json:"pattern" required:"true" description:"RE2 regular expression matched against single lines"`
	Path    string `json:"path" description:"file or directory to search, empty string is everything"`
	Glob    string `json:"glob" description:"only search files with matching paths, e.g. **/*.go"`
	Limit   uint   `json:"limit" description:"max matches, default 100"`
}

type grepMatch struct {
	Path string `json:"path"`
	Line uint   `json:"line"`
	Text string `json:"text"`
}

type grepResult struct {
	Matches   []grepMatch `json:"matches"`
	Truncated bool        `json:"truncated,omitempty" description:"present when the limit cut the results, narrow with path or glob"`
}

type globArg struct {
	Pattern string `json:"pattern" required:"true" description:"path pattern, * stays within a path segment, ** crosses directories, e.g. **/*.csv"`
	Path    string `json:"path" description:"directory to search under, empty string is everything"`
}

type globResult struct {
	Paths     []string `json:"paths"`
	Truncated bool     `json:"truncated,omitempty" description:"present when over 100 paths match, narrow the pattern"`
}

func grepTool(ctx context.Context, arg grepArg) (grepResult, error) {
	fs := common.GetController(ctx).FS()
	re, err := regexp.Compile(arg.Pattern)
	if err != nil {
		return grepResult{}, fmt.Errorf("bad pattern: %w", err)
	}
	var globRe *regexp.Regexp
	if arg.Glob != "" {
		if globRe, err = globRegexp(arg.Glob); err != nil {
			return grepResult{}, err
		}
	}
	limit := min(max(arg.Limit, 1), grepMaxLimit)
	if arg.Limit == 0 {
		limit = grepDefaultLimit
	}
	res := grepResult{Matches: []grepMatch{}}
	scan := func(name string) (bool, error) {
		if globRe != nil && !globRe.MatchString(name) {
			return true, nil
		}
		if err := grepFile(ctx, fs, name, re, limit, &res); err != nil {
			return false, err
		}
		return !res.Truncated, nil
	}
	if arg.Path != "" {
		e, err := fs.Stat(ctx, arg.Path)
		if err != nil {
			return grepResult{}, err
		}
		if !e.Dir {
			_, err := scan(e.Name)
			return res, err
		}
	}
	if err := walkFiles(ctx, fs, arg.Path, false, func(e api.FSEntry) (bool, error) {
		return scan(e.Name)
	}); err != nil {
		return grepResult{}, err
	}
	return res, nil
}

func grepFile(ctx context.Context, fs api.FS, name string, re *regexp.Regexp, limit uint, res *grepResult) error {
	rc, err := fs.ReadAll(ctx, name)
	if err != nil {
		return err
	}
	defer rc.Close()
	br := bufio.NewReaderSize(rc, 64<<10)
	head, _ := br.Peek(4096)
	if bytes.IndexByte(head, 0) >= 0 {
		return nil
	}
	var line uint
	for {
		raw, _, err := readLine(br, grepMatchMax)
		if err == io.EOF {
			return nil
		}
		if err != nil {
			return err
		}
		line++
		text := string(raw)
		if !re.MatchString(text) {
			continue
		}
		if uint(len(res.Matches)) == limit {
			res.Truncated = true
			return nil
		}
		if len(text) > grepLineMax {
			text = text[:grepLineMax] + "... (line truncated)"
		}
		res.Matches = append(res.Matches, grepMatch{Path: name, Line: line, Text: text})
	}
}

func globTool(ctx context.Context, arg globArg) (globResult, error) {
	fs := common.GetController(ctx).FS()
	re, err := globRegexp(arg.Pattern)
	if err != nil {
		return globResult{}, err
	}
	res := globResult{Paths: []string{}}
	err = walkFiles(ctx, fs, arg.Path, true, func(e api.FSEntry) (bool, error) {
		if !re.MatchString(e.Name) {
			return true, nil
		}
		if len(res.Paths) == globCap {
			res.Truncated = true
			return false, nil
		}
		res.Paths = append(res.Paths, e.Name)
		return true, nil
	})
	if err != nil {
		return globResult{}, err
	}
	return res, nil
}

func walkFiles(ctx context.Context, fs api.FS, root string, virtual bool, fn func(e api.FSEntry) (bool, error)) error {
	entries, err := fs.ReadDir(ctx, root)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.Dir {
			if !virtual && e.Virtual {
				continue
			}
			if err := walkFiles(ctx, fs, e.Name, virtual, fn); err != nil {
				return err
			}
			continue
		}
		cont, err := fn(e)
		if err != nil || !cont {
			return err
		}
	}
	return nil
}

func globRegexp(pattern string) (*regexp.Regexp, error) {
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch pattern[i] {
		case '*':
			if i+1 < len(pattern) && pattern[i+1] == '*' {
				i++
				if i+1 < len(pattern) && pattern[i+1] == '/' {
					i++
					b.WriteString(`(?:[^/]*/)*`)
				} else {
					b.WriteString(`.*`)
				}
			} else {
				b.WriteString(`[^/]*`)
			}
		case '?':
			b.WriteString(`[^/]`)
		default:
			b.WriteString(regexp.QuoteMeta(string(pattern[i])))
		}
	}
	b.WriteString("$")
	re, err := regexp.Compile(b.String())
	if err != nil {
		return nil, fmt.Errorf("bad glob %q: %w", pattern, err)
	}
	return re, nil
}
