// Managed by GoX v0.4.0

//line table.gox:1
package table

import (
	"cmp"
	"context"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/markdown"
	"neui.dev/neisland/internal/files"
)
type checkboxState int

const (
	checkboxUnchecked checkboxState = iota
	checkboxChecked
	checkboxDisabled
)

type widget struct {
	params Params
	state doors.Source[state]
	body doors.Beam[int]
	selected doors.Source[map[string]struct{}]
	submit func(ctx context.Context, s Submit) bool
}

// New builds a table widget for Library.Control. Its rows are either fixed
// (the rows param) or fetched a page at a time by the query param, with
// numbered pages when the query reports a total and previous and next
// otherwise. Columns marked sort get header controls that sort ascending and
// then flip; the search param adds a search input. Fixed rows are searched
// and sorted by the table, a query gets the sort and search text on every
// call; either change goes back to the first page. A click on a row's first
// cell (when clickable) submits the row id, and a click on one of its action
// buttons submits the row id and the button's value, to the script waiting
// in ui.draw(). With select, rows get checkboxes; the selection, at most
// 1000 ids, survives page changes and reloads, and the select button submits
// the selected ids, sorted. An
// empty patch reloads the rows.
func New(ctx context.Context, p Params, submit func(ctx context.Context, s Submit) bool) (api.Widget[Patch], error) {
	p.initDefault()
	source := doors.NewSourceEqual(p.state(), stateEqual)
	return widget{
		params: p,
		state: source,
		body: doors.DeriveBeam(source, func(s state) int {
			return s.seq
		}),
		selected: doors.DeriveSourceEqual(source, func(s state) map[string]struct{} {
			return s.selected
		}, func(s state, selected map[string]struct{}) state {
			s.selected = selected
			return s
		}, maps.Equal),
		submit: submit,
	}, nil
}

func (t widget) Update(ctx context.Context, patch Patch) {
	t.state.Mutate(ctx, func(s state) state {
		s.seq++
		if patch.Query != "" {
			s.query, s.rows = patch.Query, nil
			s.page = 1
			s.selected = nil
		}
		if patch.Rows != nil {
			s.query, s.rows = "", rowsPlain(patch.Rows)
			s.page = 1
			kept := make(map[string]struct{})
			for _, r := range patch.Rows {
				if _, ok := s.selected[r.ID]; ok {
					kept[r.ID] = struct{}{}
				}
			}
			s.selected = kept
		}
		if patch.PageSize != 0 {
			s.size = patch.PageSize
			s.page = 1
		}
		if patch.Selected != nil {
			s.selected = make(map[string]struct{}, len(patch.Selected))
			for _, id := range patch.Selected {
				s.selected[id] = struct{}{}
			}
		}
		if patch.Sort != nil && patch.Sort.Column >= 0 && patch.Sort.Column < len(t.params.Columns) && t.params.Columns[patch.Sort.Column].Sort {
			s.sort = &sortSpec{Column: patch.Sort.Column, Desc: patch.Sort.Desc}
			s.page = 1
		}
		return s
	})
}

//line table.gox:105
func (t widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:107
		current, _ := t.state.Read(ctx)

		__e = __c.Init("div"); if __e != nil { return }
		{
//line table.gox:109
			__e = __c.Set("class", "neisland-table-card"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line table.gox:110
			if t.params.Search {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line table.gox:111
					__e = __c.Set("class", "neisland-table-card-head"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line table.gox:113
						__e = __c.Set("type", "search"); if __e != nil { return }
//line table.gox:114
						__e = __c.Set("class", "neisland-form-search-input neisland-table-search"); if __e != nil { return }
//line table.gox:115
						__e = __c.Set("value", current.search); if __e != nil { return }
//line table.gox:116
						__e = __c.Set("placeholder", "Search"); if __e != nil { return }
//line table.gox:117
						__e = __c.Set("aria-label", "Search"); if __e != nil { return }
//line table.gox:118
						__e = __c.Set("maxlength", "100"); if __e != nil { return }
//line table.gox:119
						__e = __c.Modify(doors.AInput{
						Scope: &doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second},
						Race: doors.RaceStrict,
						On: func(ctx context.Context, r doors.RequestInput) bool {
							text := strings.TrimSpace(r.Event().Value)
							if utf8.RuneCountInString(text) > 100 {
								text = string([]rune(text)[:100])
							}
							t.state.Mutate(ctx, func(s state) state {
								if s.search != text {
									s.search, s.page = text, 1
									s.seq += 1
								}
								return s
							})
							return false
						},
					}); if __e != nil { return }
					}
					__e = __c.Submit(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
//line table.gox:139
			__e = __c.Any(t.body.Bind(func(i int) gox.Elem {
			return t.render()
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:143
}

//line table.gox:145
func (t widget) render() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:147
		current, ok := t.state.Read(ctx)
	if !ok {
		return
	}
	content, err := current.data(ctx)
	if err == nil && len(content.Rows) == 0 && current.isQuerySource() && current.page != 1 {
		t.state.Mutate(ctx, func(ts state) state {
			if ts.page == 1 {
				return ts
			}
			last := ts.page - 1
			if content.Total != nil {
				last = max(1, (*content.Total + ts.size - 1) / ts.size)
			}
			if last >= ts.page {
				last = 1
			}
			ts.page = last
			ts.seq += 1
			return ts
		})
		return
	}
	size := current.size
	page := current.page
	failed := ""
	if err != nil {
		failed = "Could not load the rows."
	}
	actions := slices.ContainsFunc(content.Rows, func(r row) bool {
		return len(r.Actions) > 0
	})
	columnCount := len(t.params.Columns)
	if actions {
		columnCount++
	}
	if t.params.Select != nil {
		columnCount++
	}

		__e = __c.InitContainer(); if __e != nil { return }
		{
			__e = __c.Init("div"); if __e != nil { return }
			{
//line table.gox:188
				__e = __c.Set("class", "neisland-table-card-box"); if __e != nil { return }
//line table.gox:188
				__e = __c.Set("tabindex", "0"); if __e != nil { return }
//line table.gox:188
				__e = __c.Set("role", "region"); if __e != nil { return }
//line table.gox:188
				__e = __c.Set("aria-label", "Table"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("table"); if __e != nil { return }
				{
//line table.gox:189
					__e = __c.Set("class", "neisland-table-grid"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("thead"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("tr"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:192
							__e = __c.Any(func() any {
							if t.params.Select == nil {
								return nil
							}
							ids := make([]string, len(content.Rows))
							for i, r := range content.Rows {
								ids[i] = r.ID
							}
							return gox.Elem(func(__c gox.Cursor) (__e error) {
								ctx := __c.Context(); _ = ctx
								__e = __c.Init("th"); if __e != nil { return }
								{
//line table.gox:200
									__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:200
									__e = __c.Set("class", "neisland-table-head-cell neisland-table-check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("label"); if __e != nil { return }
									{
//line table.gox:201
										__e = __c.Set("class", "neisland-table-check-target"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:201
										__e = __c.Any(t.checkboxHeader(ids)); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line table.gox:202
							return })
						}()); if __e != nil { return }
//line table.gox:204
							for i, c := range t.params.Columns {
//line table.gox:205
								__e = __c.Any(t.header(i, c, current.sort)); if __e != nil { return }
							}
//line table.gox:207
							if actions {
								__e = __c.Init("th"); if __e != nil { return }
								{
//line table.gox:208
									__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:208
									__e = __c.Set("class", "neisland-table-head-cell"); if __e != nil { return }
//line table.gox:208
									__e = __c.Set("aria-label", "Actions"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("tbody"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line table.gox:213
						if failed != "" {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line table.gox:214
								__e = __c.Set("class", "neisland-table-message-row"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line table.gox:214
									__e = __c.Set("class", "neisland-table-cell neisland-table-error"); if __e != nil { return }
//line table.gox:214
									__e = __c.Set("colspan", columnCount); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:214
									__e = __c.Any(failed); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line table.gox:215
						} else if len(content.Rows) == 0 {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line table.gox:216
								__e = __c.Set("class", "neisland-table-message-row"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line table.gox:217
									__e = __c.Set("class", "neisland-table-cell neisland-table-empty"); if __e != nil { return }
//line table.gox:217
									__e = __c.Set("colspan", columnCount); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:218
									__e = __c.Any(func() any {
									if current.search != "" {
										return "No matching rows"
									}
									return "No rows"
								}()); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						} else  {
//line table.gox:227
							for _, r := range content.Rows {
								__e = __c.Init("tr"); if __e != nil { return }
								{
//line table.gox:228
									__e = __c.Set("class", "neisland-table-row"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:229
									if t.params.Select != nil {
										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:230
											__e = __c.Set("class", "neisland-table-cell neisland-table-check"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:231
											__e = __c.Any(func() any {
											label := r.ID
											if len(r.Cells) > 0 && r.Cells[0] != "" {
												label = cmp.Or(markdown.Plain(markdown.Inline(string(r.Cells[0]))), r.ID)
											}
											return gox.Elem(func(__c gox.Cursor) (__e error) {
												ctx := __c.Context(); _ = ctx
												__e = __c.Init("label"); if __e != nil { return }
												{
//line table.gox:236
													__e = __c.Set("class", "neisland-table-check-target"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:236
													__e = __c.Any(t.checkbox("Select " + label, r.ID)); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
//line table.gox:236
											return })
										}()); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line table.gox:240
									for i := range t.params.Columns {
//line table.gox:242
										c := ""
									if i < len(r.Cells) {
										c = string(r.Cells[i])
									}

										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:247
											__e = __c.Set("class", "neisland-table-cell"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:248
											if i == 0 && t.params.Clickable {
												__e = __c.Init("button"); if __e != nil { return }
												{
//line table.gox:250
													__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:251
													__e = __c.Set("class", "neisland-table-link"); if __e != nil { return }
//line table.gox:252
													__e = __c.Modify(doors.AClick{
													Scope: neui.IslandScope(ctx),
													On: func(ctx context.Context, _ doors.RequestPointer) bool {
														t.submit(ctx, Submit{ID: r.ID})
														return false
													},
												}); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:259
													__e = __c.Any(markdown.Plain(markdown.Inline(c))); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
//line table.gox:261
											} else if c != "" {
												__e = __c.Init("span"); if __e != nil { return }
												{
//line table.gox:262
													__e = __c.Set("class", "neisland-prose neisland-table-prose"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:262
													__e = __c.Any(files.MarkdownBody(neui.IslandFS(ctx), markdown.Inline(c))); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											}
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line table.gox:266
									if actions {
										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:267
											__e = __c.Set("class", "neisland-table-cell neisland-table-actions"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:268
											for _, a := range r.Actions {
												__e = __c.Init("button"); if __e != nil { return }
												{
//line table.gox:270
													__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:271
													__e = __c.Set("class", "neisland-button neisland-table-action neisland-button-" + cmp.Or(a.Style, "secondary")); if __e != nil { return }
//line table.gox:272
													__e = __c.Modify(doors.AClick{
													Scope: neui.IslandScope(ctx),
													On: func(ctx context.Context, _ doors.RequestPointer) bool {
														t.submit(ctx, Submit{ID: r.ID, Value: a.Value})
														return false
													},
												}); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:279
													__e = __c.Any(a.Text); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											}
										}
										__e = __c.Close(); if __e != nil { return }
									}
								}
								__e = __c.Close(); if __e != nil { return }
							}
//line table.gox:286
							if page > 1 || content.Total != nil && *content.Total > size {
//line table.gox:287
								for range size - len(content.Rows) {
									__e = __c.Init("tr"); if __e != nil { return }
									{
//line table.gox:288
										__e = __c.Set("class", "neisland-table-filler"); if __e != nil { return }
//line table.gox:288
										__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:289
										if t.params.Select != nil {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:290
												__e = __c.Set("class", "neisland-table-cell neisland-table-check"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
//line table.gox:292
										for range t.params.Columns {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:293
												__e = __c.Set("class", "neisland-table-cell"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
//line table.gox:295
										if actions {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:296
												__e = __c.Set("class", "neisland-table-cell neisland-table-actions"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
									}
									__e = __c.Close(); if __e != nil { return }
								}
							}
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("div"); if __e != nil { return }
			{
//line table.gox:305
				__e = __c.Set("class", "neisland-table-footer neisland-table-footer-band"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line table.gox:306
					__e = __c.Set("class", "neisland-table-status"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:307
					__e = __c.Any(t.selected.Bind(func(sel map[string]struct{}) gox.Elem {
					return t.status(sel, content, page, size)
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:311
				__e = __c.Any(t.pager(content, page, size)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:314
}

//line table.gox:316
func (t widget) header(i int, c column, sort *sortSpec) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:317
		if !c.Sort {
			__e = __c.Init("th"); if __e != nil { return }
			{
//line table.gox:318
				__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:318
				__e = __c.Set("class", "neisland-table-head-cell"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:318
				__e = __c.Any(c.Title); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("th"); if __e != nil { return }
			{
//line table.gox:321
				__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:322
				__e = __c.Set("class", "neisland-table-head-cell"); if __e != nil { return }
//line table.gox:323
				__e = __c.Set("aria-sort", func() any {
				if sort == nil || sort.Column != i {
					return nil
				}
				if sort.Desc {
					return "descending"
				}
				return "ascending"
			}()); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("button"); if __e != nil { return }
				{
//line table.gox:333
					__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:334
					__e = __c.Set("class", "neisland-table-sort"); if __e != nil { return }
//line table.gox:335
					__e = __c.Modify(doors.AClick{
					On: func(ctx context.Context, _ doors.RequestPointer) bool {
						t.state.Mutate(ctx, func(s state) state {
							s.sort = &sortSpec{Column: i, Desc: s.sort != nil && s.sort.Column == i && !s.sort.Desc}
							s.page = 1
							s.seq += 1
							return s
						})
						return false
					},
				}); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:346
					__e = __c.Any(c.Title); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line table.gox:347
						__e = __c.Set("class", "neisland-table-sort-icon"); if __e != nil { return }
//line table.gox:347
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line table.gox:348
						__e = __c.Any(func() any {
						if sort == nil || sort.Column != i {
							return "↕"
						}
						if sort.Desc {
							return "▼"
						}
						return "▲"
					}()); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:361
}

//line table.gox:363
func (t widget) checkboxHeader(ids []string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:365
		check := doors.DeriveBeam(t.selected, func(sel map[string]struct{}) checkboxState {
		n := 0
		for _, id := range ids {
			if _, ok := sel[id]; ok {
				n++
			}
		}
		switch {
		case len(ids) > 0 && n == len(ids):
			return checkboxChecked
		case len(ids) == 0 || len(sel) >= 1000:
			return checkboxDisabled
		default:
			return checkboxUnchecked
		}
	})

//line table.gox:382
		__e = __c.Any(check.Bind(func(c checkboxState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line table.gox:384
				__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line table.gox:385
				__e = __c.Set("class", "neisland-table-checkbox"); if __e != nil { return }
//line table.gox:386
				__e = __c.Set("aria-label", "Select all rows on this page"); if __e != nil { return }
//line table.gox:387
				__e = __c.Set("checked", c == checkboxChecked); if __e != nil { return }
//line table.gox:388
				__e = __c.Set("disabled", c == checkboxDisabled); if __e != nil { return }
//line table.gox:389
				__e = __c.Modify(doors.AClick{
				PreventDefault: true,
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.selected.Mutate(ctx, func(sel map[string]struct{}) map[string]struct{} {
						sel = maps.Clone(sel)
						if sel == nil {
							sel = make(map[string]struct{})
						}
						for _, id := range ids {
							if c == checkboxChecked {
								delete(sel, id)
							} else if len(sel) < 1000 {
								sel[id] = struct{}{}
							}
						}
						return sel
					})
					return false
				},
			}); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
		return })
//line table.gox:410
	})); if __e != nil { return }
	return })
//line table.gox:411
}

//line table.gox:413
func (t widget) checkbox(label string, id string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:415
		check := doors.DeriveBeam(t.selected, func(sel map[string]struct{}) checkboxState {
		if _, ok := sel[id]; ok {
			return checkboxChecked
		}
		if len(sel) >= 1000 {
			return checkboxDisabled
		}
		return checkboxUnchecked
	})

//line table.gox:425
		__e = __c.Any(check.Bind(func(c checkboxState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line table.gox:427
				__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line table.gox:428
				__e = __c.Set("class", "neisland-table-checkbox"); if __e != nil { return }
//line table.gox:429
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line table.gox:430
				__e = __c.Set("checked", c == checkboxChecked); if __e != nil { return }
//line table.gox:431
				__e = __c.Set("disabled", c == checkboxDisabled); if __e != nil { return }
//line table.gox:432
				__e = __c.Modify(doors.AClick{
				PreventDefault: true,
				Scope: new(doors.ScopeBlocking),
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.selected.Mutate(ctx, func(sel map[string]struct{}) map[string]struct{} {
						sel = maps.Clone(sel)
						if sel == nil {
							sel = make(map[string]struct{})
						}
						if c == checkboxChecked {
							delete(sel, id)
						} else if len(sel) < 1000 {
							sel[id] = struct{}{}
						}
						return sel
					})
					return false
				},
			}); if __e != nil { return }
			}
			__e = __c.Submit(); if __e != nil { return }
		return })
//line table.gox:451
	})); if __e != nil { return }
	return })
//line table.gox:452
}

//line table.gox:454
func (t widget) status(sel map[string]struct{}, content result, page int, size int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:455
		if len(sel) > 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:456
				__e = __c.Set("class", "neisland-table-status-text"); if __e != nil { return }
//line table.gox:456
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:456
					__e = __c.Set("class", "neisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:456
					__e = __c.Any(len(sel)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text(" selected"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:458
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:459
				__e = __c.Set("class", "neisland-button neisland-button-sm neisland-button-" + t.params.Select.Style); if __e != nil { return }
//line table.gox:460
				__e = __c.Modify(doors.AClick{
				Scope: neui.IslandScope(ctx),
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.submit(ctx, Submit{IDs: slices.Sorted(maps.Keys(sel))})
					return false
				},
			}); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:467
				__e = __c.Any(t.params.Select.Text); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:470
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:471
				__e = __c.Set("class", "neisland-table-clear"); if __e != nil { return }
//line table.gox:472
				__e = __c.Modify(doors.AClick{
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.selected.Update(ctx, nil)
					return false
				},
			}); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Clear"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line table.gox:480
		} else if len(content.Rows) == 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:481
				__e = __c.Set("class", "neisland-table-status-text"); if __e != nil { return }
//line table.gox:481
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Nothing to show"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:483
				__e = __c.Set("class", "neisland-table-status-text"); if __e != nil { return }
//line table.gox:483
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Showing "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:483
					__e = __c.Set("class", "neisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:483
					__e = __c.Any((page - 1) * size + 1); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text("–"); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:483
					__e = __c.Set("class", "neisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:483
					__e = __c.Any((page - 1) * size + len(content.Rows)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:483
				if content.Total != nil {
					__e = __c.InitContainer(); if __e != nil { return }
					{
						__e = __c.Text(" of "); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line table.gox:484
							__e = __c.Set("class", "neisland-table-count"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:484
							__e = __c.Any(*content.Total); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:487
}

//line table.gox:489
func (t widget) pager(content result, page int, size int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:491
		pages := 0
	if content.Total != nil {
		pages = (*content.Total + size - 1) / size
	}
	first := max(1, min(page - 2, pages - 4))
	last := min(pages, first + 4)

//line table.gox:498
		if content.Total != nil && pages > 1 {
			__e = __c.Init("nav"); if __e != nil { return }
			{
//line table.gox:499
				__e = __c.Set("class", "neisland-table-pager"); if __e != nil { return }
//line table.gox:499
				__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:500
				__e = __c.Any(t.pageButton("First", 1, page > 1)); if __e != nil { return }
//line table.gox:501
				__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:502
				for n := first; n <= last; n++ {
//line table.gox:503
					if n == page {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line table.gox:504
							__e = __c.Set("class", "neisland-table-current"); if __e != nil { return }
//line table.gox:504
							__e = __c.Set("aria-current", "page"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:504
							__e = __c.Any(n); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
//line table.gox:506
						__e = __c.Any(t.pageButton(strconv.Itoa(n), n, true)); if __e != nil { return }
					}
				}
//line table.gox:509
				__e = __c.Any(t.pageButton("Next", page + 1, page < pages)); if __e != nil { return }
//line table.gox:510
				__e = __c.Any(t.pageButton("Last", pages, page < pages)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line table.gox:512
		} else if content.Total == nil && (page > 1 || len(content.Rows) == size) {
			__e = __c.Init("nav"); if __e != nil { return }
			{
//line table.gox:513
				__e = __c.Set("class", "neisland-table-pager"); if __e != nil { return }
//line table.gox:513
				__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:514
				__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:515
				__e = __c.Any(t.pageButton("Next", page + 1, len(content.Rows) == size)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:518
}

//line table.gox:520
func (t widget) pageButton(label string, page int, enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:521
		if !enabled {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:522
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:522
				__e = __c.Set("class", "neisland-button neisland-table-page neisland-button-secondary"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:523
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:527
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:528
				__e = __c.Set("class", "neisland-button neisland-table-page neisland-button-secondary"); if __e != nil { return }
//line table.gox:529
				__e = __c.Modify(doors.AClick{
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.state.Mutate(ctx, func(s state) state {
						s.page = page
						s.seq += 1
						return s
					})
					return false
				},
			}); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:539
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:542
}
