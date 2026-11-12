// Managed by GoX v0.3.4

//line form_file.gox:1
package ui

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
)

func newFormWidgetFile(ctx context.Context, group formGroup, params fileInput) (*formWidgetFile, error) {
	if params.Max != 0 && !params.Multiple {
		return nil, fmt.Errorf("field %q: max needs multiple", params.Name)
	}
	switch {
	case !params.Multiple:
		params.Max = 1
	case params.Max == 0:
		params.Max = 10
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
	return &formWidgetFile{
		id: id,
		folder: path.Join(neui.IslandDir(ctx), id),
		fs: neui.IslandFS(ctx),
		limit: int64(common.FilesConfigDefaults(config).DocumentLimit),
		field: field,
		params: params,
		count: doors.NewSource(formFileCount{}),
	}, nil
}

type formFilePicked struct {
	Name string `json:"name"`
	Size int64 `json:"size"`
	Type string `json:"type"`
}

type formFileStatus int

const (
	formFileUploading formFileStatus = iota
	formFileDone
	formFileFailed
)

type formFileEntry struct {
	key string
	name string
	path string
	size int64
	status formFileStatus
	reason string
	retry bool
	own bool
	cancel context.CancelFunc
	door *doors.Door
}

type formFileCount struct {
	held int
	done int
}

type formWidgetFile struct {
	id string
	folder string
	fs api.FS
	limit int64
	field formGroupField
	params fileInput
	entries []*formFileEntry
	seq int
	edge *doors.Door
	focus *doors.Door
	count doors.Source[formFileCount]
	empty doors.Setter
	list doors.Setter
	label doors.Setter
}

func (f *formWidgetFile) name() string {
	return f.params.Name
}

func (f *formWidgetFile) init(ctx context.Context, value any) error {
	paths := f.params.Value
	switch v := value.(type) {
	case nil:
	case string:
		if f.params.Multiple {
			return fmt.Errorf("field %q: default value is a string, want an array of paths", f.params.Name)
		}
		paths = []string{v}
	case []any:
		if !f.params.Multiple {
			return fmt.Errorf("field %q: default value is an array, want a path", f.params.Name)
		}
		paths = make([]string, 0, len(v))
		for _, item := range v {
			p, ok := item.(string)
			if !ok {
				return fmt.Errorf("field %q: default item is %T, want a path", f.params.Name, item)
			}
			paths = append(paths, p)
		}
	default:
		return fmt.Errorf("field %q: default value is %T, want a path or an array of paths", f.params.Name, value)
	}
	if len(paths) > f.params.Max {
		return fmt.Errorf("field %q: %d default files, most is %d", f.params.Name, len(paths), f.params.Max)
	}
	for _, p := range paths {
		p = path.Clean(p)
		if f.fs == nil {
			return fmt.Errorf("field %q: default file %q: no workspace", f.params.Name, p)
		}
		entry, err := f.fs.Stat(ctx, p)
		if err != nil {
			return fmt.Errorf("field %q: default file %q not found", f.params.Name, p)
		}
		if entry.Dir {
			return fmt.Errorf("field %q: default file %q is a directory", f.params.Name, p)
		}
		f.seq++
		e := &formFileEntry{
			key: strconv.Itoa(f.seq),
			name: path.Base(p),
			path: p,
			size: entry.Size,
			status: formFileDone,
			door: new(doors.Door),
		}
		f.entries = append(f.entries, e)
		e.door.Outer(ctx, f.chip(*e))
	}
	f.setLocked(ctx)
	return nil
}

func (f *formWidgetFile) destroy(ctx context.Context) {
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

func (f *formWidgetFile) setLocked(ctx context.Context) {
	var paths []string
	count := formFileCount{}
	for _, e := range f.entries {
		switch e.status {
		case formFileDone:
			paths = append(paths, e.path)
			count.held++
			count.done++
		case formFileUploading:
			count.held++
		}
	}
	var err error
	switch {
	case count.held > count.done:
		err = errUploading
	case f.params.Required && count.done == 0:
		err = errRequired
	}
	var value any
	switch {
	case len(paths) == 0:
	case f.params.Multiple:
		value = paths
	default:
		value = paths[0]
	}
	f.field.set(ctx, value, err)
	f.count.Update(ctx, count)
}

func (f *formWidgetFile) tooLarge() string {
	if f.limit < 1 << 20 {
		return fmt.Sprintf("larger than %d KB", f.limit >> 10)
	}
	return fmt.Sprintf("larger than %d MB", f.limit >> 20)
}

func (f *formWidgetFile) pick(ctx context.Context, picked []formFilePicked) []any {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	held := 0
	for _, e := range f.entries {
		if e.status != formFileFailed {
			held++
		}
	}
	keys := make([]any, len(picked))
	for i, file := range picked {
		name := path.Base(file.Name)
		f.seq++
		e := &formFileEntry{
			key: strconv.Itoa(f.seq),
			name: name,
			size: file.Size,
			status: formFileFailed,
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
				if !slices.ContainsFunc(f.entries, func(e *formFileEntry) bool {
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
			e.status = formFileUploading
			e.own = true
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
//line form_file.gox:298
				__e = __c.Any(e.door); if __e != nil { return }
//line form_file.gox:299
				__e = __c.Any(next); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:300
		return }))
		f.edge = next
	}
	f.setLocked(ctx)
	return keys
}

func (f *formWidgetFile) receive(ctx context.Context, w http.ResponseWriter, r *http.Request) bool {
	if r.Method != http.MethodPut {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return false
	}
	key := r.URL.Query().Get("k")
	f.field.mutex().Lock()
	i := slices.IndexFunc(f.entries, func(e *formFileEntry) bool {
		return e.key == key
	})
	if i == -1 || f.entries[i].status != formFileUploading || f.entries[i].cancel != nil {
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
			size, err = io.Copy(dst, formFileReader{ctx: read, r: http.MaxBytesReader(w, r.Body, f.limit)})
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
		e.status = formFileDone
		e.size = size
	case errors.As(err, &maxBytes):
		e.status, e.reason, status = formFileFailed, f.tooLarge(), http.StatusRequestEntityTooLarge
	case errors.Is(err, api.ErrQuota):
		e.status, e.reason, status = formFileFailed, "workspace is full", http.StatusInsufficientStorage
	case errors.Is(err, api.ErrInvalidName):
		e.status, e.reason, status = formFileFailed, "bad file name", http.StatusBadRequest
	case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) || errors.Is(err, io.ErrUnexpectedEOF):
		e.status, e.reason, e.retry, status = formFileFailed, "upload interrupted", true, http.StatusBadRequest
	default:
		doors.Logger(ctx).Error("ui: form file upload", "name", e.path, "err", err)
		e.status, e.reason, e.retry, status = formFileFailed, "upload failed", true, http.StatusInternalServerError
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

type formFileReader struct {
	ctx context.Context
	r io.Reader
}

func (r formFileReader) Read(p []byte) (int, error) {
	if err := r.ctx.Err(); err != nil {
		return 0, err
	}
	return r.r.Read(p)
}

func (f *formWidgetFile) retry(ctx context.Context, key string) bool {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.IndexFunc(f.entries, func(e *formFileEntry) bool {
		return e.key == key
	})
	if i == -1 || f.entries[i].status != formFileFailed || !f.entries[i].retry {
		return false
	}
	held := 0
	for _, e := range f.entries {
		if e.status != formFileFailed {
			held++
		}
	}
	if held >= f.params.Max {
		return false
	}
	e := f.entries[i]
	e.status, e.reason, e.retry = formFileUploading, "", false
	e.door.Outer(ctx, f.chip(*e))
	f.setLocked(ctx)
	return true
}

func (f *formWidgetFile) failed(ctx context.Context, key string) {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.IndexFunc(f.entries, func(e *formFileEntry) bool {
		return e.key == key
	})
	if i == -1 || f.entries[i].status != formFileUploading || f.entries[i].cancel != nil {
		return
	}
	e := f.entries[i]
	e.status, e.reason, e.retry = formFileFailed, "upload interrupted", true
	e.door.Outer(ctx, f.chip(*e))
	f.setLocked(ctx)
}

func (f *formWidgetFile) remove(ctx context.Context, key string) {
	f.field.mutex().Lock()
	defer f.field.mutex().Unlock()
	i := slices.IndexFunc(f.entries, func(e *formFileEntry) bool {
		return e.key == key
	})
	if i == -1 {
		return
	}
	e := f.entries[i]
	held := 0
	for _, e := range f.entries {
		if e.status != formFileFailed {
			held++
		}
	}
	f.entries = slices.Delete(f.entries, i, i + 1)
	if e.cancel != nil {
		e.cancel()
	}
	if e.status == formFileDone && e.own {
		if err := f.fs.Remove(ctx, e.path); err != nil {
			doors.Logger(ctx).Error("ui: form file remove", "name", e.path, "err", err)
		}
	}
	f.setLocked(ctx)
	target := f.id
	switch {
	case held >= f.params.Max && e.status != formFileFailed:
	case i < len(f.entries):
		target = f.id + "-x" + f.entries[i].key
	case i > 0:
		target = f.id + "-x" + f.entries[i - 1].key
	}
	f.focus.Inner(ctx, gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("script"); if __e != nil { return }
		{
//line form_file.gox:474
			__e = __c.Set("data:id", target); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Raw("const id = $data(\"id\")\n\t\tconst focus = (tries) => {\n\t\t\tconst el = document.getElementById(id)\n\t\t\tif (el) {\n\t\t\t\tel.focus()\n\t\t\t} else if (tries > 0) {\n\t\t\t\trequestAnimationFrame(() => focus(tries - 1))\n\t\t\t}\n\t\t}\n\t\tfocus(30)"); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
//line form_file.gox:485
	return }))
	e.door.Static(ctx, nil)
}

func formFileSize(size int64) string {
	switch {
	case size < 1000:
		return fmt.Sprintf("%d B", size)
	case size < 1000 * 1000:
		return fmt.Sprintf("%d KB", (size + 500) / 1000)
	default:
		return fmt.Sprintf("%.1f MB", float64(size) / 1e6)
	}
}

//line form_file.gox:500
func (f *formWidgetFile) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_file.gox:502
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
	full := doors.DeriveBeam(f.count, func(c formFileCount) bool {
		return c.held >= f.params.Max
	})
	empty := doors.DeriveBeam(f.count, func(c formFileCount) bool {
		return f.params.Required && c.done == 0
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

		__e = __c.Init("div"); if __e != nil { return }
		{
//line form_file.gox:537
			__e = __c.Set("class", "kisland-form-field"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:538
			if label == nil {
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_file.gox:539
					__e = __c.Set("class", "kisland-form-label"); if __e != nil { return }
//line form_file.gox:539
					__e = __c.Set("id", f.id + "-l"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:540
					__e = __c.Any(f.params.Label); if __e != nil { return }
//line form_file.gox:541
					__e = __c.Any(empty.Bind(func(empty bool) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line form_file.gox:542
						if empty {
							__e = __c.Init("span"); if __e != nil { return }
							{
//line form_file.gox:543
								__e = __c.Set("class", "kisland-form-required"); if __e != nil { return }
//line form_file.gox:543
								__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("*"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					return })
//line form_file.gox:545
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("ul"); if __e != nil { return }
			{
//line form_file.gox:549
				__e = __c.Modify(&f.label); if __e != nil { return }
//line form_file.gox:550
				__e = __c.Modify(&f.list); if __e != nil { return }
//line form_file.gox:551
				__e = __c.Set("class", "kisland-form-search-chosen"); if __e != nil { return }
//line form_file.gox:552
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_file.gox:553
				__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_file.gox:554
				__e = __c.Set("aria-describedby", describedby(isFull)); if __e != nil { return }
//line form_file.gox:555
				__e = __c.Set("aria-live", "polite"); if __e != nil { return }
//line form_file.gox:556
				__e = __c.Set("aria-relevant", "additions"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:557
				for _, e := range entries {
//line form_file.gox:558
					__e = __c.Any(e.door); if __e != nil { return }
				}
//line form_file.gox:560
				__e = __c.Any(edge); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:562
			__e = __c.Any(full.Bind(f.zone)); if __e != nil { return }
//line form_file.gox:563
			__e = __c.Any(focus); if __e != nil { return }
			__e = __c.Init("script"); if __e != nil { return }
			{
//line form_file.gox:565
				__e = __c.Modify(doors.AHook[[]formFilePicked]{
				Name: "pick",
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestHook[[]formFilePicked]) (any, bool) {
					return f.pick(ctx, r.Data()), false
				},
			}); if __e != nil { return }
//line form_file.gox:572
				__e = __c.Modify(doors.AHook[string]{
				Name: "retry",
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestHook[string]) (any, bool) {
					return f.retry(ctx, r.Data()), false
				},
			}); if __e != nil { return }
//line form_file.gox:579
				__e = __c.Modify(doors.AHook[string]{
				Name: "failed",
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestHook[string]) (any, bool) {
					f.failed(ctx, r.Data())
					return nil, false
				},
			}); if __e != nil { return }
//line form_file.gox:587
				__e = __c.Set("data:upload", func() any {
				url, _ := doors.NewHook(ctx, doors.ResourceHook(f.receive))
				return url
			}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Raw("const root = document.currentScript.parentElement\n\t\t\tconst list = root.querySelector(\":scope > .kisland-form-search-chosen\")\n\t\t\tconst upload = $data(\"upload\")\n\t\t\tconst files = new Map()\n\t\t\tconst size = (n) =>\n\t\t\t\tn < 1000\n\t\t\t\t\t? n + \" B\"\n\t\t\t\t\t: n < 1e6\n\t\t\t\t\t\t? Math.round(n / 1000) + \" KB\"\n\t\t\t\t\t\t: (n / 1e6).toFixed(1) + \" MB\"\n\t\t\tconst report = (key) => {\n\t\t\t\t$hook(\"failed\", key).catch(() => {\n\t\t\t\t\tsetTimeout(() => files.has(key) && report(key), 1000)\n\t\t\t\t})\n\t\t\t}\n\t\t\tconst send = (key, file) => {\n\t\t\t\tconst xhr = new XMLHttpRequest()\n\t\t\t\tfiles.set(key, file)\n\t\t\t\tlet last = 0\n\t\t\t\txhr.upload.onprogress = (event) => {\n\t\t\t\t\tconst now = Date.now()\n\t\t\t\t\tif (!event.lengthComputable || now - last < 500) {\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tlast = now\n\t\t\t\t\tconst p = Math.min(99, Math.floor((event.loaded / event.total) * 100))\n\t\t\t\t\tconst chip = list.querySelector(`li[data-key=\"${key}\"][data-uploading]`)\n\t\t\t\t\tconst bar = chip?.querySelector(\".kisland-form-file-percent\")\n\t\t\t\t\tif (!bar) {\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tchip.style.setProperty(\"--kisland-file-p\", p + \"%\")\n\t\t\t\t\tbar.textContent = p + \"%\"\n\t\t\t\t\tbar.setAttribute(\"aria-valuenow\", p)\n\t\t\t\t\tbar.setAttribute(\"aria-valuetext\", `${p}%, ${size(event.loaded)} of ${size(event.total)}`)\n\t\t\t\t}\n\t\t\t\txhr.onload = () => {\n\t\t\t\t\tif (xhr.status < 300) {\n\t\t\t\t\t\tfiles.delete(key)\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\txhr.onerror = () => report(key)\n\t\t\t\txhr.open(\"PUT\", `${upload}?k=${encodeURIComponent(key)}`)\n\t\t\t\txhr.send(file)\n\t\t\t}\n\t\t\tconst pick = async (picked) => {\n\t\t\t\tif (picked.length === 0) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tlet keys\n\t\t\t\ttry {\n\t\t\t\t\tkeys = await $hook(\n\t\t\t\t\t\t\"pick\",\n\t\t\t\t\t\tpicked.map((file) => ({ name: file.name, size: file.size, type: file.type })),\n\t\t\t\t\t)\n\t\t\t\t} catch {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tkeys.forEach((key, i) => key && send(key, picked[i]))\n\t\t\t}\n\t\t\tconst reveal = () => {\n\t\t\t\tfor (const button of list.querySelectorAll(\"[data-file-retry][hidden]\")) {\n\t\t\t\t\tif (files.has(button.dataset.fileRetry)) {\n\t\t\t\t\t\tbutton.hidden = false\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t}\n\t\t\tconst observer = new MutationObserver(reveal)\n\t\t\tobserver.observe(list, { childList: true, subtree: true })\n\t\t\t$sys.clean(() => observer.disconnect())\n\t\t\troot.addEventListener(\"change\", (event) => {\n\t\t\t\tif (!event.target.matches(\".kisland-form-file-input\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tconst picked = Array.from(event.target.files)\n\t\t\t\tevent.target.value = \"\"\n\t\t\t\tpick(picked)\n\t\t\t})\n\t\t\troot.addEventListener(\"click\", async (event) => {\n\t\t\t\tconst retry = event.target.closest(\"[data-file-retry]\")\n\t\t\t\tif (retry) {\n\t\t\t\t\tconst key = retry.dataset.fileRetry\n\t\t\t\t\tconst file = files.get(key)\n\t\t\t\t\tif (!file) {\n\t\t\t\t\t\treturn\n\t\t\t\t\t}\n\t\t\t\t\tretry.disabled = true\n\t\t\t\t\tlet ok = false\n\t\t\t\t\ttry {\n\t\t\t\t\t\tok = await $hook(\"retry\", key)\n\t\t\t\t\t} catch {}\n\t\t\t\t\tif (ok) {\n\t\t\t\t\t\tsend(key, file)\n\t\t\t\t\t} else {\n\t\t\t\t\t\tretry.disabled = false\n\t\t\t\t\t}\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tconst remove = event.target.closest(\"li[data-key] > .kisland-form-remove\")\n\t\t\t\tif (remove) {\n\t\t\t\t\tfiles.delete(remove.parentElement.dataset.key)\n\t\t\t\t}\n\t\t\t})\n\t\t\tlet text = null\n\t\t\tconst leave = (zone) => {\n\t\t\t\tzone.removeAttribute(\"data-dragover\")\n\t\t\t\tconst summary = zone.querySelector(\".kisland-form-file-summary\")\n\t\t\t\tif (summary && text !== null) {\n\t\t\t\t\tsummary.textContent = text\n\t\t\t\t}\n\t\t\t\ttext = null\n\t\t\t}\n\t\t\tconst over = (event) => {\n\t\t\t\tif (!event.dataTransfer?.types.includes(\"Files\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tevent.preventDefault()\n\t\t\t\tconst zone = event.target.closest(\".kisland-form-file-zone\")\n\t\t\t\tif (!zone) {\n\t\t\t\t\tevent.dataTransfer.dropEffect = \"none\"\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tevent.dataTransfer.dropEffect = \"copy\"\n\t\t\t\tif (zone.hasAttribute(\"data-dragover\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tzone.setAttribute(\"data-dragover\", \"\")\n\t\t\t\tconst summary = zone.querySelector(\".kisland-form-file-summary\")\n\t\t\t\tconst n = event.dataTransfer.items.length\n\t\t\t\ttext = summary.textContent\n\t\t\t\tsummary.textContent = n === 1 ? \"Drop to upload 1 file\" : `Drop to upload ${n} files`\n\t\t\t}\n\t\t\troot.addEventListener(\"dragenter\", over)\n\t\t\troot.addEventListener(\"dragover\", over)\n\t\t\troot.addEventListener(\"dragleave\", (event) => {\n\t\t\t\tconst zone = event.target.closest(\".kisland-form-file-zone\")\n\t\t\t\tif (zone && !zone.contains(event.relatedTarget)) {\n\t\t\t\t\tleave(zone)\n\t\t\t\t}\n\t\t\t})\n\t\t\troot.addEventListener(\"drop\", (event) => {\n\t\t\t\tif (!event.dataTransfer?.types.includes(\"Files\")) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tevent.preventDefault()\n\t\t\t\tconst zone = event.target.closest(\".kisland-form-file-zone\")\n\t\t\t\tif (!zone) {\n\t\t\t\t\treturn\n\t\t\t\t}\n\t\t\t\tleave(zone)\n\t\t\t\tpick(Array.from(event.dataTransfer.files))\n\t\t\t})"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_file.gox:745
}

//line form_file.gox:747
func (f *formWidgetFile) zone(full bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line form_file.gox:748
		if !full {
//line form_file.gox:750
			count, _ := f.count.Read(ctx)
		label, labelledby := any(nil), any(f.id + "-l")
		if f.field.item != nil {
			label, _ = f.field.item.Read(ctx)
			labelledby = nil
		}

			__e = __c.Init("label"); if __e != nil { return }
			{
//line form_file.gox:757
				__e = __c.Set("class", "kisland-form-file-zone"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line form_file.gox:759
					__e = __c.Modify(&f.empty); if __e != nil { return }
//line form_file.gox:760
					__e = __c.Modify(&f.label); if __e != nil { return }
//line form_file.gox:761
					__e = __c.Set("class", "kisland-form-file-input"); if __e != nil { return }
//line form_file.gox:762
					__e = __c.Set("id", f.id); if __e != nil { return }
//line form_file.gox:763
					__e = __c.Set("type", "file"); if __e != nil { return }
//line form_file.gox:764
					__e = __c.Set("accept", f.params.Accept); if __e != nil { return }
//line form_file.gox:765
					__e = __c.Set("multiple", f.params.Multiple); if __e != nil { return }
//line form_file.gox:766
					__e = __c.Set("aria-required", formAria(f.params.Required)); if __e != nil { return }
//line form_file.gox:767
					__e = __c.Set("data-required-empty", f.params.Required && count.done == 0); if __e != nil { return }
//line form_file.gox:768
					__e = __c.Set("aria-label", label); if __e != nil { return }
//line form_file.gox:769
					__e = __c.Set("aria-labelledby", labelledby); if __e != nil { return }
//line form_file.gox:770
					__e = __c.Set("aria-describedby", f.id + "-z"); if __e != nil { return }
				}
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line form_file.gox:771
					__e = __c.Set("class", "kisland-form-file-summary"); if __e != nil { return }
//line form_file.gox:771
					__e = __c.Set("id", f.id + "-z"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:772
					if f.params.Multiple {
						__e = __c.Text("Drop files or click to choose"); if __e != nil { return }
					} else  {
						__e = __c.Text("Drop a file or click to choose"); if __e != nil { return }
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:779
		} else if f.params.Max > 1 {
			__e = __c.Init("p"); if __e != nil { return }
			{
//line form_file.gox:780
				__e = __c.Set("class", "kisland-form-search-max"); if __e != nil { return }
//line form_file.gox:780
				__e = __c.Set("id", f.id + "-h"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Maximum of "); if __e != nil { return }
//line form_file.gox:780
				__e = __c.Any(f.params.Max); if __e != nil { return }
				__e = __c.Text(" chosen."); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line form_file.gox:782
}

//line form_file.gox:784
func (f *formWidgetFile) chip(e formFileEntry) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("li"); if __e != nil { return }
		{
//line form_file.gox:786
			__e = __c.Set("class", "kisland-form-file-chip"); if __e != nil { return }
//line form_file.gox:787
			__e = __c.Set("data-key", e.key); if __e != nil { return }
//line form_file.gox:788
			__e = __c.Set("data-uploading", e.status == formFileUploading); if __e != nil { return }
//line form_file.gox:789
			__e = __c.Set("data-failed", e.status == formFileFailed); if __e != nil { return }
//line form_file.gox:790
			__e = __c.Set("style", func() any {
			if e.status == formFileUploading {
				return "--kisland-file-p:0%"
			}
			return nil
		}()); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("span"); if __e != nil { return }
			{
//line form_file.gox:796
				__e = __c.Set("class", "kisland-form-file-body"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:797
				if e.status == formFileDone {
					__e = __c.Init("a"); if __e != nil { return }
					{
//line form_file.gox:799
						__e = __c.Set("class", "kisland-form-file-link"); if __e != nil { return }
//line form_file.gox:800
						__e = __c.Set("href", markdownServe(f.fs, e.path, true)); if __e != nil { return }
//line form_file.gox:801
						__e = __c.Set("download", path.Base(e.path)); if __e != nil { return }
//line form_file.gox:802
						__e = __c.Set("title", e.name); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:803
						__e = __c.Any(e.name); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:805
					__e = __c.Any(" "); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:806
						__e = __c.Set("class", "kisland-form-file-size"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:806
						__e = __c.Any(formFileSize(e.size)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:807
				} else if e.status == formFileUploading {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:808
						__e = __c.Set("class", "kisland-form-file-text"); if __e != nil { return }
//line form_file.gox:808
						__e = __c.Set("title", e.name); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:808
						__e = __c.Any(e.name); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:809
					__e = __c.Any(" "); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:811
						__e = __c.Set("class", "kisland-form-file-percent"); if __e != nil { return }
//line form_file.gox:812
						__e = __c.Set("role", "progressbar"); if __e != nil { return }
//line form_file.gox:813
						__e = __c.Set("aria-label", "Uploading " + e.name); if __e != nil { return }
//line form_file.gox:814
						__e = __c.Set("aria-valuemin", "0"); if __e != nil { return }
//line form_file.gox:815
						__e = __c.Set("aria-valuemax", "100"); if __e != nil { return }
//line form_file.gox:816
						__e = __c.Set("aria-valuenow", "0"); if __e != nil { return }
//line form_file.gox:817
						__e = __c.Set("aria-valuetext", "0%"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("0%"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:821
						__e = __c.Set("class", "kisland-form-file-text"); if __e != nil { return }
//line form_file.gox:821
						__e = __c.Set("title", e.name); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:821
						__e = __c.Any(e.name); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line form_file.gox:822
					__e = __c.Any(" "); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line form_file.gox:823
						__e = __c.Set("class", "kisland-form-file-reason"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line form_file.gox:823
						__e = __c.Any(e.reason); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
//line form_file.gox:826
			if e.retry {
				__e = __c.Init("button"); if __e != nil { return }
				{
//line form_file.gox:827
					__e = __c.Set("type", "button"); if __e != nil { return }
//line form_file.gox:827
					__e = __c.Set("class", "kisland-form-file-retry"); if __e != nil { return }
//line form_file.gox:827
					__e = __c.Set("data-file-retry", e.key); if __e != nil { return }
					__e = __c.Set("hidden", true); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Retry"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Init("button"); if __e != nil { return }
			{
//line form_file.gox:830
				__e = __c.Modify(doors.AClick{
				Scope: f.field.scope(),
				On: func(ctx context.Context, r doors.RequestPointer) bool {
					f.remove(ctx, e.key)
					return false
				},
			}); if __e != nil { return }
//line form_file.gox:837
				__e = __c.Set("type", "button"); if __e != nil { return }
//line form_file.gox:838
				__e = __c.Set("class", "kisland-form-remove"); if __e != nil { return }
//line form_file.gox:839
				__e = __c.Set("id", f.id + "-x" + e.key); if __e != nil { return }
//line form_file.gox:840
				__e = __c.Set("aria-label", func() any {
				switch e.status {
				case formFileUploading:
					return "Cancel upload of " + e.name
				case formFileFailed:
					return "Dismiss " + e.name
				default:
					return "Remove " + e.name
				}
			}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("×"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line form_file.gox:853
}
