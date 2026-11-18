// Managed by GoX v0.4.0

//line uploader.gox:1
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
	"context"
	"mime/multipart"
	"net/http"
	"slices"
	"strconv"
	"sync"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/params"
)

type Uploader = *uploader

type Controller interface {
	UpdateAttachments(ctx context.Context, seq int, files []string)
	UploadStarted(ctx context.Context, seq int)
	UploadFinished(ctx context.Context, seq int)
}

func New(uploads []string, fs api.FS, controller Controller, seq int, params params.Params) Uploader {
	return &uploader{
		uploads: slices.Clone(uploads),
		fs: fs,
		controller: controller,
		seq: seq,
		appender: new(doors.Door),
		params: params,
	}
}

type uploader struct {
	mu sync.Mutex
	uploads []string
	inflight int
	fs api.FS
	controller Controller
	seq int
	appender *doors.Door
	params params.Params
}

func (u *uploader) reserve() bool {
	u.mu.Lock()
	defer u.mu.Unlock()
	if len(u.uploads) + u.inflight >= u.params.MaxAttachments {
		return false
	}
	u.inflight++
	return true
}

func (u *uploader) release() {
	u.mu.Lock()
	u.inflight--
	u.mu.Unlock()
}

func (u *uploader) add(ctx context.Context, filename string) {
	u.mu.Lock()
	u.inflight--
	u.uploads = append(u.uploads, filename)
	uploads := slices.Clone(u.uploads)
	u.mu.Unlock()
	u.controller.UpdateAttachments(ctx, u.seq, uploads)
}

func (u *uploader) remove(ctx context.Context, filename string) {
	u.mu.Lock()
	u.uploads = slices.DeleteFunc(u.uploads, func(name string) bool {
		return name == filename
	})
	uploads := slices.Clone(u.uploads)
	u.mu.Unlock()
	u.controller.UpdateAttachments(ctx, u.seq, uploads)
}

func (u *uploader) append(ctx context.Context, row gox.Comp) {
	u.mu.Lock()
	next, prev := new(doors.Door), u.appender
	u.appender = next
	u.mu.Unlock()
	prev.Static(ctx, gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.InitContainer(); if __e != nil { return }
		{
//line uploader.gox:103
			__e = __c.Any(row); if __e != nil { return }
//line uploader.gox:104
			__e = __c.Any(next); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line uploader.gox:105
	return }))
}

const multipartOverhead = 64 << 10

func (u *uploader) upload(ctx context.Context, r doors.RequestRawHook) bool {
	u.controller.UploadStarted(ctx, u.seq)
	defer u.controller.UploadFinished(ctx, u.seq)
	r.SetRequestBodyLimit(u.params.MaxAttachmentSize + multipartOverhead)
	reader, err := r.Reader()
	if err != nil {
		r.ResponseWriter().WriteHeader(http.StatusBadRequest)
		return false
	}
	part, err := firstFile(reader)
	if err != nil {
		r.ResponseWriter().WriteHeader(http.StatusBadRequest)
		return false
	}
	name := part.FileName()
	if !u.reserve() {
		u.refuse(ctx, r, name, http.StatusUnprocessableEntity, api.ErrTooMany)
		return false
	}
	total, _ := strconv.ParseInt(r.RequestHeader().Get("Content-Length"), 10, 64)
	if total > int64(u.params.MaxAttachmentSize + multipartOverhead) {
		u.release()
		u.refuse(ctx, r, name, http.StatusRequestEntityTooLarge, api.ErrTooLarge)
		return false
	}
	f := newFileProgress(u.fs, u, r, name, total, u.params)
	u.append(ctx, f)
	added, err := f.download(ctx, part)
	if err == nil {
		part.Close()
	}
	if !added {
		u.release()
	}
	return false
}

func firstFile(reader *multipart.Reader) (*multipart.Part, error) {
	for {
		part, err := reader.NextPart()
		if err != nil {
			return nil, err
		}
		if part.FileName() != "" {
			return part, nil
		}
		part.Close()
	}
}

func (u *uploader) refuse(ctx context.Context, r doors.RequestRawHook, name string, status int, err error) {
	r.ResponseWriter().Header().Set("Connection", "close")
	r.ResponseWriter().WriteHeader(status)
	u.append(ctx, newFileErr(ctx, name, err, u.params))
}

//line uploader.gox:166
func (u *uploader) List() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line uploader.gox:168
		u.mu.Lock()
	uploads := slices.Clone(u.uploads)
	u.mu.Unlock()

		__e = __c.Init("div"); if __e != nil { return }
		{
//line uploader.gox:172
			__e = __c.Set("class", "nechat-tray"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line uploader.gox:173
			for _, name := range uploads {
//line uploader.gox:174
				__e = __c.Any(newFileDone(u.fs, u, name, u.params)); if __e != nil { return }
			}
//line uploader.gox:176
			__e = __c.Any(u.appender); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line uploader.gox:178
}

//line uploader.gox:180
func (u *uploader) Form(pending doors.Beam[bool]) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line uploader.gox:181
		__e = __c.Any(pending.Bind(func(pending bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line uploader.gox:182
			if pending {
				__e = __c.Init("button"); if __e != nil { return }
				{
//line uploader.gox:183
					__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line uploader.gox:183
					__e = __c.Set("aria-label", "Attach files"); if __e != nil { return }
					__e = __c.Set("disabled", true); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line uploader.gox:184
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line uploader.gox:184
						__e = __c.Any(icons.Attach()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
				__e = __c.Init("label"); if __e != nil { return }
				{
//line uploader.gox:187
					__e = __c.Set("id", "attach" + u.params.ID); if __e != nil { return }
//line uploader.gox:187
					__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line uploader.gox:188
					__e = (doors.Class("nechat-ic nechat-icon_idle")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line uploader.gox:188
						__e = __c.Any(icons.Attach()); if __e != nil { return }
					return })); if __e != nil { return }
//line uploader.gox:189
					__e = (doors.Class("nechat-ic nechat-icon_busy")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line uploader.gox:189
						__e = __c.Any(icons.Spinner()); if __e != nil { return }
					return })); if __e != nil { return }
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line uploader.gox:190
						__e = __c.Set("type", "file"); if __e != nil { return }
						__e = __c.Set("multiple", true); if __e != nil { return }
//line uploader.gox:190
						__e = __c.Set("aria-label", "Attach files"); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line uploader.gox:192
				__e = (doors.ARawHook{
				Name: "upload",
				On: u.upload,
				Race: doors.RaceParallel,
				RequestTimeout: u.params.AttachmentTimeout,
				Indicator: doors.IndicateAttrQuery("#attach" + u.params.ID, "aria-busy", "true"),
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("script"); if __e != nil { return }
					{
//line uploader.gox:198
						__e = __c.Set("data:id", u.params.ID); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Raw("const input =\n\t\t\t\t\tdocument.currentScript.previousElementSibling.querySelector(\"input\")\n\t\t\t\tconst text = document.getElementById(\"text\" + $data(\"id\"))\n\t\t\t\tconst widget = input.closest(\".nechat-widget\")\n\t\t\t\tinput.addEventListener(\"change\", () => {\n\t\t\t\t\tconst files = [...input.files]\n\t\t\t\t\tinput.value = \"\"\n\t\t\t\t\ttext?.focus()\n\t\t\t\t\tPromise.allSettled(\n\t\t\t\t\t\tfiles.map((file) => {\n\t\t\t\t\t\t\tconst form = new FormData()\n\t\t\t\t\t\t\tform.append(\"file\", file)\n\t\t\t\t\t\t\treturn $fetch(\"upload\", form).catch((err) => {\n\t\t\t\t\t\t\t\tif (err instanceof HookErr && err.network())\n\t\t\t\t\t\t\t\t\twidget.dispatchEvent(\n\t\t\t\t\t\t\t\t\t\tnew CustomEvent(\"nechat_error\", { detail: err.kind }),\n\t\t\t\t\t\t\t\t\t)\n\t\t\t\t\t\t\t})\n\t\t\t\t\t\t}),\n\t\t\t\t\t)\n\t\t\t\t})"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
		return })
//line uploader.gox:222
	})); if __e != nil { return }
	return })
//line uploader.gox:223
}
