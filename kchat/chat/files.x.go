// Managed by GoX v0.3.3

//line files.gox:1
package chat

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"path"
	"slices"
	"strconv"
	"strings"
	"time"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev/api"
	"neui.dev/kchat/icons"
)

//line files.gox:20
func (c *chat) files() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:21
		__e = __c.Any(c.pops.BindControls("files",
		func(show func(ctx context.Context)) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line files.gox:23
				__e = __c.Any(filesButton(show, false)); if __e != nil { return }
			return })
//line files.gox:24
		},
		func(hide func(ctx context.Context)) gox.Elem {
			return gox.Elem(func(__c gox.Cursor) (__e error) {
				ctx := __c.Context(); _ = ctx
//line files.gox:26
				__e = __c.Any(filesButton(hide, true)); if __e != nil { return }
			return })
//line files.gox:27
		},
	)); if __e != nil { return }
//line files.gox:29
		__e = __c.Any(c.pops.BindPop("files", func(hide func(context.Context)) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line files.gox:30
			__e = __c.Any(&fileList{fs: c.thread.FS(), loc: c.inst.Location()}); if __e != nil { return }
		return })
//line files.gox:31
	})); if __e != nil { return }
	return })
//line files.gox:32
}

//line files.gox:34
func filesButton(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:36
		class := "kchat-iconbtn"
	if open {
		class = "kchat-iconbtn kchat-iconbtn_open"
	}

//line files.gox:41
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			toggle(ctx)
			return false
		},
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line files.gox:47
				__e = __c.Set("class", class); if __e != nil { return }
//line files.gox:47
				__e = __c.Set("title", "Files"); if __e != nil { return }
//line files.gox:47
				__e = __c.Set("aria-label", "Files"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line files.gox:48
				__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line files.gox:48
					__e = __c.Any(icons.Files()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line files.gox:50
}

type fileList struct {
	fs api.FS
	loc *time.Location
	body *doors.Door
}

//line files.gox:58
func (l *fileList) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:60
		l.body = new(doors.Door)
	l.body.Inner(ctx, l.content())

		__e = __c.Init("div"); if __e != nil { return }
		{
//line files.gox:63
			__e = __c.Set("class", "kchat-rowmenu kchat-popbody kchat-files"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line files.gox:64
			__e = __c.Any(l.body); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line files.gox:66
}

//line files.gox:68
func (l *fileList) content() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:72
		entries := []api.FSEntry{}
	for _, dir := range []string{api.UserUploadsDir, api.AgentUploadsDir} {
		found, err := l.fs.ReadDir(ctx, dir)
		if err != nil {
			doors.Logger(ctx).Error("kchat: files", "dir", dir, "err", err)
			continue
		}
		entries = append(entries, found...)
	}
	slices.SortFunc(entries, func(a api.FSEntry, b api.FSEntry) int {
		return b.ModTime.Compare(a.ModTime)
	})

//line files.gox:85
		if len(entries) == 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line files.gox:86
				__e = __c.Set("class", "kchat-side-empty"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("No files yet"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line files.gox:88
					__e = __c.Set("class", "kchat-side-empty-hint"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Attachments and anything the agent saves show up here"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line files.gox:91
				__e = __c.Set("class", "kchat-pop-group"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line files.gox:92
				for _, e := range entries {
//line files.gox:93
					__e = __c.Any(l.row(e)); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line files.gox:97
}

//line files.gox:99
func (l *fileList) row(e api.FSEntry) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:101
		name := path.Base(e.Name)
	del := doors.NewSource(false)

//line files.gox:104
		__e = __c.Any(del.Bind(func(asking bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line files.gox:105
			if asking {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line files.gox:106
					__e = __c.Set("class", "kchat-form"); if __e != nil { return }
//line files.gox:106
					__e = __c.Set("role", "group"); if __e != nil { return }
//line files.gox:106
					__e = __c.Set("aria-label", "Delete file"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line files.gox:107
						__e = __c.Set("class", "kchat-form-text"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Delete "); if __e != nil { return }
						__e = __c.Init("b"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:107
							__e = __c.Any(name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Text("?"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line files.gox:108
						__e = __c.Set("class", "kchat-form-actions"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line files.gox:109
						__e = (doors.AClick{
						Scope: new(doors.ScopeBlocking),
						On: func(ctx context.Context, rp doors.RequestPointer) bool {
							del.Update(ctx, false)
							return false
						},
					}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line files.gox:115
								__e = __c.Set("type", "button"); if __e != nil { return }
//line files.gox:115
								__e = __c.Set("class", "kchat-btn kchat-btn_2nd"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Cancel"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
//line files.gox:116
						__e = (doors.AClick{
						Scope: new(doors.ScopeBlocking),
						On: func(ctx context.Context, rp doors.RequestPointer) bool {
							if err := l.fs.DeleteUpload(ctx, e.Name); err != nil {
								doors.Logger(ctx).Error("kchat: file delete", "name", e.Name, "err", err)
							}
							l.body.Reload(ctx)
							return false
						},
					}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line files.gox:125
								__e = __c.Set("type", "button"); if __e != nil { return }
//line files.gox:125
								__e = __c.Set("class", "kchat-btn kchat-btn_del"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Delete"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line files.gox:129
					__e = __c.Set("class", "kchat-entry"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line files.gox:130
					__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line files.gox:130
						if strings.HasPrefix(e.Name, api.UserUploadsDir + "/") {
//line files.gox:131
							__e = __c.Any(icons.UploadSimple()); if __e != nil { return }
						} else  {
//line files.gox:133
							__e = __c.Any(icons.DownloadSimple()); if __e != nil { return }
						}
					return })); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line files.gox:135
						__e = __c.Set("class", "kchat-entry-body"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("a"); if __e != nil { return }
						{
//line files.gox:136
							__e = __c.Set("class", "kchat-entry-name"); if __e != nil { return }
//line files.gox:136
							__e = __c.Set("href", download(l.fs, e.Name)); if __e != nil { return }
//line files.gox:136
							__e = __c.Set("download", name); if __e != nil { return }
//line files.gox:136
							__e = __c.Set("title", name); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:136
							__e = __c.Any(name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line files.gox:137
							__e = __c.Set("class", "kchat-entry-detail"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:137
							__e = __c.Any(size(e.Size) + " · " + when(e.ModTime, l.loc)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line files.gox:139
					__e = (doors.AClick{
					Scope: new(doors.ScopeBlocking),
					On: func(ctx context.Context, rp doors.RequestPointer) bool {
						del.Update(ctx, true)
						return false
					},
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line files.gox:145
							__e = __c.Set("class", "kchat-ctl"); if __e != nil { return }
//line files.gox:145
							__e = __c.Set("aria-label", "Delete " + name); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:146
							__e = (doors.Class("kchat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line files.gox:146
								__e = __c.Any(icons.Trash()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		return })
//line files.gox:150
	})); if __e != nil { return }
	return })
//line files.gox:151
}

func size(n int64) string {
	switch {
	case n >= 1 << 20:
		return fmt.Sprintf("%.1f MB", float64(n) / (1 << 20))
	case n >= 1 << 10:
		return fmt.Sprintf("%d KB", n >> 10)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

func when(t time.Time, loc *time.Location) string {
	u := t.In(loc)
	now := time.Now().In(loc)
	y, m, d := now.Date()
	if u.After(time.Date(y, m, d, 0, 0, 0, 0, loc)) {
		return u.Format("15:04")
	}
	if u.Year() == now.Year() {
		return u.Format("Jan 2")
	}
	return u.Format("Jan 2, 2006")
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
