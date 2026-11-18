// Managed by GoX v0.4.0

//line files.gox:1
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
	"neui.dev/nekit/internal/chat/icons"
	"neui.dev/nekit/internal/chat/notify"
)

//line files.gox:35
func (c *chat) files() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:36
		__e = __c.Any(doors.DeriveBeam(c.dead, func(k closedKind) bool {
		return k != alive
	}).Bind(func(dead bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line files.gox:39
			if dead {
				__e = __c.Init("button"); if __e != nil { return }
				{
//line files.gox:40
					__e = __c.Set("class", "nechat-iconbtn"); if __e != nil { return }
//line files.gox:40
					__e = __c.Set("title", "Files"); if __e != nil { return }
//line files.gox:40
					__e = __c.Set("aria-label", "Files"); if __e != nil { return }
					__e = __c.Set("disabled", true); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line files.gox:41
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line files.gox:41
						__e = __c.Any(icons.Files()); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			} else  {
//line files.gox:44
				__e = __c.Any(c.pops.BindControls("files",
				func(show func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line files.gox:46
						__e = __c.Any(filesButton(show, false)); if __e != nil { return }
					return })
//line files.gox:47
				},
				func(hide func(ctx context.Context)) gox.Elem {
					return gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line files.gox:49
						__e = __c.Any(filesButton(hide, true)); if __e != nil { return }
					return })
//line files.gox:50
				},
			)); if __e != nil { return }
			}
		return })
//line files.gox:53
	})); if __e != nil { return }
//line files.gox:54
		__e = __c.Any(c.pops.BindPop("files", func(hide func(context.Context)) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line files.gox:55
			__e = __c.Any(&fileList{fs: c.thread.FS(), loc: c.inst.Location()}); if __e != nil { return }
		return })
//line files.gox:56
	})); if __e != nil { return }
	return })
//line files.gox:57
}

//line files.gox:59
func filesButton(toggle func(ctx context.Context), open bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:61
		class := "nechat-iconbtn"
	expanded := "false"
	if open {
		class = "nechat-iconbtn nechat-iconbtn_open"
		expanded = "true"
	}

//line files.gox:68
		__e = (doors.AClick{
		Scope: new(doors.ScopeBlocking),
		On: func(ctx context.Context, rp doors.RequestPointer) bool {
			toggle(ctx)
			return false
		},
		OnError: notify.OnError,
	}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("button"); if __e != nil { return }
			{
//line files.gox:75
				__e = __c.Set("class", class); if __e != nil { return }
//line files.gox:75
				__e = __c.Set("title", "Files"); if __e != nil { return }
//line files.gox:75
				__e = __c.Set("aria-label", "Files"); if __e != nil { return }
//line files.gox:75
				__e = __c.Set("aria-expanded", expanded); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line files.gox:76
				__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
//line files.gox:76
					__e = __c.Any(icons.Files()); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line files.gox:78
}

type fileList struct {
	fs api.FS
	loc *time.Location
	body *doors.Door
}

//line files.gox:86
func (l *fileList) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:88
		l.body = new(doors.Door)
	l.body.Inner(ctx, l.content())

		__e = __c.Init("div"); if __e != nil { return }
		{
//line files.gox:91
			__e = __c.Set("class", "nechat-rowmenu nechat-popbody nechat-files"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line files.gox:92
			__e = __c.Any(l.body); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line files.gox:94
}

//line files.gox:96
func (l *fileList) content() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:98
		entries := []api.FSEntry{}
	var failed error
	for _, dir := range []string{api.UserUploadsDir, api.AgentUploadsDir} {
		found, err := l.fs.ReadDir(ctx, dir)
		if err != nil {
			doors.Logger(ctx).Error("nechat: files", "dir", dir, "err", err)
			failed = err
			continue
		}
		entries = append(entries, found...)
	}
	slices.SortFunc(entries, func(a api.FSEntry, b api.FSEntry) int {
		return b.ModTime.Compare(a.ModTime)
	})

//line files.gox:113
		if failed != nil {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line files.gox:114
				__e = __c.Set("class", "nechat-side-retry"); if __e != nil { return }
//line files.gox:114
				__e = __c.Set("role", "alert"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Couldn't load."); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line files.gox:116
				__e = (doors.AClick{
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, rp doors.RequestPointer) bool {
					l.body.Reload(ctx)
					return false
				},
				OnError: notify.OnError,
			}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
					ctx := __c.Context(); _ = ctx
					__e = __c.Init("button"); if __e != nil { return }
					{
//line files.gox:123
						__e = __c.Set("type", "button"); if __e != nil { return }
//line files.gox:123
						__e = __c.Set("class", "nechat-btn nechat-btn_2nd"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Retry"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				return })); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line files.gox:125
		} else if len(entries) == 0 {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line files.gox:126
				__e = __c.Set("class", "nechat-side-empty"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("No files yet"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line files.gox:128
					__e = __c.Set("class", "nechat-side-empty-hint"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Attachments and anything the agent saves show up here"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("div"); if __e != nil { return }
			{
//line files.gox:131
				__e = __c.Set("class", "nechat-pop-group"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line files.gox:132
				for _, e := range entries {
//line files.gox:133
					__e = __c.Any(l.row(e)); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line files.gox:137
}

//line files.gox:139
func (l *fileList) row(e api.FSEntry) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line files.gox:141
		name := path.Base(e.Name)
	del := doors.NewSource(false)

//line files.gox:144
		__e = __c.Any(del.Bind(func(asking bool) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
//line files.gox:145
			if asking {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line files.gox:146
					__e = __c.Set("class", "nechat-form"); if __e != nil { return }
//line files.gox:146
					__e = __c.Set("role", "group"); if __e != nil { return }
//line files.gox:146
					__e = __c.Set("aria-label", "Delete file"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line files.gox:147
						__e = __c.Set("class", "nechat-form-text"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("Delete "); if __e != nil { return }
						__e = __c.Init("b"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:147
							__e = __c.Any(name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Text("?"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line files.gox:148
						__e = __c.Set("class", "nechat-form-actions"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line files.gox:149
						__e = (doors.AClick{
						Scope: new(doors.ScopeBlocking),
						On: func(ctx context.Context, rp doors.RequestPointer) bool {
							del.Update(ctx, false)
							return false
						},
						OnError: notify.OnError,
					}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line files.gox:156
								__e = __c.Set("type", "button"); if __e != nil { return }
//line files.gox:156
								__e = __c.Set("class", "nechat-btn nechat-btn_2nd"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("Cancel"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						return })); if __e != nil { return }
//line files.gox:157
						__e = (doors.AClick{
						Scope: new(doors.ScopeBlocking),
						On: func(ctx context.Context, rp doors.RequestPointer) bool {
							if err := l.fs.DeleteUpload(ctx, e.Name); err != nil {
								doors.Logger(ctx).Error("nechat: file delete", "name", e.Name, "err", err)
								notify.Error(ctx, err)
							}
							l.body.Reload(ctx)
							return false
						},
						OnError: notify.OnError,
					}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
							ctx := __c.Context(); _ = ctx
							__e = __c.Init("button"); if __e != nil { return }
							{
//line files.gox:168
								__e = __c.Set("type", "button"); if __e != nil { return }
//line files.gox:168
								__e = __c.Set("class", "nechat-btn nechat-btn_del"); if __e != nil { return }
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
//line files.gox:172
					__e = __c.Set("class", "nechat-entry"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line files.gox:173
					__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
//line files.gox:173
						if strings.HasPrefix(e.Name, api.UserUploadsDir + "/") {
//line files.gox:174
							__e = __c.Any(icons.UploadSimple()); if __e != nil { return }
						} else  {
//line files.gox:176
							__e = __c.Any(icons.DownloadSimple()); if __e != nil { return }
						}
					return })); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line files.gox:178
						__e = __c.Set("class", "nechat-entry-body"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("a"); if __e != nil { return }
						{
//line files.gox:179
							__e = __c.Set("class", "nechat-entry-name"); if __e != nil { return }
//line files.gox:179
							__e = __c.Set("href", download(l.fs, e.Name)); if __e != nil { return }
//line files.gox:179
							__e = __c.Set("download", name); if __e != nil { return }
//line files.gox:179
							__e = __c.Set("title", name); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:179
							__e = __c.Any(name); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line files.gox:180
							__e = __c.Set("class", "nechat-entry-detail"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:180
							__e = __c.Any(size(e.Size) + " · " + when(e.ModTime, l.loc)); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line files.gox:182
					__e = (doors.AClick{
					Scope: new(doors.ScopeBlocking),
					On: func(ctx context.Context, rp doors.RequestPointer) bool {
						del.Update(ctx, true)
						return false
					},
					OnError: notify.OnError,
				}).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
						ctx := __c.Context(); _ = ctx
						__e = __c.Init("button"); if __e != nil { return }
						{
//line files.gox:189
							__e = __c.Set("class", "nechat-ctl"); if __e != nil { return }
//line files.gox:189
							__e = __c.Set("aria-label", "Delete " + name); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line files.gox:190
							__e = (doors.Class("nechat-ic")).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
//line files.gox:190
								__e = __c.Any(icons.Trash()); if __e != nil { return }
							return })); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					return })); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		return })
//line files.gox:194
	})); if __e != nil { return }
	return })
//line files.gox:195
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
