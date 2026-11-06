// Managed by GoX v0.3.3

//line file.gox:1
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
	"neui.dev/kchat/icons"
	"neui.dev/kchat/params"
	"neui.dev/kchat/notify"
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

//line file.gox:62
func (f fileDone) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:63
		__e = __c.Any(f.door); if __e != nil { return }
	return })
//line file.gox:64
}

//line file.gox:66
func (f fileDone) main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:68
		entry, err := f.fs.UploadStat(ctx, f.name)

//line file.gox:70
		if err != nil {
//line file.gox:72
			f.list.remove(ctx, f.name)

//line file.gox:74
			__e = __c.Any(fileErr{
			door: f.door,
			name: f.name,
			text: describe(ctx, err, f.params),
		}.main()); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:80
				__e = __c.Set("class", "kchat-file kchat-file_done"); if __e != nil { return }
//line file.gox:80
				__e = __c.Set("title", entry.MIME); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:81
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line file.gox:81
					__e = __c.Any(fileIcon(entry.MIME)); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line file.gox:82
					__e = __c.Set("class", "kchat-file-name"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line file.gox:82
					__e = __c.Any(path.Base(f.name)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line file.gox:83
					__e = __c.Set("class", "kchat-file-trail"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line file.gox:84
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
//line file.gox:92
							__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line file.gox:92
							__e = __c.Set("aria-label", "Remove " + path.Base(f.name)); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line file.gox:93
							__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line file.gox:93
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
//line file.gox:98
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

//line file.gox:219
func (f *fileProgress) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:220
		__e = __c.Any(f.door); if __e != nil { return }
	return })
//line file.gox:221
}
//line file.gox:222
func (f *fileProgress) main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line file.gox:223
			__e = __c.Set("class", "kchat-file"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line file.gox:224
			__e = __c.Any(f.percent.Bind(func(i int) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("i"); if __e != nil { return }
				{
//line file.gox:225
					__e = __c.Set("class", "kchat-file-fill"); if __e != nil { return }
//line file.gox:225
					__e = __c.Set("style", "width:" + strconv.Itoa(i) + "%"); if __e != nil { return }
//line file.gox:225
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })
//line file.gox:226
		})); if __e != nil { return }
//line file.gox:227
			__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line file.gox:227
				__e = __c.Any(icons.File()); if __e != nil { return }
			return })); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:228
				__e = __c.Set("class", "kchat-file-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:228
				__e = __c.Any(path.Base(f.name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line file.gox:229
			__e = __c.Any(f.percent.Bind(func(i int) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("span"); if __e != nil { return }
				{
//line file.gox:230
					__e = __c.Set("class", "kchat-file-meta"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line file.gox:230
					__e = __c.Any(i); if __e != nil { return }
					__e = __c.Text("%"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })
//line file.gox:231
		})); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:232
				__e = __c.Set("class", "kchat-file-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:233
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
//line file.gox:250
						__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line file.gox:250
						__e = __c.Set("aria-label", "Cancel upload"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line file.gox:251
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line file.gox:251
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
//line file.gox:255
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

//line file.gox:293
func (f fileErr) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line file.gox:294
		__e = __c.Any(f.door); if __e != nil { return }
	return })
//line file.gox:295
}

//line file.gox:297
func (f fileErr) main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("span"); if __e != nil { return }
		{
//line file.gox:298
			__e = __c.Set("class", "kchat-file kchat-file_err"); if __e != nil { return }
//line file.gox:298
			__e = __c.Set("title", f.text); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line file.gox:299
			__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line file.gox:299
				__e = __c.Any(icons.WarningFill()); if __e != nil { return }
			return })); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:300
				__e = __c.Set("class", "kchat-file-name"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:300
				__e = __c.Any(path.Base(f.name)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:301
				__e = __c.Set("class", "kchat-file-meta"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:301
				__e = __c.Any(f.text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line file.gox:302
				__e = __c.Set("class", "kchat-file-trail"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line file.gox:303
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
//line file.gox:310
						__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line file.gox:310
						__e = __c.Set("aria-label", "Dismiss"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line file.gox:311
						__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
//line file.gox:311
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
//line file.gox:315
}
