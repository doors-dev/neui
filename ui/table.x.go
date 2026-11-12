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
			__e = __c.Set("class", "kisland-table-card"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line table.gox:106
			if t.params.Search {
				__e = __c.Init("div"); if __e != nil { return }
				{
//line table.gox:107
					__e = __c.Set("class", "kisland-table-card-head"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.InitVoid("input"); if __e != nil { return }
					{
//line table.gox:109
						__e = __c.Set("type", "search"); if __e != nil { return }
//line table.gox:110
						__e = __c.Set("class", "kisland-form-search-input kisland-table-search"); if __e != nil { return }
//line table.gox:111
						__e = __c.Set("placeholder", "Search"); if __e != nil { return }
//line table.gox:112
						__e = __c.Set("aria-label", "Search"); if __e != nil { return }
//line table.gox:113
						__e = __c.Set("maxlength", "100"); if __e != nil { return }
//line table.gox:114
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
				__e = __c.Close(); if __e != nil { return }
			}
//line table.gox:134
			__e = __c.Any(t.body.Bind(func(i int) gox.Elem {
			return t.render()
		})); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:138
}

//line table.gox:140
func (t tableWidget) render() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:142
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
//line table.gox:183
				__e = __c.Set("class", "kisland-table-card-box"); if __e != nil { return }
//line table.gox:183
				__e = __c.Set("tabindex", "0"); if __e != nil { return }
//line table.gox:183
				__e = __c.Set("role", "region"); if __e != nil { return }
//line table.gox:183
				__e = __c.Set("aria-label", "Table"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("table"); if __e != nil { return }
				{
//line table.gox:184
					__e = __c.Set("class", "kisland-table-grid"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("thead"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Init("tr"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:187
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
//line table.gox:195
									__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:195
									__e = __c.Set("class", "kisland-table-head-cell kisland-table-check"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
									__e = __c.Init("label"); if __e != nil { return }
									{
//line table.gox:196
										__e = __c.Set("class", "kisland-table-check-target"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:196
										__e = __c.Any(t.checkboxHeader(ids)); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
//line table.gox:197
							return })
						}()); if __e != nil { return }
//line table.gox:199
							for i, c := range t.params.Columns {
//line table.gox:200
								__e = __c.Any(t.header(i, c, state.sort)); if __e != nil { return }
							}
//line table.gox:202
							if actions {
								__e = __c.Init("th"); if __e != nil { return }
								{
//line table.gox:203
									__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:203
									__e = __c.Set("class", "kisland-table-head-cell"); if __e != nil { return }
//line table.gox:203
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
//line table.gox:208
						if failed != "" {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line table.gox:209
								__e = __c.Set("class", "kisland-table-message-row"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line table.gox:209
									__e = __c.Set("class", "kisland-table-cell kisland-table-error"); if __e != nil { return }
//line table.gox:209
									__e = __c.Set("colspan", columnCount); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:209
									__e = __c.Any(failed); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line table.gox:210
						} else if len(content.Rows) == 0 {
							__e = __c.Init("tr"); if __e != nil { return }
							{
//line table.gox:211
								__e = __c.Set("class", "kisland-table-message-row"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Init("td"); if __e != nil { return }
								{
//line table.gox:212
									__e = __c.Set("class", "kisland-table-cell kisland-table-empty"); if __e != nil { return }
//line table.gox:212
									__e = __c.Set("colspan", columnCount); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:213
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
//line table.gox:222
							for _, r := range content.Rows {
								__e = __c.Init("tr"); if __e != nil { return }
								{
//line table.gox:223
									__e = __c.Set("class", "kisland-table-row"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line table.gox:224
									if t.params.Selectable {
										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:225
											__e = __c.Set("class", "kisland-table-cell kisland-table-check"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:226
											__e = __c.Any(func() any {
											label := r.ID
											if len(r.Cells) > 0 && r.Cells[0] != "" {
												label = cmp.Or(markdown.Plain(markdown.Inline(string(r.Cells[0]))), r.ID)
											}
											return gox.Elem(func(__c gox.Cursor) (__e error) {
												ctx := __c.Context(); _ = ctx
												__e = __c.Init("label"); if __e != nil { return }
												{
//line table.gox:231
													__e = __c.Set("class", "kisland-table-check-target"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:231
													__e = __c.Any(t.checkbox("Select " + label, r.ID)); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
//line table.gox:231
											return })
										}()); if __e != nil { return }
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line table.gox:235
									for i := range t.params.Columns {
//line table.gox:237
										c := ""
									if i < len(r.Cells) {
										c = string(r.Cells[i])
									}

										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:242
											__e = __c.Set("class", "kisland-table-cell"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:243
											if i == 0 && t.params.Clickable {
												__e = __c.Init("button"); if __e != nil { return }
												{
//line table.gox:245
													__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:246
													__e = __c.Set("class", "kisland-table-link"); if __e != nil { return }
//line table.gox:247
													__e = __c.Modify(doors.AClick{
													Scope: neui.IslandScope(ctx),
													On: func(ctx context.Context, _ doors.RequestPointer) bool {
														t.submit(ctx, tableSubmit{ID: r.ID})
														return false
													},
												}); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:254
													__e = __c.Any(markdown.Plain(markdown.Inline(c))); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
//line table.gox:256
											} else if c != "" {
												__e = __c.Init("span"); if __e != nil { return }
												{
//line table.gox:257
													__e = __c.Set("class", "kisland-prose kisland-table-prose"); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:257
													__e = __c.Any(markdownBody(neui.IslandFS(ctx), markdown.Inline(c))); if __e != nil { return }
												}
												__e = __c.Close(); if __e != nil { return }
											}
										}
										__e = __c.Close(); if __e != nil { return }
									}
//line table.gox:261
									if actions {
										__e = __c.Init("td"); if __e != nil { return }
										{
//line table.gox:262
											__e = __c.Set("class", "kisland-table-cell kisland-table-actions"); if __e != nil { return }
											__e = __c.Submit(); if __e != nil { return }
//line table.gox:263
											for _, a := range r.Actions {
												__e = __c.Init("button"); if __e != nil { return }
												{
//line table.gox:265
													__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:266
													__e = __c.Set("class", "kisland-button kisland-table-action kisland-button-" + cmp.Or(a.Style, "secondary")); if __e != nil { return }
//line table.gox:267
													__e = __c.Modify(doors.AClick{
													Scope: neui.IslandScope(ctx),
													On: func(ctx context.Context, _ doors.RequestPointer) bool {
														t.submit(ctx, tableSubmit{ID: r.ID, Value: a.Value})
														return false
													},
												}); if __e != nil { return }
													__e = __c.Submit(); if __e != nil { return }
//line table.gox:274
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
//line table.gox:281
							if page > 1 || content.Total != nil && *content.Total > size {
//line table.gox:282
								for range size - len(content.Rows) {
									__e = __c.Init("tr"); if __e != nil { return }
									{
//line table.gox:283
										__e = __c.Set("class", "kisland-table-filler"); if __e != nil { return }
//line table.gox:283
										__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:284
										if t.params.Selectable {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:285
												__e = __c.Set("class", "kisland-table-cell kisland-table-check"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
//line table.gox:287
										for range t.params.Columns {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:288
												__e = __c.Set("class", "kisland-table-cell"); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										}
//line table.gox:290
										if actions {
											__e = __c.Init("td"); if __e != nil { return }
											{
//line table.gox:291
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
//line table.gox:300
				__e = __c.Set("class", "kisland-table-footer kisland-table-footer-band"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("div"); if __e != nil { return }
				{
//line table.gox:301
					__e = __c.Set("class", "kisland-table-status"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:302
					__e = __c.Any(t.selected.Bind(func(sel map[string]struct{}) gox.Elem {
					return t.status(sel, content, page, size)
				})); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:306
				__e = __c.Any(t.pager(content, page, size)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:309
}

//line table.gox:311
func (t tableWidget) header(i int, c tableColumn, sort *tableSort) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:312
		if !c.Sort {
			__e = __c.Init("th"); if __e != nil { return }
			{
//line table.gox:313
				__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:313
				__e = __c.Set("class", "kisland-table-head-cell"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:313
				__e = __c.Any(c.Title); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("th"); if __e != nil { return }
			{
//line table.gox:316
				__e = __c.Set("scope", "col"); if __e != nil { return }
//line table.gox:317
				__e = __c.Set("class", "kisland-table-head-cell"); if __e != nil { return }
//line table.gox:318
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
//line table.gox:328
					__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:329
					__e = __c.Set("class", "kisland-table-sort"); if __e != nil { return }
//line table.gox:330
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
//line table.gox:341
					__e = __c.Any(c.Title); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line table.gox:342
						__e = __c.Set("class", "kisland-table-sort-icon"); if __e != nil { return }
//line table.gox:342
						__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line table.gox:343
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
//line table.gox:356
}

//line table.gox:358
func (t tableWidget) checkboxHeader(ids []string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:360
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

//line table.gox:377
		__e = __c.Any(check.Bind(func(c checkboxState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line table.gox:379
				__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line table.gox:380
				__e = __c.Set("class", "kisland-table-checkbox"); if __e != nil { return }
//line table.gox:381
				__e = __c.Set("aria-label", "Select all rows on this page"); if __e != nil { return }
//line table.gox:382
				__e = __c.Set("checked", c == checkboxChecked); if __e != nil { return }
//line table.gox:383
				__e = __c.Set("disabled", c == checkboxDisabled); if __e != nil { return }
//line table.gox:384
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
//line table.gox:405
	})); if __e != nil { return }
	return })
//line table.gox:406
}

//line table.gox:408
func (t tableWidget) checkbox(label string, id string) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:410
		check := doors.DeriveBeam(t.selected, func(sel map[string]struct{}) checkboxState {
		if _, ok := sel[id]; ok {
			return checkboxChecked
		}
		if len(sel) >= 1000 {
			return checkboxDisabled
		}
		return checkboxUnchecked
	})

//line table.gox:420
		__e = __c.Any(check.Bind(func(c checkboxState) gox.Elem {
		return gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.InitVoid("input"); if __e != nil { return }
			{
//line table.gox:422
				__e = __c.Set("type", "checkbox"); if __e != nil { return }
//line table.gox:423
				__e = __c.Set("class", "kisland-table-checkbox"); if __e != nil { return }
//line table.gox:424
				__e = __c.Set("aria-label", label); if __e != nil { return }
//line table.gox:425
				__e = __c.Set("checked", c == checkboxChecked); if __e != nil { return }
//line table.gox:426
				__e = __c.Set("disabled", c == checkboxDisabled); if __e != nil { return }
//line table.gox:427
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
//line table.gox:446
	})); if __e != nil { return }
	return })
//line table.gox:447
}

//line table.gox:449
func (t tableWidget) status(sel map[string]struct{}, content tableResult, page int, size int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:450
		if len(sel) > 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:451
				__e = __c.Set("class", "kisland-table-status-text"); if __e != nil { return }
//line table.gox:451
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:451
					__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:451
					__e = __c.Any(len(sel)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text(" selected"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:453
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:454
				__e = __c.Set("class", "kisland-button kisland-button-sm kisland-button-primary"); if __e != nil { return }
//line table.gox:455
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
//line table.gox:465
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:466
				__e = __c.Set("class", "kisland-table-clear"); if __e != nil { return }
//line table.gox:467
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
//line table.gox:475
		} else if len(content.Rows) == 0 {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:476
				__e = __c.Set("class", "kisland-table-status-text"); if __e != nil { return }
//line table.gox:476
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Nothing to show"); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("span"); if __e != nil { return }
			{
//line table.gox:478
				__e = __c.Set("class", "kisland-table-status-text"); if __e != nil { return }
//line table.gox:478
				__e = __c.Set("role", "status"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Text("Showing "); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:478
					__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:478
					__e = __c.Any((page - 1) * size + 1); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Text("–"); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line table.gox:478
					__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:478
					__e = __c.Any((page - 1) * size + len(content.Rows)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:478
				if content.Total != nil {
					__e = __c.InitContainer(); if __e != nil { return }
					{
						__e = __c.Text(" of "); if __e != nil { return }
						__e = __c.Init("span"); if __e != nil { return }
						{
//line table.gox:479
							__e = __c.Set("class", "kisland-table-count"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:479
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
//line table.gox:482
}

//line table.gox:484
func (t tableWidget) pager(content tableResult, page int, size int) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:486
		pages := 0
	if content.Total != nil {
		pages = (*content.Total + size - 1) / size
	}
	first := max(1, min(page - 2, pages - 4))
	last := min(pages, first + 4)

//line table.gox:493
		if content.Total != nil && pages > 1 {
			__e = __c.Init("nav"); if __e != nil { return }
			{
//line table.gox:494
				__e = __c.Set("class", "kisland-table-pager"); if __e != nil { return }
//line table.gox:494
				__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:495
				__e = __c.Any(t.pageButton("First", 1, page > 1)); if __e != nil { return }
//line table.gox:496
				__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:497
				for n := first; n <= last; n++ {
//line table.gox:498
					if n == page {
						__e = __c.Init("span"); if __e != nil { return }
						{
//line table.gox:499
							__e = __c.Set("class", "kisland-table-current"); if __e != nil { return }
//line table.gox:499
							__e = __c.Set("aria-current", "page"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
//line table.gox:499
							__e = __c.Any(n); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
//line table.gox:501
						__e = __c.Any(t.pageButton(strconv.Itoa(n), n, true)); if __e != nil { return }
					}
				}
//line table.gox:504
				__e = __c.Any(t.pageButton("Next", page + 1, page < pages)); if __e != nil { return }
//line table.gox:505
				__e = __c.Any(t.pageButton("Last", pages, page < pages)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line table.gox:507
		} else if content.Total == nil && (page > 1 || len(content.Rows) == size) {
			__e = __c.Init("nav"); if __e != nil { return }
			{
//line table.gox:508
				__e = __c.Set("class", "kisland-table-pager"); if __e != nil { return }
//line table.gox:508
				__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:509
				__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:510
				__e = __c.Any(t.pageButton("Next", page + 1, len(content.Rows) == size)); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:513
}

//line table.gox:515
func (t tableWidget) pageButton(label string, page int, enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:516
		if !enabled {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:517
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:517
				__e = __c.Set("class", "kisland-button kisland-table-page kisland-button-secondary"); if __e != nil { return }
				__e = __c.Set("disabled", true); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line table.gox:518
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		} else  {
			__e = __c.Init("button"); if __e != nil { return }
			{
//line table.gox:522
				__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:523
				__e = __c.Set("class", "kisland-button kisland-table-page kisland-button-secondary"); if __e != nil { return }
//line table.gox:524
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
//line table.gox:534
				__e = __c.Any(label); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		}
	return })
//line table.gox:537
}
