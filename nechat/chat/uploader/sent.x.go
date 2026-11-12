// Managed by GoX v0.3.4

//line sent.gox:1
package uploader

import (
	"errors"
	"io"
	"net/http"
	"path"
	"strconv"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nechat/icons"
)

// Sent is an attachment of a sent message: a chip that downloads the file on
// click, or an error chip when the file can no longer be read.
type Sent struct {
	FS api.FS
	Name string
}

//line sent.gox:23
func (s Sent) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line sent.gox:25
		entry, err := s.FS.UploadStat(ctx, s.Name)
	text := ""
	if err != nil {
		text = "file unavailable"
		if errors.Is(err, api.ErrNotFound) {
			text = "file is gone"
		} else {
			doors.Logger(ctx).Error("nechat: sent file", "name", s.Name, "err", err)
		}
	}

//line sent.gox:36
		if err != nil {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line sent.gox:37
				__e = __c.Set("class", "nechat-file nechat-file_err"); if __e != nil { return }
//line sent.gox:37
				__e = __c.Set("title", text); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line sent.gox:38
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line sent.gox:38
					__e = __c.Any(icons.WarningFill()); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line sent.gox:39
					__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line sent.gox:39
					__e = __c.Any(path.Base(s.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line sent.gox:40
					__e = __c.Set("class", "nechat-file-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line sent.gox:40
					__e = __c.Any(text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("a"); if __e != nil { return }
			{
//line sent.gox:43
				__e = __c.Set("class", "nechat-file nechat-file_done"); if __e != nil { return }
//line sent.gox:43
				__e = __c.Set("title", entry.MIME); if __e != nil { return }
//line sent.gox:43
				__e = __c.Set("href", download(s.FS, s.Name)); if __e != nil { return }
//line sent.gox:43
				__e = __c.Set("download", path.Base(s.Name)); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line sent.gox:44
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line sent.gox:44
					__e = __c.Any(fileIcon(entry.MIME)); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line sent.gox:45
					__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line sent.gox:45
					__e = __c.Any(path.Base(s.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line sent.gox:48
}

func download(fs api.FS, name string) doors.Resource {
	return doors.ResourceHandler(func(w http.ResponseWriter, r *http.Request) {
		e, err := fs.UploadStat(r.Context(), name)
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
		w.Header().Set("Content-Disposition", "attachment; filename=" + strconv.Quote(path.Base(name)))
		io.Copy(w, rc)
	})
}
