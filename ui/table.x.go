// Managed by GoX v0.3.4

//line table.gox:1
package ui

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
)
type checkboxState int

const (
	checkboxUnchecked checkboxState = iota
	checkboxChecked
	checkboxDisabled
)

type tableWidget struct {
	params table
	state doors.Source[tableState]
	body doors.Beam[int]
	selected doors.Source[map[string]struct{}]
	submit func(ctx context.Context, s tableSubmit) bool
}

// NewTable builds a table widget for Library.Control. Its rows are either fixed
// (the rows param) or fetched a page at a time by the query param, with
// numbered pages when the query reports a total and previous and next
// otherwise. Columns marked sort get header controls that sort ascending and
// then flip; the search param adds a search input. Fixed rows are searched
// and sorted by the table, a query gets the sort and search text on every
// call; either change goes back to the first page. A click on a row's first
// cell (when clickable) submits the row id, and a click on one of its action
// buttons submits the row id and the button's value, to the script waiting
// in ui.draw(). When selectable, rows
// get checkboxes; the selection, at most 1000 ids, survives page changes and
// reloads, and its button submits the selected ids, sorted. An
// empty patch reloads the rows.
func NewTable(ctx context.Context, p table, submit func(ctx context.Context, s tableSubmit) bool) (api.Widget[tablePatch], error) {
	p.initDefault()
	state := doors.NewSourceEqual(p.state(), tableStateEqual)
	return tableWidget{
		params: p,
		state: state,
		body: doors.DeriveBeam(state, func(s tableState) int {
			return s.seq
		}),
		selected: doors.DeriveSourceEqual(state, func(s tableState) map[string]struct{} {
			return s.selected
		}, func(s tableState, selected map[string]struct{}) tableState {
			s.selected = selected
			return s
		}, maps.Equal),
		submit: submit,
	}, nil
}

func (t tableWidget) Update(ctx context.Context, patch tablePatch) {
	t.state.Mutate(ctx, func(s tableState) tableState {
		s.seq++
		if patch.Query != "" {
			s.query, s.rows = patch.Query, nil
			s.page = 1
			s.selected = nil
		}
		if patch.Rows != nil {
			s.query, s.rows = "", tableRowsPlain(patch.Rows)
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
			s.sort = &tableSort{Column: patch.Sort.Column, Desc: patch.Sort.Desc}
			s.page = 1
		}
		return s
	})
}

//line table.gox:104
func (t tableWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("div"); if __e != nil { return }
		{
//line table.gox:105
			__e = __c.Set("class", "kisland-table"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line table.gox:106
			if t.params.Search {
				__e = __c.InitVoid("input"); if __e != nil { return }
				{
//line table.gox:108
					__e = __c.Set("type", "search"); if __e != nil { return }
//line table.gox:109
					__e = __c.Set("class", "kisland-form-search-input kisland-table-search"); if __e != nil { return }
//line table.gox:110
					__e = __c.Set("placeholder", "Search"); if __e != nil { return }
//line table.gox:111
					__e = __c.Set("aria-label", "Search"); if __e != nil { return }
//line table.gox:112
					__e = __c.Set("maxlength", "100"); if __e != nil { return }
//line table.gox:113
					__e = __c.Modify(doors.AInput{
					Scope: &doors.ScopeDebounce{Duration: 250 * time.Millisecond, Limit: time.Second},
					Race: doors.RaceStrict,
					On: func(ctx context.Context, r doors.RequestInput) bool {
						text := strings.TrimSpace(r.Event().Value)
						if utf8.RuneCountInString(text) > 100 {
							text = string([]rune(text)[:100])
						}
						t.state.Mutate(ctx, func(s tableState) tableState {
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
//line table.gox:132
			__e = __c.Any(t.body.Bind(func(i int) gox.Elem {
			return t.render()
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:136
}

//line table.gox:138
func (t tableWidget) render() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:140
		state, ok := t.state.Read(ctx)
	if !ok {
		return
	}
	content, err := state.data(ctx)
	if err == nil && len(content.Rows) == 0 && state.isQuerySource() && state.page != 1 {
		t.state.Mutate(ctx, func(ts tableState) tableState {
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
	size := state.size
	page := state.page
	failed := ""
	if err != nil {
		failed = "Could not load the rows."
	}
	actions := slices.ContainsFunc(content.Rows, func(r tableRow) bool {
		return len(r.Actions) > 0
	})
	columnCount := len(t.params.Columns)
	if actions {
		columnCount++
	}
	if t.params.Selectable {
		columnCount++
	}

		__e = __c.InitContainer(); if __e != nil { return }
		{
			__e = __c.Init("div"); if __e != nil { return }
			{
//line table.gox:181
				__e = __c.Set("class", "kisland-table-box"); if __e != nil { return }
//line table.gox:181
				__e = __c.Set("tabindex", "0"); if __e != nil { return }
//line table.gox:181
				__e = __c.Set("role", "region"); if __e != nil { return }
//line table.gox:181
				__e = __c.Set("aria-label", "Table"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("table"); if __e != nil { return }
				{
//line table.gox:182
					__e = __c.Set("class", "kisland-table-grid"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("thead"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("tr"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:185
							__e = __c.Any(func() any {
							if !t.params.Selectable {
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
//line table.gox:193
									__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:193
									__e = __c.Set("class", "kisland-table-head-cell kisland-table-check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("label"); if __e != nil { return }
									{
//line table.gox:194
										__e = __c.Set("class", "kisland-table-check-target"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:194
										__e = __c.Any(t.checkboxHeader(ids)); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line table.gox:195
							return })
						}()); if __e != nil { return }
//line table.gox:197
							for i, c := range t.params.Columns {
//line table.gox:198
								__e = __c.Any(t.header(i, c, state.sort)); if __e != nil { return }
							}
//line table.gox:200
							if actions {
								__e = __c.Init("th"); if __e != nil { return }
								{
//line table.gox:201
									__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:201
									__e = __c.Set("class", "kisland-table-head-cell"); if __e != nil { return }
//line table.gox:201
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
//line table.gox:206
						if failed != "" {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line table.gox:207
								__e = __c.Set("class", "kisland-table-message-row"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line table.gox:207
									__e = __c.Set("class", "kisland-table-cell kisland-table-error"); if __e != nil { return }
//line table.gox:207
									__e = __c.Set("colspan", columnCount); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:207
									__e = __c.Any(failed); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line table.gox:208
						} else if len(content.Rows) == 0 {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line table.gox:209
								__e = __c.Set("class", "kisland-table-message-row"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line table.gox:210
									__e = __c.Set("class", "kisland-table-cell kisland-table-empty"); if __e != nil { return }
//line table.gox:210
									__e = __c.Set("colspan", columnCount); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:211
									__e = __c.Any(func() any {
									if state.search != "" {
										return "No matching rows"
									}
									return "No rows"
								}()); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						} else  {
//line table.gox:220
							for _, r := range content.Rows {
								__e = __c.Init("tr"); if __e != nil { return }
								{
//line table.gox:221
									__e = __c.Set("class", "kisland-table-row"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:222
									if t.params.Selectable {
										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:223
											__e = __c.Set("class", "kisland-table-cell kisland-table-check"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:224
											__e = __c.Any(func() any {
											label := r.ID
											if len(r.Cells) > 0 && r.Cells[0] != "" {
												label = cmp.Or(markdown.Plain(markdown.Inline(string(r.Cells[0]))), r.ID)
											}
											return gox.Elem(func(__c gox.Cursor) (__e error) {
												ctx := __c.Context(); _ = ctx
												__e = __c.Init("label"); if __e != nil { return }
												{
//line table.gox:229
													__e = __c.Set("class", "kisland-table-check-target"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:229
													__e = __c.Any(t.checkbox("Select " + label, r.ID)); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
//line table.gox:229
											return })
										}()); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line table.gox:233
									for i := range t.params.Columns {
//line table.gox:235
										c := ""
									if i < len(r.Cells) {
										c = string(r.Cells[i])
									}

										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:240
											__e = __c.Set("class", "kisland-table-cell"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:241
											if i == 0 && t.params.Clickable {
												__e = __c.Init("button"); if __e != nil { return }
												{
//line table.gox:243
													__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:244
													__e = __c.Set("class", "kisland-table-link"); if __e != nil { return }
//line table.gox:245
													__e = __c.Modify(doors.AClick{
													Scope: neui.IslandScope(ctx),
													On: func(ctx context.Context, _ doors.RequestPointer) bool {
														t.submit(ctx, tableSubmit{ID: r.ID})
														return false
													},
												}); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:252
													__e = __c.Any(markdown.Plain(markdown.Inline(c))); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
//line table.gox:254
											} else if c != "" {
												__e = __c.Init("span"); if __e != nil { return }
												{
//line table.gox:255
													__e = __c.Set("class", "kisland-prose kisland-table-prose"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:255
													__e = __c.Any(markdownBody(neui.IslandFS(ctx), markdown.Inline(c))); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											}
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line table.gox:259
									if actions {
										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:260
											__e = __c.Set("class", "kisland-table-cell kisland-table-actions"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:261
											for _, a := range r.Actions {
												__e = __c.Init("button"); if __e != nil { return }
												{
//line table.gox:263
													__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:264
													__e = __c.Set("class", "kisland-button kisland-table-action kisland-button-" + cmp.Or(a.Style, "secondary")); if __e != nil { return }
//line table.gox:265
													__e = __c.Modify(doors.AClick{
													Scope: neui.IslandScope(ctx),
													On: func(ctx context.Context, _ doors.RequestPointer) bool {
														t.submit(ctx, tableSubmit{ID: r.ID, Value: a.Value})
														return false
													},
												}); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:272
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
//line table.gox:279
							if page > 1 || content.Total != nil && *content.Total > size {
//line table.gox:280
								for range size - len(content.Rows) {
									__e = __c.Init("tr"); if __e != nil { return }
									{
//line table.gox:281
										__e = __c.Set("class", "kisland-table-filler"); if __e != nil { return }
//line table.gox:281
										__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:282
										if t.params.Selectable {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:283
												__e = __c.Set("class", "kisland-table-cell kisland-table-check"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
//line table.gox:285
										for range t.params.Columns {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:286
												__e = __c.Set("class", "kisland-table-cell"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
//line table.gox:288
										if actions {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:289
												__e = __c.Set("class", "kisland-table-cell kisland-table-actions"); if __e != nil { return }
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
//line table.gox:298
				__e = __c.Set("class", "kisland-table-footer"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line table.gox:299
					__e = __c.Set("class", "kisland-table-status"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:300
					__e = __c.Any(t.selected.Bind(func(sel map[string]struct{}) gox.Elem {
					return t.status(sel, content, page, size)
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:304
				__e = __c.Any(t.pager(content, page, size)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:307
}

//line table.gox:309
func (t tableWidget) header(i int, c tableColumn, sort *tableSort) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:310
		if !c.Sort {
			__e = __c.Init("th"); if __e != nil { return }
			{
//line table.gox:311
				__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:311
				__e = __c.Set("class", "kisland-table-head-cell"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:311
				__e = __c.Any(c.Title); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("th"); if __e != nil { return }
			{
//line table.gox:314
				__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:315
				__e = __c.Set("class", "kisland-table-head-cell"); if __e != nil { return }
//line table.gox:316
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
//line table.gox:326
					__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:327
					__e = __c.Set("class", "kisland-table-sort"); if __e != nil { return }
//line table.gox:328
					__e = __c.Modify(doors.AClick{
					On: func(ctx context.Context, _ doors.RequestPointer) bool {
						t.state.Mutate(ctx, func(s tableState) tableState {
							s.sort = &tableSort{Column: i, Desc: s.sort != nil && s.sort.Column == i && !s.sort.Desc}
							s.page = 1
							s.seq += 1
							return s
						})
						return false
					},
				}); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:339
					__e = __c.Any(c.Title); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line table.gox:340
						__e = __c.Set("class", "kisland-table-sort-icon"); if __e != nil { return }
//line table.gox:340
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line table.gox:341
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
//line table.gox:354
}

//line table.gox:356
func (t tableWidget) checkboxHeader(ids []string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:358
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

//line table.gox:375
		__e = __c.Any(check.Bind(func(c checkboxState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line table.gox:377
				__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line table.gox:378
				__e = __c.Set("class", "kisland-table-checkbox"); if __e != nil { return }
//line table.gox:379
				__e = __c.Set("aria-label", "Select all rows on this page"); if __e != nil { return }
//line table.gox:380
				__e = __c.Set("checked", c == checkboxChecked); if __e != nil { return }
//line table.gox:381
				__e = __c.Set("disabled", c == checkboxDisabled); if __e != nil { return }
//line table.gox:382
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
//line table.gox:403
	})); if __e != nil { return }
	return })
//line table.gox:404
}

//line table.gox:406
func (t tableWidget) checkbox(label string, id string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:408
		check := doors.DeriveBeam(t.selected, func(sel map[string]struct{}) checkboxState {
		if _, ok := sel[id]; ok {
			return checkboxChecked
		}
		if len(sel) >= 1000 {
			return checkboxDisabled
		}
		return checkboxUnchecked
	})

//line table.gox:418
		__e = __c.Any(check.Bind(func(c checkboxState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line table.gox:420
				__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line table.gox:421
				__e = __c.Set("class", "kisland-table-checkbox"); if __e != nil { return }
//line table.gox:422
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line table.gox:423
				__e = __c.Set("checked", c == checkboxChecked); if __e != nil { return }
//line table.gox:424
				__e = __c.Set("disabled", c == checkboxDisabled); if __e != nil { return }
//line table.gox:425
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
//line table.gox:444
	})); if __e != nil { return }
	return })
//line table.gox:445
}

//line table.gox:447
func (t tableWidget) status(sel map[string]struct{}, content tableResult, page int, size int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:448
		if len(sel) > 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:449
				__e = __c.Set("class", "kisland-table-status-text"); if __e != nil { return }
//line table.gox:449
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:449
					__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:449
					__e = __c.Any(len(sel)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text(" selected"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:451
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:452
				__e = __c.Set("class", "kisland-button kisland-button-sm kisland-button-primary"); if __e != nil { return }
//line table.gox:453
				__e = __c.Modify(doors.AClick{
				Scope: neui.IslandScope(ctx),
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.submit(ctx, tableSubmit{IDs: slices.Sorted(maps.Keys(sel))})
					return false
				},
			}); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Submit"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:463
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:464
				__e = __c.Set("class", "kisland-table-clear"); if __e != nil { return }
//line table.gox:465
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
//line table.gox:473
		} else if len(content.Rows) == 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:474
				__e = __c.Set("class", "kisland-table-status-text"); if __e != nil { return }
//line table.gox:474
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Nothing to show"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:476
				__e = __c.Set("class", "kisland-table-status-text"); if __e != nil { return }
//line table.gox:476
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Showing "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:476
					__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:476
					__e = __c.Any((page - 1) * size + 1); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text("–"); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:476
					__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:476
					__e = __c.Any((page - 1) * size + len(content.Rows)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:476
				if content.Total != nil {
					__e = __c.InitContainer(); if __e != nil { return }
					{
						__e = __c.Text(" of "); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line table.gox:477
							__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:477
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
//line table.gox:480
}

//line table.gox:482
func (t tableWidget) pager(content tableResult, page int, size int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:484
		pages := 0
	if content.Total != nil {
		pages = (*content.Total + size - 1) / size
	}
	first := max(1, min(page - 2, pages - 4))
	last := min(pages, first + 4)

//line table.gox:491
		if content.Total != nil && pages > 1 {
			__e = __c.Init("nav"); if __e != nil { return }
			{
//line table.gox:492
				__e = __c.Set("class", "kisland-table-pager"); if __e != nil { return }
//line table.gox:492
				__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:493
				__e = __c.Any(t.pageButton("First", 1, page > 1)); if __e != nil { return }
//line table.gox:494
				__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:495
				for n := first; n <= last; n++ {
//line table.gox:496
					if n == page {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line table.gox:497
							__e = __c.Set("class", "kisland-table-current"); if __e != nil { return }
//line table.gox:497
							__e = __c.Set("aria-current", "page"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:497
							__e = __c.Any(n); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
//line table.gox:499
						__e = __c.Any(t.pageButton(strconv.Itoa(n), n, true)); if __e != nil { return }
					}
				}
//line table.gox:502
				__e = __c.Any(t.pageButton("Next", page + 1, page < pages)); if __e != nil { return }
//line table.gox:503
				__e = __c.Any(t.pageButton("Last", pages, page < pages)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line table.gox:505
		} else if content.Total == nil && (page > 1 || len(content.Rows) == size) {
			__e = __c.Init("nav"); if __e != nil { return }
			{
//line table.gox:506
				__e = __c.Set("class", "kisland-table-pager"); if __e != nil { return }
//line table.gox:506
				__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:507
				__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:508
				__e = __c.Any(t.pageButton("Next", page + 1, len(content.Rows) == size)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:511
}

//line table.gox:513
func (t tableWidget) pageButton(label string, page int, enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:514
		if !enabled {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:515
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:515
				__e = __c.Set("class", "kisland-button kisland-table-page kisland-button-secondary"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:516
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:520
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:521
				__e = __c.Set("class", "kisland-button kisland-table-page kisland-button-secondary"); if __e != nil { return }
//line table.gox:522
				__e = __c.Modify(doors.AClick{
				On: func(ctx context.Context, _ doors.RequestPointer) bool {
					t.state.Mutate(ctx, func(s tableState) tableState {
						s.page = page
						s.seq += 1
						return s
					})
					return false
				},
			}); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:532
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:535
}
