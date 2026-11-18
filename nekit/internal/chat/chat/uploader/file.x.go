// Managed by GoX v0.4.0

//line file.gox:1
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
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"path"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/params"
	"neui.dev/nekit/internal/chat/notify"
)

func fileIcon(mime string) gox.Elem {
	switch {
	case strings.HasPrefix(mime, "image/"):
		return icons.FileImage()
	case strings.HasPrefix(mime, "text/"), mime == "application/json", mime == "application/xml":
		return icons.FileText()
	case mime == "application/zip", mime == "application/gzip", mime == "application/x-tar", mime == "application/x-7z-compressed":
		return icons.FileArchive()
	default:
		return icons.File()
	}
}

type fileList interface {
	add(ctx context.Context, filename string)
	remove(ctx context.Context, filename string)
}

func newFileDone(fs api.FS, list fileList, name string, p params.Params) fileDone {
	f := fileDone{
		fs: fs,
		door: new(doors.Door),
		name: name,
		list: list,
		params: p,
	}
	f.door.Outer(context.Background(), f.main())
	return f
}

type fileDone struct {
	fs api.FS
	door *doors.Door
	name string
	list fileList
	params params.Params
}

//line file.gox:76
func (f fileDone) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:77
		__e = __c.Any(f.door); if __e != nil { return }
	return })
//line file.gox:78
}

//line file.gox:80
func (f fileDone) main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:82
		entry, err := f.fs.UploadStat(ctx, f.name)

//line file.gox:84
		if err != nil {
//line file.gox:86
			f.list.remove(ctx, f.name)

//line file.gox:88
			__e = __c.Any(fileErr{
			door: f.door,
			name: f.name,
			text: describe(ctx, err, f.params),
		}.main()); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:94
				__e = __c.Set("class", "nechat-file nechat-file_done"); if __e != nil { return }
//line file.gox:94
				__e = __c.Set("title", entry.MIME); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:95
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line file.gox:95
					__e = __c.Any(fileIcon(entry.MIME)); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line file.gox:96
					__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line file.gox:96
					__e = __c.Any(path.Base(f.name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line file.gox:97
					__e = __c.Set("class", "nechat-file-trail"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line file.gox:98
					__e = (doors.AClick{
					Scope: new(doors.ScopeBlocking),
					On: func(ctx context.Context, rp doors.RequestPointer) bool {
						f.list.remove(ctx, f.name)
						f.door.Static(ctx, nil)
						return true
					},
					OnError: notify.OnError,
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line file.gox:106
							__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line file.gox:106
							__e = __c.Set("aria-label", "Remove " + path.Base(f.name)); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line file.gox:107
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line file.gox:107
								__e = __c.Any(icons.Close()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line file.gox:112
}

func newFileProgress(fs api.FS, list fileList, r doors.RequestRawHook, name string, total int64, p params.Params) *fileProgress {
	ctx, cancel := context.WithCancel(r.Context())
	f := &fileProgress{
		list: list,
		door: new(doors.Door),
		fs: fs,
		name: name,
		percent: doors.NewSource(0),
		ctx: ctx,
		cancel: cancel,
		total: total,
		params: p,
		w: r.ResponseWriter(),
	}
	f.door.Outer(context.Background(), f.main())
	return f
}

type progressStatus int
const (
	downloding progressStatus = iota
	done
	errored
	canceled
)

type fileProgress struct {
	mu sync.Mutex
	entry *api.FSEntry
	status progressStatus
	fs api.FS
	list fileList
	door *doors.Door
	name string
	percent doors.Source[int]
	cancel context.CancelFunc
	ctx context.Context
	total int64
	params params.Params
	w http.ResponseWriter
}

type countingReader struct {
	r io.Reader
	limit int64
	read atomic.Int64
}

func (c *countingReader) Read(b []byte) (int, error) {
	n, err := c.r.Read(b)
	if c.read.Add(int64(n)) > c.limit {
		return n, api.ErrTooLarge
	}
	return n, err
}

const progressTick = time.Second / 2

func (f *fileProgress) track(ctx context.Context, counter *countingReader) {
	if f.total <= 0 {
		return
	}
	ticker := time.NewTicker(progressTick)
	defer ticker.Stop()
	last := 0
	for {
		select {
		case <-f.ctx.Done():
			return
		case <-ticker.C:
			percent := int(min(counter.read.Load() * 100 / f.total, 99))
			if percent != last {
				last = percent
				f.percent.Update(ctx, percent)
			}
		}
	}
}

func (f *fileProgress) download(ctx context.Context, part *multipart.Part) (added bool, err error) {
	defer f.cancel()
	counter := &countingReader{r: part, limit: int64(f.params.MaxAttachmentSize)}
	go f.track(ctx, counter)
	entry, err := f.fs.UserUpload(f.ctx, f.name, counter)
	f.mu.Lock()
	if err != nil {
		defer f.mu.Unlock()
		f.w.Header().Set("Connection", "close")
		if f.status == canceled {
			f.w.WriteHeader(http.StatusRequestTimeout)
			return false, err
		}
		f.status = errored
		status := http.StatusInternalServerError
		var maxBytes *http.MaxBytesError
		if errors.Is(err, api.ErrTooLarge) || errors.As(err, &maxBytes) {
			status = http.StatusRequestEntityTooLarge
		} else if errors.Is(err, api.ErrQuota) || errors.Is(err, api.ErrExists) || errors.Is(err, api.ErrInvalidName) || errors.Is(err, api.ErrNotFound) {
			status = http.StatusUnprocessableEntity
		} else if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF) {
			status = http.StatusRequestTimeout
		}
		f.w.WriteHeader(status)
		f.intoError(ctx, describe(ctx, err, f.params))
		return false, err
	}
	if f.status == canceled {
		f.mu.Unlock()
		f.fs.DeleteUpload(ctx, entry.Name)
		return false, nil
	}
	f.status = done
	f.entry = &entry
	f.mu.Unlock()
	f.list.add(ctx, f.entry.Name)
	f.intoDone(ctx)
	return true, nil
}

//line file.gox:233
func (f *fileProgress) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:234
		__e = __c.Any(f.door); if __e != nil { return }
	return })
//line file.gox:235
}
//line file.gox:236
func (f *fileProgress) main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line file.gox:237
			__e = __c.Set("class", "nechat-file"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line file.gox:238
			__e = __c.Any(f.percent.Bind(func(i int) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("i"); if __e != nil { return }
				{
//line file.gox:239
					__e = __c.Set("class", "nechat-file-fill"); if __e != nil { return }
//line file.gox:239
					__e = __c.Set("style", "width:" + strconv.Itoa(i) + "%"); if __e != nil { return }
//line file.gox:239
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })
//line file.gox:240
		})); if __e != nil { return }
//line file.gox:241
			__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line file.gox:241
				__e = __c.Any(icons.File()); if __e != nil { return }
			return })); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:242
				__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:242
				__e = __c.Any(path.Base(f.name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line file.gox:243
			__e = __c.Any(f.percent.Bind(func(i int) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
//line file.gox:244
					__e = __c.Set("class", "nechat-file-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line file.gox:244
					__e = __c.Any(i); if __e != nil { return }
					__e = __c.Text("%"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })
//line file.gox:245
		})); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:246
				__e = __c.Set("class", "nechat-file-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:247
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					f.mu.Lock()
					remove := f.status == done
					if f.status == downloding {
						f.status = canceled
						f.cancel()
					}
					f.mu.Unlock()
					if remove {
						f.list.remove(ctx, f.entry.Name)
					}
					f.door.Static(ctx, nil)
					return true
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line file.gox:264
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line file.gox:264
						__e = __c.Set("aria-label", "Cancel upload"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line file.gox:265
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line file.gox:265
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line file.gox:269
}

func (f *fileProgress) intoDone(ctx context.Context) {
	ef := &fileDone{
		fs: f.fs,
		door: f.door,
		list: f.list,
		name: f.entry.Name,
		params: f.params,
	}
	f.door.Outer(ctx, ef.main())
}

func (f *fileProgress) intoError(ctx context.Context, text string) {
	ef := &fileErr{
		door: f.door,
		name: f.name,
		text: text,
	}
	f.door.Outer(ctx, ef.main())
}

type fileErr struct {
	door *doors.Door
	name string
	text string
}

func newFileErr(ctx context.Context, name string, err error, p params.Params) fileErr {
	f := fileErr{
		door: new(doors.Door),
		name: name,
		text: describe(ctx, err, p),
	}
	f.door.Outer(context.Background(), f.main())
	return f
}

//line file.gox:307
func (f fileErr) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:308
		__e = __c.Any(f.door); if __e != nil { return }
	return })
//line file.gox:309
}

//line file.gox:311
func (f fileErr) main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line file.gox:312
			__e = __c.Set("class", "nechat-file nechat-file_err"); if __e != nil { return }
//line file.gox:312
			__e = __c.Set("title", f.text); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line file.gox:313
			__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line file.gox:313
				__e = __c.Any(icons.WarningFill()); if __e != nil { return }
			return })); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:314
				__e = __c.Set("class", "nechat-file-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:314
				__e = __c.Any(path.Base(f.name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:315
				__e = __c.Set("class", "nechat-file-meta"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:315
				__e = __c.Any(f.text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:316
				__e = __c.Set("class", "nechat-file-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:317
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					f.door.Static(ctx, nil)
					return true
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line file.gox:324
						__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line file.gox:324
						__e = __c.Set("aria-label", "Dismiss"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line file.gox:325
						__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line file.gox:325
							__e = __c.Any(icons.Close()); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line file.gox:329
}
