package marksman

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"path"
	"strconv"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"golang.org/x/net/html"
	"neui.dev/api"
	"neui.dev/kchat/chat/uploader"
)

const fileScheme = "file:///"

func chunk(fs api.FS, data []byte) gox.Elem {
	return gox.Elem(func(cur gox.Cursor) error {
		if fs == nil || !bytes.Contains(data, []byte(fileScheme)) {
			return cur.Bytes(data)
		}
		z := html.NewTokenizer(bytes.NewReader(data))
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
			if tt == html.ErrorToken {
				if err := z.Err(); err != io.EOF {
					return err
				}
				return flush()
			}
			if tt != html.StartTagToken && tt != html.SelfClosingTagToken {
				pend = append(pend, z.Raw()...)
				continue
			}
			name, hasAttr := z.TagName()
			tag := string(name)
			if !hasAttr || (tag != "a" && tag != "img") {
				pend = append(pend, z.Raw()...)
				continue
			}
			raw := append([]byte(nil), z.Raw()...)
			attrs := map[string]string{}
			for {
				k, v, more := z.TagAttr()
				attrs[string(k)] = string(v)
				if !more {
					break
				}
			}
			if tag == "a" {
				if file, ok := fileName(attrs["href"]); ok {
					if err := flush(); err != nil {
						return err
					}
					if err := renderFileLink(cur, fs, z, file); err != nil {
						return err
					}
					continue
				}
			}
			if tag == "img" {
				if file, ok := fileName(attrs["src"]); ok {
					if err := flush(); err != nil {
						return err
					}
					if err := renderFileImage(cur, fs, file, attrs["alt"]); err != nil {
						return err
					}
					continue
				}
			}
			pend = append(pend, raw...)
		}
	})
}

func fileName(href string) (string, bool) {
	rest, ok := strings.CutPrefix(href, fileScheme)
	if !ok {
		return "", false
	}
	name, err := url.PathUnescape(rest)
	if err != nil || name == "" {
		return "", false
	}
	name = path.Clean(name)
	if !strings.HasPrefix(name, api.UserUploadsDir+"/") && !strings.HasPrefix(name, api.AgentUploadsDir+"/") {
		return "", false
	}
	return name, true
}

func renderFileLink(cur gox.Cursor, fs api.FS, z *html.Tokenizer, name string) error {
loop:
	for {
		switch z.Next() {
		case html.ErrorToken:
			break loop
		case html.EndTagToken:
			t, _ := z.TagName()
			if string(t) == "a" {
				break loop
			}
		}
	}
	return cur.Comp(uploader.Sent{FS: fs, Name: name})
}

func renderFileImage(cur gox.Cursor, fs api.FS, name string, alt string) error {
	if err := cur.InitVoid("img"); err != nil {
		return err
	}
	if err := cur.Set("src", serveFile(fs, name)); err != nil {
		return err
	}
	if alt != "" {
		if err := cur.Set("alt", alt); err != nil {
			return err
		}
	}
	return cur.Submit()
}

func serveFile(fs api.FS, name string) doors.Resource {
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
		io.Copy(w, rc)
	})
}
