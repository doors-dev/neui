// Managed by GoX v0.4.0

//line form_file.gox:1
package form

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/common"
	"neui.dev/neisland/internal/files"
)

func newFileWidget(ctx context.Context, group group, params fileParams) (*fileWidget, error) {
	if params.Max == 0 {
		params.Max = 1
	}
	if params.Max < params.Min {
		return nil, fmt.Errorf("field %q: max %d is below min %d", params.Name, params.Max, params.Min)
	}
	field, err := group.newField(params.Name)
	if err != nil {
		return nil, err
	}
	config := api.FilesConfig{}
	if store := neui.IslandStore(ctx); store != nil {
		config, _ = store.Load(common.IslandFilesConfigKey{}).(api.FilesConfig)
	}
	id := doors.IDRand()
	return &fileWidget{
		id: id,
		folder: path.Join(neui.IslandDir(ctx), id),
		fs: neui.IslandFS(ctx),
		limit: int64(min(common.FilesConfigDefaults(config).DocumentLimit, doors.AppConf(ctx).ServerRequestBodyLimit)),
		field: field,
		params: params,
		count: doors.NewSource(fileCount{}),
	}, nil
}

type filePicked struct {
	Name string `json:"name"`
	Size int64 `json:"size"`
	Type string `json:"type"`
}

type fileStatus int

const (
	fileUploading fileStatus = iota
	fileDone
	fileFailed
)

type fileEntry struct {
	key string
	name string
	path string
	size int64
	status fileStatus
	reason string
	retry bool
	cancel context.CancelFunc
	door *doors.Door
}

type fileCount struct {
	held int
	done int
}

type fileWidget struct {
	id string
	folder string
	fs api.FS
	limit int64
	field groupField
	params fileParams
	entries []*fileEntry
	seq int
	edge *doors.Door
	focus *doors.Door
	count doors.Source[fileCount]
	empty doors.Setter
	list doors.Setter
	label doors.Setter
}

func (f *fileWidget) name() string {
	return f.params.Name
}

func (f *fileWidget) init(ctx context.Context, value any) error {
	paths := f.params.Value
	if value != nil {
		items, ok := value.([]any)
		if !ok {
			return fmt.Errorf("field %q: default value is %T, want an array of paths", f.params.Name, value)
		}
		paths = make([]string, 0, len(items))
		for _, item := range items {
			p, ok := item.(string)
			if !ok {
				return fmt.Errorf("field %q: default item is %T, want a path", f.params.Name, item)
			}
			paths = append(paths, p)
		}
	}
	if len(paths) > f.params.Max {
		return fmt.Errorf("field %q: %d default files, most is %d", f.params.Name, len(paths), f.params.Max)
	}
	for _, p := range paths {
		p = path.Clean(p)
		if f.fs == nil {
			return fmt.Errorf("field %q: default file %q: no workspace", f.params.Name, p)
		}
		f.seq++
		e := &fileEntry{
			key: strconv.Itoa(f.seq),
			name: path.Base(p),
			path: p,
			status: fileDone,
			door: new(doors.Door),
		}
		entry, err := f.fs.Stat(ctx, p)
		switch {
		case err != nil:
			e.status, e.reason = fileFailed, "file not found"
		case entry.Dir:
			e.status, e.reason = fileFailed, "is a folder"
		default:
			e.size = entry.Size
		}
		f.entries = append(f.entries, e)
		e.door.Outer(ctx, f.chip(*e))
	}
	f.setLocked(ctx)
	return nil
}

func (f *fileWidget) destroy(ctx context.Context) {
	for _, e := range f.entries {
		if e.cancel != nil {
			e.cancel()
		}
	}
	f.entries = nil
	if f.fs != nil {
		if err := f.fs.Remove(ctx, f.folder); err != nil {
			doors.Logger(ctx).Error("ui: form file cleanup", "dir", f.folder, "err", err)
		}
	}
	f.field.destroy(ctx)
}

func (f *fileWidget) setLocked(ctx context.Context) {
	var paths []string
	count := fileCount{}
	for _, e := range f.entries {
		switch e.status {
		case fileDone:
			paths = append(paths, e.path)
			count.held++
			count.done++
		case fileUploading:
			count.held++
		}
	}
	var err error
	switch {
	case count.held > count.done:
		err = errUploading
	case f.params.Min > 0 && count.done == 0:
		err = errRequired
	case count.done < f.params.Min:
		err = fmt.Errorf("choose at least %d", f.params.Min)
	}
	if len(paths) == 0 {
		f.field.set(ctx, nil, err)
	} else {
		f.field.set(ctx, paths, err)
	}
	f.count.Update(ctx, count)
}

func (f *fileWidget) tooLarge() string {
	if f.limit < 1 << 20 {
		return fmt.Sprintf("larger than %d KB", f.limit >> 10)
	}
	return fmt.Sprintf("larger than %d MB", f.limit >> 20)
}

func (f *fileWidget) pick(ctx context.Context, picked []filePicked) []any {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	held := 0
	for _, e := range f.entries {
		if e.status != fileFailed {
			held++
		}
	}
	keys := make([]any, len(picked))
	for i, file := range picked {
		name := path.Base(file.Name)
		f.seq++
		e := &fileEntry{
			key: strconv.Itoa(f.seq),
			name: name,
			size: file.Size,
			status: fileFailed,
			door: new(doors.Door),
		}
		if file.Name == "" {
			e.name = "unnamed"
		}
		kind, _, _ := strings.Cut(mime.TypeByExtension(strings.ToLower(path.Ext(name))), ";")
		if kind == "" {
			kind = file.Type
		}
		kind = strings.ToLower(strings.TrimSpace(kind))
		accepted := f.params.Accept == ""
		var tokens []string
		for _, token := range strings.Split(f.params.Accept, ",") {
			token = strings.ToLower(strings.TrimSpace(token))
			if token == "" {
				continue
			}
			tokens = append(tokens, token)
			switch {
			case strings.HasPrefix(token, "."):
				accepted = accepted || strings.HasSuffix(strings.ToLower(name), token)
			case strings.HasSuffix(token, "/*"):
				accepted = accepted || strings.HasPrefix(kind, strings.TrimSuffix(token, "*"))
			default:
				accepted = accepted || kind == token
			}
		}
		switch {
		case file.Name == "" || name == "." || name == ".." || name == "/" || strings.ContainsAny(name, "/\\"):
			e.reason = "bad file name"
		case !accepted:
			e.reason = "not " + strings.Join(tokens, " or ")
		case file.Size > f.limit:
			e.reason = f.tooLarge()
		case held >= f.params.Max && f.params.Max == 1:
			e.reason = "at most 1 file"
		case held >= f.params.Max:
			e.reason = fmt.Sprintf("at most %d files", f.params.Max)
		case f.fs == nil:
			e.reason = "upload failed"
		default:
			ext := path.Ext(name)
			stem := strings.TrimSuffix(name, ext)
			stored := name
			for n := 2; ; n++ {
				p := path.Join(f.folder, stored)
				if !slices.ContainsFunc(f.entries, func(e *fileEntry) bool {
					return e.path == p
				}) {
					if _, err := f.fs.Stat(ctx, p); err != nil {
						break
					}
				}
				stored = fmt.Sprintf("%s (%d)%s", stem, n, ext)
			}
			e.name = stored
			e.path = path.Join(f.folder, stored)
			e.status = fileUploading
			held++
			keys[i] = e.key
		}
		f.entries = append(f.entries, e)
		e.door.Outer(ctx, f.chip(*e))
		next := new(doors.Door)
		f.edge.Static(ctx, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitContainer(); if __e != nil { return }
			{
//line form_file.gox:285
				__e = __c.Any(e.door); if __e != nil { return }
//line form_file.gox:286
				__e = __c.Any(next); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:287
		return }))
		f.edge = next
	}
	f.setLocked(ctx)
	return keys
}

func (f *fileWidget) receive(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	key := r.URL.Query().Get("k")
	f.field.mutex().Lock()
	i := slices.IndexFunc(f.entries, func(e *fileEntry) bool {
		return e.key == key
	})
	if i == -1 || f.entries[i].status != fileUploading || f.entries[i].cancel != nil {
		f.field.mutex().Unlock()
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusGone)
		return false
	}
	e := f.entries[i]
	read, cancel := context.WithCancel(r.Context())
	defer cancel()
	e.cancel = cancel
	f.field.mutex().Unlock()
	var err error
	var size int64
	if r.ContentLength > f.limit {
		err = &http.MaxBytesError{Limit: f.limit}
	} else {
		var dst io.WriteCloser
		dst, err = f.fs.Create(e.path)
		if err == nil {
			size, err = io.Copy(dst, fileReader{ctx: read, r: http.MaxBytesReader(w, r.Body, f.limit)})
			if closeErr := dst.Close(); err == nil {
				err = closeErr
			}
			if err != nil {
				if removeErr := f.fs.Remove(context.WithoutCancel(r.Context()), e.path); removeErr != nil {
					doors.Logger(ctx).Error("ui: form file cleanup", "name", e.path, "err", removeErr)
				}
			}
		}
	}
	ctx = doors.InstanceContext(ctx)
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	if !slices.Contains(f.entries, e) {
		if err == nil {
			if removeErr := f.fs.Remove(context.WithoutCancel(r.Context()), e.path); removeErr != nil {
				doors.Logger(ctx).Error("ui: form file cleanup", "name", e.path, "err", removeErr)
			}
		}
		w.Header().Set("Connection", "close")
		w.WriteHeader(http.StatusGone)
		return false
	}
	var maxBytes *http.MaxBytesError
	status := http.StatusOK
	switch {
	case err == nil:
		e.status = fileDone
		e.size = size
	case errors.As(err, &maxBytes):
		e.status, e.reason, status = fileFailed, f.tooLarge(), http.StatusRequestEntityTooLarge
	case errors.Is(err, api.ErrQuota):
		e.status, e.reason, status = fileFailed, "workspace is full", http.StatusInsufficientStorage
	case errors.Is(err, api.ErrInvalidName):
		e.status, e.reason, status = fileFailed, "bad file name", http.StatusBadRequest
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF):
		e.status, e.reason, e.retry, status = fileFailed, "upload interrupted", true, http.StatusBadRequest
	default:
		doors.Logger(ctx).Error("ui: form file upload", "name", e.path, "err", err)
		e.status, e.reason, e.retry, status = fileFailed, "upload failed", true, http.StatusInternalServerError
	}
	e.cancel = nil
	if status != http.StatusOK {
		w.Header().Set("Connection", "close")
	}
	w.WriteHeader(status)
	e.door.Outer(ctx, f.chip(*e))
	f.setLocked(ctx)
	return false
}

type fileReader struct {
	ctx context.Context
	r io.Reader
}

func (r fileReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (f *fileWidget) retry(ctx context.Context, key string) bool {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.IndexFunc(f.entries, func(e *fileEntry) bool {
		return e.key == key
	})
	if i == -1 || f.entries[i].status != fileFailed || !f.entries[i].retry {
		return false
	}
	held := 0
	for _, e := range f.entries {
		if e.status != fileFailed {
			held++
		}
	}
	if held >= f.params.Max {
		return false
	}
	e := f.entries[i]
	e.status, e.reason, e.retry = fileUploading, "", false
	e.door.Outer(ctx, f.chip(*e))
	f.setLocked(ctx)
	return true
}

func (f *fileWidget) failed(ctx context.Context, key string) {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.IndexFunc(f.entries, func(e *fileEntry) bool {
		return e.key == key
	})
	if i == -1 || f.entries[i].status != fileUploading || f.entries[i].cancel != nil {
		return
	}
	e := f.entries[i]
	e.status, e.reason, e.retry = fileFailed, "upload interrupted", true
	e.door.Outer(ctx, f.chip(*e))
	f.setLocked(ctx)
}

func (f *fileWidget) remove(ctx context.Context, key string) {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.IndexFunc(f.entries, func(e *fileEntry) bool {
		return e.key == key
	})
	if i == -1 {
		return
	}
	e := f.entries[i]
	held := 0
	for _, e := range f.entries {
		if e.status != fileFailed {
			held++
		}
	}
	f.entries = slices.Delete(f.entries, i, i + 1)
	if e.cancel != nil {
		e.cancel()
	}
	if e.status == fileDone {
		if err := f.fs.Remove(ctx, e.path); err != nil {
			doors.Logger(ctx).Error("ui: form file remove", "name", e.path, "err", err)
		}
	}
	f.setLocked(ctx)
	target := f.id
	switch {
	case held >= f.params.Max && e.status != fileFailed:
	case i < len(f.entries):
		target = f.id + "-x" + f.entries[i].key
	case i > 0:
		target = f.id + "-x" + f.entries[i - 1].key
	}
	f.focus.Inner(ctx, gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line form_file.gox:461
			__e = __c.Set("data:id", target); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const id = $data(\"id\")\n\t\tconst focus = (tries) => {\n\t\t\tconst el = document.getElementById(id)\n\t\t\tif (el) {\n\t\t\t\tel.focus()\n\t\t\t} else if (tries > 0) {\n\t\t\t\trequestAnimationFrame(() => focus(tries - 1))\n\t\t\t}\n\t\t}\n\t\tfocus(30)"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_file.gox:472
	return }))
	e.door.Static(ctx, nil)
}

//line form_file.gox:476
func (f *fileWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_file.gox:478
		label, labelledby := any(nil), any(f.id + "-l")
	if f.field.item != nil {
		label, _ = f.field.item.ReadAndSub(ctx, func(ctx context.Context, name string) bool {
			doors.Call(ctx, f.label.Set("aria-label", name))
			return false
		})
		labelledby = nil
	}
	f.field.mutex().Lock()
	entries := slices.Clone(f.entries)
	f.edge = new(doors.Door)
	f.focus = new(doors.Door)
	edge, focus := f.edge, f.focus
	f.field.mutex().Unlock()
	full := doors.DeriveBeam(f.count, func(c fileCount) bool {
		return c.held >= f.params.Max
	})
	empty := doors.DeriveBeam(f.count, func(c fileCount) bool {
		return f.params.Min > 0 && c.done == 0
	})
	describedby := func(full bool) any {
		if full && f.params.Max > 1 {
			return f.id + "-h"
		}
		return nil
	}
	isFull, _ := full.ReadAndSub(ctx, func(ctx context.Context, full bool) bool {
		doors.Call(ctx, f.list.Set("aria-describedby", describedby(full)))
		return false
	})
	empty.Sub(ctx, func(ctx context.Context, empty bool) bool {
		doors.Call(ctx, f.empty.Set("data-required-empty", empty))
		return false
	})
	f.field.error.Sub(ctx, func(ctx context.Context, err error) bool {
		doors.Call(ctx, f.empty.Set("aria-invalid", aria(shownErr(err))))
		return false
	})
	body := gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_file.gox:516
			__e = __c.Set("class", "neisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:517
			if label == nil {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_file.gox:518
					__e = __c.Set("class", "neisland-form-label"); if __e != nil { return }
//line form_file.gox:518
					__e = __c.Set("id", f.id + "-l"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:519
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_file.gox:520
					__e = __c.Any(empty.Bind(func(empty bool) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_file.gox:521
						if empty {
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_file.gox:522
								__e = __c.Set("class", "neisland-form-required"); if __e != nil { return }
//line form_file.gox:522
								__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("*"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					return })
//line form_file.gox:524
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line form_file.gox:528
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_file.gox:529
				__e = __c.Modify(&f.list); if __e != nil { return }
//line form_file.gox:530
				__e = __c.Set("class", "neisland-form-search-chosen"); if __e != nil { return }
//line form_file.gox:531
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_file.gox:532
				__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_file.gox:533
				__e = __c.Set("aria-describedby", describedby(isFull)); if __e != nil { return }
//line form_file.gox:534
				__e = __c.Set("aria-live", "polite"); if __e != nil { return }
//line form_file.gox:535
				__e = __c.Set("aria-relevant", "additions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:536
				for _, e := range entries {
//line form_file.gox:537
					__e = __c.Any(e.door); if __e != nil { return }
				}
//line form_file.gox:539
				__e = __c.Any(edge); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:541
			__e = __c.Any(full.Bind(f.zone)); if __e != nil { return }
//line form_file.gox:542
			__e = __c.Any(f.field.error.RouteBeam(
			doors.RouteBeamMatch(shownErr, doors.RouteBind(func(err error) gox.Elem {
				return gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("p"); if __e != nil { return }
					{
//line form_file.gox:544
						__e = __c.Set("class", "neisland-form-error"); if __e != nil { return }
//line form_file.gox:544
						__e = __c.Set("id", f.id + "-e"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:544
						__e = __c.Any(err.Error()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:544
				return })
			})),
		)); if __e != nil { return }
//line form_file.gox:547
			__e = __c.Any(errorData(f.params.Name, f.field.error)); if __e != nil { return }
//line form_file.gox:548
			__e = __c.Any(focus); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
//line form_file.gox:550
				__e = __c.Modify(doors.AHook[[]filePicked]{
				Name: "pick",
				Scope: f.field.scope(),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestHook[[]filePicked]) (any, bool) {
					return f.pick(ctx, r.Data()), false
				},
			}); if __e != nil { return }
//line form_file.gox:558
				__e = __c.Modify(doors.AHook[string]{
				Name: "retry",
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestHook[string]) (any, bool) {
					return f.retry(ctx, r.Data()), false
				},
			}); if __e != nil { return }
//line form_file.gox:565
				__e = __c.Modify(doors.AHook[string]{
				Name: "failed",
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestHook[string]) (any, bool) {
					f.failed(ctx, r.Data())
					return nil, false
				},
			}); if __e != nil { return }
//line form_file.gox:573
				__e = __c.Set("data:upload", func() any {
				url, _ := doors.ResourceHook(f.receive).URL(ctx)
				return url
			}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const root = document.currentScript.parentElement\n\t\t\tconst list = root.querySelector(\":scope > .neisland-form-search-chosen\")\n\t\t\tconst upload = $data(\"upload\")\n\t\t\tconst files = new Map()\n\t\t\tconst size = (n) =>\n\t\t\t\tn < 1000\n\t\t\t\t\t? n + \" B\"\n\t\t\t\t\t: n < 1e6\n\t\t\t\t\t\t? Math.round(n / 1000) + \" KB\"\n\t\t\t\t\t\t: (n / 1e6).toFixed(1) + \" MB\"\n\t\t\tconst report = (key) => {\n\t\t\t\t$hook(\"failed\", key).catch(() => {\n\t\t\t\t\tsetTimeout(() => files.has(key) && report(key), 1000)\n\t\t\t\t})\n\t\t\t}\n\t\t\tconst send = (key, file) => {\n\t\t\t\tconst xhr = new XMLHttpRequest()\n\t\t\t\tfiles.set(key, file)\n\t\t\t\tlet last = 0\n\t\t\t\txhr.upload.onprogress = (event) => {\n\t\t\t\t\tconst now = Date.now()\n\t\t\t\t\tif (!event.lengthComputable || now - last < 500) {\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tlast = now\n\t\t\t\t\tconst p = Math.min(99, Math.floor((event.loaded / event.total) * 100))\n\t\t\t\t\tconst chip = list.querySelector(`li[data-key=\"${key}\"][data-uploading]`)\n\t\t\t\t\tconst bar = chip?.querySelector(\".neisland-form-file-percent\")\n\t\t\t\t\tif (!bar) {\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tchip.style.setProperty(\"--neisland-file-p\", p + \"%\")\n\t\t\t\t\tbar.textContent = p + \"%\"\n\t\t\t\t\tbar.setAttribute(\"aria-valuenow\", p)\n\t\t\t\t\tbar.setAttribute(\"aria-valuetext\", `${p}%, ${size(event.loaded)} of ${size(event.total)}`)\n\t\t\t\t}\n\t\t\t\txhr.onload = () => {\n\t\t\t\t\tif (xhr.status < 300) {\n\t\t\t\t\t\tfiles.delete(key)\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\txhr.onerror = () => report(key)\n\t\t\t\txhr.open(\"PUT\", `${upload}?k=${encodeURIComponent(key)}`)\n\t\t\t\txhr.send(file)\n\t\t\t}\n\t\t\tconst pick = async (picked) => {\n\t\t\t\tif (picked.length === 0) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tlet keys\n\t\t\t\ttry {\n\t\t\t\t\tkeys = await $hook(\n\t\t\t\t\t\t\"pick\",\n\t\t\t\t\t\tpicked.map((file) => ({ name: file.name, size: file.size, type: file.type })),\n\t\t\t\t\t)\n\t\t\t\t} catch {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tkeys.forEach((key, i) => key && send(key, picked[i]))\n\t\t\t}\n\t\t\tconst reveal = () => {\n\t\t\t\tfor (const button of list.querySelectorAll(\"[data-file-retry][hidden]\")) {\n\t\t\t\t\tif (files.has(button.dataset.fileRetry)) {\n\t\t\t\t\t\tbutton.hidden = false\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t\tconst observer = new MutationObserver(reveal)\n\t\t\tobserver.observe(list, { childList: true, subtree: true })\n\t\t\t$sys.clean(() => observer.disconnect())\n\t\t\troot.addEventListener(\"change\", (event) => {\n\t\t\t\tif (!event.target.matches(\".neisland-form-file-input\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tconst picked = Array.from(event.target.files)\n\t\t\t\tevent.target.value = \"\"\n\t\t\t\tpick(picked)\n\t\t\t})\n\t\t\troot.addEventListener(\"click\", async (event) => {\n\t\t\t\tconst retry = event.target.closest(\"[data-file-retry]\")\n\t\t\t\tif (retry) {\n\t\t\t\t\tconst key = retry.dataset.fileRetry\n\t\t\t\t\tconst file = files.get(key)\n\t\t\t\t\tif (!file) {\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tretry.disabled = true\n\t\t\t\t\tlet ok = false\n\t\t\t\t\ttry {\n\t\t\t\t\t\tok = await $hook(\"retry\", key)\n\t\t\t\t\t} catch {}\n\t\t\t\t\tif (ok) {\n\t\t\t\t\t\tsend(key, file)\n\t\t\t\t\t} else {\n\t\t\t\t\t\tretry.disabled = false\n\t\t\t\t\t}\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tconst remove = event.target.closest(\"li[data-key] > .neisland-form-remove\")\n\t\t\t\tif (remove) {\n\t\t\t\t\tfiles.delete(remove.parentElement.dataset.key)\n\t\t\t\t}\n\t\t\t})\n\t\t\tlet text = null\n\t\t\tconst leave = (zone) => {\n\t\t\t\tzone.removeAttribute(\"data-dragover\")\n\t\t\t\tconst summary = zone.querySelector(\".neisland-form-file-summary\")\n\t\t\t\tif (summary && text !== null) {\n\t\t\t\t\tsummary.textContent = text\n\t\t\t\t}\n\t\t\t\ttext = null\n\t\t\t}\n\t\t\tconst over = (event) => {\n\t\t\t\tif (!event.dataTransfer?.types.includes(\"Files\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tevent.preventDefault()\n\t\t\t\tconst zone = event.target.closest(\".neisland-form-file-zone\")\n\t\t\t\tif (!zone) {\n\t\t\t\t\tevent.dataTransfer.dropEffect = \"none\"\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tevent.dataTransfer.dropEffect = \"copy\"\n\t\t\t\tif (zone.hasAttribute(\"data-dragover\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tzone.setAttribute(\"data-dragover\", \"\")\n\t\t\t\tconst summary = zone.querySelector(\".neisland-form-file-summary\")\n\t\t\t\tconst n = event.dataTransfer.items.length\n\t\t\t\ttext = summary.textContent\n\t\t\t\tsummary.textContent = n === 1 ? \"Drop to upload 1 file\" : `Drop to upload ${n} files`\n\t\t\t}\n\t\t\troot.addEventListener(\"dragenter\", over)\n\t\t\troot.addEventListener(\"dragover\", over)\n\t\t\troot.addEventListener(\"dragleave\", (event) => {\n\t\t\t\tconst zone = event.target.closest(\".neisland-form-file-zone\")\n\t\t\t\tif (zone && !zone.contains(event.relatedTarget)) {\n\t\t\t\t\tleave(zone)\n\t\t\t\t}\n\t\t\t})\n\t\t\troot.addEventListener(\"drop\", (event) => {\n\t\t\t\tif (!event.dataTransfer?.types.includes(\"Files\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tevent.preventDefault()\n\t\t\t\tconst zone = event.target.closest(\".neisland-form-file-zone\")\n\t\t\t\tif (!zone) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tleave(zone)\n\t\t\t\tpick(Array.from(event.dataTransfer.files))\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_file.gox:730
	return })

//line form_file.gox:732
		if f.field.item != nil {
//line form_file.gox:733
			__e = __c.Any(body); if __e != nil { return }
		} else  {
//line form_file.gox:735
			__e = (neui.Space{Name: f.params.Name, Desc: f.params.Label}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line form_file.gox:735
				__e = __c.Any(body); if __e != nil { return }
			return })); if __e != nil { return }
		}
	return })
//line form_file.gox:737
}

//line form_file.gox:739
func (f *fileWidget) zone(full bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_file.gox:740
		if !full {
//line form_file.gox:742
			count, _ := f.count.Read(ctx)
		label, labelledby := any(nil), any(f.id + "-l")
		if f.field.item != nil {
			label, _ = f.field.item.Read(ctx)
			labelledby = nil
		}

			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_file.gox:749
				__e = __c.Set("class", "neisland-form-file-zone"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:750
				__e = (neui.Input{Desc: "file chooser"}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line form_file.gox:751
						__e = __c.Modify(&f.empty); if __e != nil { return }
//line form_file.gox:752
						__e = __c.Modify(&f.label); if __e != nil { return }
//line form_file.gox:753
						__e = __c.Set("class", "neisland-form-file-input"); if __e != nil { return }
//line form_file.gox:754
						__e = __c.Set("id", f.id); if __e != nil { return }
//line form_file.gox:755
						__e = __c.Set("type", "file"); if __e != nil { return }
//line form_file.gox:756
						__e = __c.Set("accept", f.params.Accept); if __e != nil { return }
//line form_file.gox:757
						__e = __c.Set("multiple", f.params.Max > 1); if __e != nil { return }
//line form_file.gox:758
						__e = __c.Set("aria-required", aria(f.params.Min > 0)); if __e != nil { return }
//line form_file.gox:759
						__e = __c.Set("aria-invalid", aria(shownErr(f.field.error.Get()))); if __e != nil { return }
//line form_file.gox:760
						__e = __c.Set("data-required-empty", f.params.Min > 0 && count.done == 0); if __e != nil { return }
//line form_file.gox:761
						__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_file.gox:762
						__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_file.gox:763
						__e = __c.Set("aria-describedby", f.id + "-z " + f.id + "-e"); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				return })); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_file.gox:764
					__e = __c.Set("class", "neisland-form-file-summary"); if __e != nil { return }
//line form_file.gox:764
					__e = __c.Set("id", f.id + "-z"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:765
					if f.params.Max > 1 {
						__e = __c.Text("Drop files or click to choose"); if __e != nil { return }
					} else  {
						__e = __c.Text("Drop a file or click to choose"); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:772
		} else if f.params.Max > 1 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_file.gox:773
				__e = __c.Set("class", "neisland-form-search-max"); if __e != nil { return }
//line form_file.gox:773
				__e = __c.Set("id", f.id + "-h"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_file.gox:773
				__e = __c.Any(f.params.Max); if __e != nil { return }
				__e = __c.Text(" chosen."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_file.gox:775
}

//line form_file.gox:777
func (f *fileWidget) chip(e fileEntry) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("li"); if __e != nil { return }
		{
//line form_file.gox:779
			__e = __c.Set("class", "neisland-form-file-chip"); if __e != nil { return }
//line form_file.gox:780
			__e = __c.Set("data-key", e.key); if __e != nil { return }
//line form_file.gox:781
			__e = __c.Set("data-uploading", e.status == fileUploading); if __e != nil { return }
//line form_file.gox:782
			__e = __c.Set("data-failed", e.status == fileFailed); if __e != nil { return }
//line form_file.gox:783
			__e = __c.Set("style", func() any {
			if e.status == fileUploading {
				return "--neisland-file-p:0%"
			}
			return nil
		}()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_file.gox:789
				__e = __c.Set("class", "neisland-form-file-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:790
				if e.status == fileDone {
					__e = __c.Init("a"); if __e != nil { return }
					{
//line form_file.gox:792
						__e = __c.Set("class", "neisland-form-file-link"); if __e != nil { return }
//line form_file.gox:793
						__e = __c.Set("href", files.MarkdownServe(f.fs, e.path, true)); if __e != nil { return }
//line form_file.gox:794
						__e = __c.Set("download", path.Base(e.path)); if __e != nil { return }
//line form_file.gox:795
						__e = __c.Set("title", e.name); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:796
						__e = __c.Any(e.name); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:798
					__e = __c.Any(" "); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:799
						__e = __c.Set("class", "neisland-form-file-size"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:799
						__e = __c.Any(files.Size(e.size)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:800
				} else if e.status == fileUploading {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:801
						__e = __c.Set("class", "neisland-form-file-text"); if __e != nil { return }
//line form_file.gox:801
						__e = __c.Set("title", e.name); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:801
						__e = __c.Any(e.name); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:802
					__e = __c.Any(" "); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:804
						__e = __c.Set("class", "neisland-form-file-percent"); if __e != nil { return }
//line form_file.gox:805
						__e = __c.Set("role", "progressbar"); if __e != nil { return }
//line form_file.gox:806
						__e = __c.Set("aria-label", "Uploading " + e.name); if __e != nil { return }
//line form_file.gox:807
						__e = __c.Set("aria-valuemin", "0"); if __e != nil { return }
//line form_file.gox:808
						__e = __c.Set("aria-valuemax", "100"); if __e != nil { return }
//line form_file.gox:809
						__e = __c.Set("aria-valuenow", "0"); if __e != nil { return }
//line form_file.gox:810
						__e = __c.Set("aria-valuetext", "0%"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("0%"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:814
						__e = __c.Set("class", "neisland-form-file-text"); if __e != nil { return }
//line form_file.gox:814
						__e = __c.Set("title", e.name); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:814
						__e = __c.Any(e.name); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:815
					__e = __c.Any(" "); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:816
						__e = __c.Set("class", "neisland-form-file-reason"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:816
						__e = __c.Any(e.reason); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:819
			if e.status == fileDone {
//line form_file.gox:820
				__e = __c.Any(neui.Data{Text: "uploaded file", Object: map[string]any{"name": e.name, "size": e.size, "path": e.path}}); if __e != nil { return }
//line form_file.gox:821
			} else if e.status == fileFailed {
//line form_file.gox:822
				__e = __c.Any(neui.Data{Text: "file not uploaded", Object: map[string]any{"name": e.name, "error": e.reason}}); if __e != nil { return }
			}
//line form_file.gox:824
			if e.retry {
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_file.gox:825
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_file.gox:825
					__e = __c.Set("class", "neisland-form-file-retry"); if __e != nil { return }
//line form_file.gox:825
					__e = __c.Set("data-file-retry", e.key); if __e != nil { return }
					__e = __c.Set("hidden", true); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Retry"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line form_file.gox:827
			__e = (neui.Input{Desc: "remove " + e.name}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_file.gox:828
					__e = __c.Modify(doors.AClick{
				Scope: f.field.scope(),
				Batch: true,
				On: func(ctx context.Context, r doors.RequestPointer) bool {
					f.remove(ctx, e.key)
					return false
				},
			}); if __e != nil { return }
//line form_file.gox:836
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_file.gox:837
					__e = __c.Set("class", "neisland-form-remove"); if __e != nil { return }
//line form_file.gox:838
					__e = __c.Set("id", f.id + "-x" + e.key); if __e != nil { return }
//line form_file.gox:839
					__e = __c.Set("value", e.key); if __e != nil { return }
//line form_file.gox:840
					__e = __c.Set("aria-label", func() any {
				switch e.status {
				case fileUploading:
					return "Cancel upload of " + e.name
				case fileFailed:
					return "Dismiss " + e.name
				default:
					return "Remove " + e.name
				}
			}()); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("×"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			return })); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_file.gox:853
}
