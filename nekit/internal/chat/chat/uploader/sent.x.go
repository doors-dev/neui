// Managed by GoX v0.4.0

//line sent.gox:1
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
	"neui.dev/nekit/internal/chat/icons"
)

// Sent is an attachment of a sent message: a chip that downloads the file on
// click, or an error chip when the file can no longer be read.
type Sent struct {
	FS api.FS
	Name string
}

//line sent.gox:37
func (s Sent) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line sent.gox:39
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

//line sent.gox:50
		if err != nil {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line sent.gox:51
				__e = __c.Set("class", "nechat-file nechat-file_err"); if __e != nil { return }
//line sent.gox:51
				__e = __c.Set("title", text); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line sent.gox:52
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line sent.gox:52
					__e = __c.Any(icons.WarningFill()); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line sent.gox:53
					__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line sent.gox:53
					__e = __c.Any(path.Base(s.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line sent.gox:54
					__e = __c.Set("class", "nechat-file-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line sent.gox:54
					__e = __c.Any(text); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("a"); if __e != nil { return }
			{
//line sent.gox:57
				__e = __c.Set("class", "nechat-file nechat-file_done"); if __e != nil { return }
//line sent.gox:57
				__e = __c.Set("title", entry.MIME); if __e != nil { return }
//line sent.gox:57
				__e = __c.Set("href", download(s.FS, s.Name)); if __e != nil { return }
//line sent.gox:57
				__e = __c.Set("download", path.Base(s.Name)); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line sent.gox:58
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line sent.gox:58
					__e = __c.Any(fileIcon(entry.MIME)); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line sent.gox:59
					__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line sent.gox:59
					__e = __c.Any(path.Base(s.Name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line sent.gox:62
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
