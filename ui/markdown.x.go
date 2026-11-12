// Managed by GoX v0.3.4

//line markdown.gox:1
package ui

import (
	"bytes"
	"context"
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	xhtml "golang.org/x/net/html"
	"neui.dev/api"
	"neui.dev/internal/markdown"
)

func markdownBody(fs api.FS, data []byte) gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		if !bytes.Contains(data, []byte(markdown.FileScheme)) {
			return cur.Bytes(data)
		}
		z := xhtml.NewTokenizer(bytes.NewReader(data))
		var pend []byte
		flush := func() error {
			if len(pend) == 0 {
				return nil
			}
			err := cur.Bytes(pend)
			pend = nil
			return err
		}
		for {
			tt := z.Next()
			if tt == xhtml.ErrorToken {
				if err := z.Err(); err != io.EOF {
					return err
				}
				return flush()
			}
			if tt != xhtml.StartTagToken && tt != xhtml.SelfClosingTagToken {
				pend = append(pend, z.Raw()...)
				continue
			}
			raw := append([]byte(nil), z.Raw()...)
			name, hasAttr := z.TagName()
			tag := string(name)
			attrs := map[string]string{}
			for hasAttr {
				k, v, more := z.TagAttr()
				attrs[string(k)] = string(v)
				hasAttr = more
			}
			if file, ok := markdown.FilePath(attrs["href"]); ok && tag == "a" {
				var text strings.Builder
			link:
				for {
					switch z.Next() {
					case xhtml.ErrorToken:
						break link
					case xhtml.TextToken:
						text.Write(z.Text())
					case xhtml.EndTagToken:
						if t, _ := z.TagName(); string(t) == "a" {
							break link
						}
					}
				}
				if err := flush(); err != nil {
					return err
				}
				if err := cur.Comp(markdownFile{fs: fs, name: file, label: markdown.FileLabel(text.String(), attrs["href"], file)}); err != nil {
					return err
				}
				continue
			}
			if file, ok := markdown.FilePath(attrs["src"]); ok && tag == "img" {
				if err := flush(); err != nil {
					return err
				}
				if err := cur.Comp(markdownImage{fs: fs, name: file, alt: attrs["alt"], title: attrs["title"]}); err != nil {
					return err
				}
				continue
			}
			pend = append(pend, raw...)
		}
	})
}

type markdownFile struct {
	fs api.FS
	name string
	label string
}

//line markdown.gox:99
func (m markdownFile) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line markdown.gox:101
		entry, err := markdownStat(ctx, m.fs, m.name)

//line markdown.gox:103
		if err != nil {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line markdown.gox:104
				__e = __c.Set("class", "kisland-file kisland-file-gone not-prose"); if __e != nil { return }
//line markdown.gox:104
				__e = __c.Set("title", m.name); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line markdown.gox:105
					__e = __c.Set("class", "kisland-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line markdown.gox:105
					__e = __c.Any(m.label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line markdown.gox:106
				__e = __c.Any(" "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line markdown.gox:107
					__e = __c.Set("class", "kisland-file-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line markdown.gox:108
					__e = __c.Any(func() any {
					if errors.Is(err, api.ErrNotFound) {
						return "file is gone"
					}
					return "file unavailable"
				}()); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("a"); if __e != nil { return }
			{
//line markdown.gox:117
				__e = __c.Set("class", "kisland-file not-prose"); if __e != nil { return }
//line markdown.gox:117
				__e = __c.Set("href", markdownServe(m.fs, m.name, true)); if __e != nil { return }
//line markdown.gox:117
				__e = __c.Set("download", path.Base(m.name)); if __e != nil { return }
//line markdown.gox:117
				__e = __c.Set("title", m.name); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("svg"); if __e != nil { return }
				{
//line markdown.gox:118
					__e = __c.Set("class", "kisland-file-icon"); if __e != nil { return }
//line markdown.gox:118
					__e = __c.Set("viewBox", "0 0 12 12"); if __e != nil { return }
//line markdown.gox:118
					__e = __c.Set("fill", "none"); if __e != nil { return }
//line markdown.gox:118
					__e = __c.Set("stroke", "currentColor"); if __e != nil { return }
//line markdown.gox:118
					__e = __c.Set("stroke-width", "1.4"); if __e != nil { return }
//line markdown.gox:118
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("path"); if __e != nil { return }
					{
//line markdown.gox:119
						__e = __c.Set("d", "M6 1.5v6M3.25 5 6 7.75 8.75 5M2 10.5h8"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line markdown.gox:121
					__e = __c.Set("class", "kisland-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line markdown.gox:121
					__e = __c.Any(m.label); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line markdown.gox:122
				__e = __c.Any(" "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line markdown.gox:123
					__e = __c.Set("class", "kisland-file-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line markdown.gox:123
					__e = __c.Any(formFileSize(entry.Size)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line markdown.gox:126
}

type markdownImage struct {
	fs api.FS
	name string
	alt string
	title string
}

//line markdown.gox:135
func (m markdownImage) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line markdown.gox:137
		_, err := markdownStat(ctx, m.fs, m.name)

//line markdown.gox:139
		if err != nil {
//line markdown.gox:140
			__e = __c.Any(markdownFile{fs: m.fs, name: m.name, label: markdown.FileLabel(m.alt, "", m.name)}); if __e != nil { return }
		} else  {
			__e = __c.InitVoid("img"); if __e != nil { return }
			{
//line markdown.gox:143
				__e = __c.Set("src", markdownServe(m.fs, m.name, false)); if __e != nil { return }
//line markdown.gox:144
				__e = __c.Set("alt", m.alt); if __e != nil { return }
//line markdown.gox:145
				__e = __c.Set("title", func() any {
				if m.title == "" {
					return nil
				}
				return m.title
			}()); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
		}
	return })
//line markdown.gox:152
}

func markdownStat(ctx context.Context, fs api.FS, name string) (api.FSEntry, error) {
	if fs == nil {
		return api.FSEntry{}, errors.New("no workspace")
	}
	entry, err := fs.Stat(ctx, name)
	if err != nil {
		if !errors.Is(err, api.ErrNotFound) && !errors.Is(err, api.ErrReserved) && !errors.Is(err, api.ErrInvalidName) {
			doors.Logger(ctx).Error("ui: markdown file", "name", name, "err", err)
		}
		return entry, err
	}
	if entry.Dir {
		return entry, errors.New("a directory")
	}
	return entry, nil
}

func markdownServe(fs api.FS, name string, download bool) doors.Resource {
	return doors.ResourceHandler(func(w http.ResponseWriter, r *http.Request) {
		e, err := fs.Stat(r.Context(), name)
		if err != nil || e.Dir {
			http.NotFound(w, r)
			return
		}
		rc, err := fs.ReadAll(r.Context(), name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer rc.Close()
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Content-Security-Policy", "sandbox")
		if e.MIME != "" {
			w.Header().Set("Content-Type", e.MIME)
		}
		if e.Size > 0 {
			w.Header().Set("Content-Length", strconv.FormatInt(e.Size, 10))
		}
		if download {
			w.Header().Set("Content-Disposition", "attachment; filename=" + strconv.Quote(path.Base(name)))
		}
		io.Copy(w, rc)
	})
}
