// Managed by GoX v0.3.3

//line table.gox:1
package ui

import (
	"cmp"
	"context"
	"strconv"
	
	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"github.com/swaggest/jsonschema-go"
	"neui.dev"
	"neui.dev/api"
)

type tablePage struct {
	_ struct{} `additionalProperties:"false"`
	Offset int `json:"offset" required:"true" minimum:"0" description:"index of the first row to return"`
	Limit int `json:"limit" required:"true" minimum:"1" description:"how many rows to return at most"`
}

type tableResult struct {
	_ struct{} `additionalProperties:"false"`
	Rows []tableRow `json:"rows" required:"true" nullable:"false" maxItems:"100" description:"rows from offset on, at most limit of them"`
	Total *int `json:"total,omitempty" minimum:"0" description:"number of rows in the whole table; omit when unknown, and the table offers only previous and next"`
}

type cellButton struct {
	_ struct{} `additionalProperties:"false"`
	Value string `json:"value" required:"true" minLength:"1" maxLength:"1000" description:"data the click submits with the row id"`
	Style string `json:"style,omitempty" enum:"primary,secondary,success,danger" default:"secondary" description:"visual weight: primary for the main action, danger for destructive ones"`
	Text string `json:"text" required:"true" minLength:"1" maxLength:"80" description:"label shown on the button"`
}

type cellText string

func (cellText) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithMaxLength(500)
	return nil
}

type tableRow struct {
	_ struct{} `additionalProperties:"false"`
	ID string `json:"id" required:"true" minLength:"1" maxLength:"200" description:"identifies the row in what a click submits"`
	Cells []cellText `json:"cells" required:"true" nullable:"false" description:"one text per column, in column order"`
	Actions []cellButton `json:"actions,omitempty" maxItems:"5" description:"buttons shown at the end of the row"`
}

type tableSubmit struct {
	ID string `json:"id" description:"id of the row the user acted on"`
	Value string `json:"value,omitempty" description:"value of the action button clicked; empty when the row's first column was clicked"`
}

type tablePatch struct {
	_ struct{} `additionalProperties:"false" description:"an empty patch reloads the rows: the current page of query, or the rows as they are"`
	Query neui.IslandCall[tablePage,tableResult] `json:"query,omitempty" description:"new function that returns the rows; replaces rows, and the table reloads from the first page"`
	Rows []tableRow `json:"rows,omitempty" maxItems:"100" description:"new fixed rows; replaces query, and the table shows them all with no pages"`
	PageSize int `json:"pageSize,omitempty" minimum:"1" maximum:"100" description:"new rows per page for query; the table goes back to the first page"`
}

type table struct {
	_ struct{} `additionalProperties:"false"`
	Columns []string `json:"columns" required:"true" nullable:"false" minItems:"1" maxItems:"12" items.maxLength:"80" description:"column headers, in order"`
	Query neui.IslandCall[tablePage,tableResult] `json:"query,omitempty" description:"returns the rows for a page, called when the table shows and on every page change; set either query or rows"`
	Rows []tableRow `json:"rows,omitempty" maxItems:"100" description:"fixed rows, all shown with no pages; set either query or rows"`
	PageSize int `json:"pageSize,omitempty" minimum:"1" maximum:"100" default:"20" description:"rows per page for query"`
	Clickable bool `json:"clickable,omitempty" description:"true makes each row's first cell clickable; a click submits the row id with no value"`
}

// NewTable builds a table widget for Library.Control. Its rows are either fixed
// (the rows param) or fetched a page at a time by the query param, with
// numbered pages when the query reports a total and previous and next
// otherwise. A click on a row's first cell (when clickable)
// or on one of its action buttons submits the row id and the button's value
// to the script waiting in ui.draw(). An empty patch reloads the rows.
func NewTable(p table, submit func(ctx context.Context, s tableSubmit) bool) api.Widget[tablePatch] {
	if p.PageSize == 0 {
		p.PageSize = 20
	}
	return tableWidget{
		params: p,
		state: doors.NewSourceEqual(tableState{query: p.Query, rows: p.Rows, size: p.PageSize, page: 1}, nil),
		submit: submit,
	}
}

type tableState struct {
	query neui.IslandCall[tablePage,tableResult]
	rows []tableRow
	size int
	page int
}

type tableWidget struct {
	params table
	state doors.Source[tableState]
	submit func(ctx context.Context, s tableSubmit) bool
}

func (t tableWidget) Update(ctx context.Context, patch tablePatch) {
	t.state.Mutate(ctx, func(s tableState) tableState {
		if patch.Query != "" {
			s.query, s.rows = patch.Query, nil
			s.page = 1
		}
		if patch.Rows != nil {
			s.query, s.rows = "", patch.Rows
		}
		if patch.PageSize != 0 {
			s.size = patch.PageSize
			s.page = 1
		}
		return s
	})
}

//line table.gox:116
func (t tableWidget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:117
		__e = __c.Any(t.state.Bind(t.render)); if __e != nil { return }
	return })
//line table.gox:118
}

//line table.gox:120
func (t tableWidget) render(s tableState) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line table.gox:122
		size := s.size
	page := s.page
	content := tableResult{Rows: s.rows}
	pages := 0
	failed := ""
	if (s.query == "") == (s.rows == nil) {
		failed = "Set either query or rows."
	} else if s.query != "" {
		var err error
		content, err = s.query.Call(ctx, tablePage{Offset: (page - 1) * size, Limit: size})
		if err == nil && content.Total != nil {
			pages = (*content.Total + size - 1) / size
			if page > max(pages, 1) {
				page = max(pages, 1)
				content, err = s.query.Call(ctx, tablePage{Offset: (page - 1) * size, Limit: size})
			}
		}
		if err != nil {
			failed = "Could not load the rows."
		}
		content.Rows = content.Rows[:min(len(content.Rows), size)]
	}
	actions := false
	for _, r := range content.Rows {
		if len(r.Actions) > 0 {
			actions = true
		}
		if failed == "" && len(r.Cells) != len(t.params.Columns) {
			failed = "Row " + r.ID + " has " + strconv.Itoa(len(r.Cells)) + " cells for " + strconv.Itoa(len(t.params.Columns)) + " columns."
		}
	}
	span := len(t.params.Columns)
	if actions {
		span++
	}
	first := max(1, min(page - 2, pages - 4))
	last := min(pages, first + 4)

		__e = __c.Init("div"); if __e != nil { return }
		{
//line table.gox:160
			__e = __c.Set("class", "kisland-table"); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
			__e = __c.Init("table"); if __e != nil { return }
			{
//line table.gox:161
				__e = __c.Set("class", "kisland-table-grid"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
				__e = __c.Init("thead"); if __e != nil { return }
				{
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("tr"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
//line table.gox:164
						for _, c := range t.params.Columns {
							__e = __c.Init("th"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line table.gox:165
								__e = __c.Any(c); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
//line table.gox:167
						if actions {
							__e = __c.Init("th"); if __e != nil { return }
							{
//line table.gox:168
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
//line table.gox:173
					if failed != "" {
						__e = __c.Init("tr"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line table.gox:174
								__e = __c.Set("class", "kisland-table-error"); if __e != nil { return }
//line table.gox:174
								__e = __c.Set("colspan", span); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line table.gox:174
								__e = __c.Any(failed); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
//line table.gox:175
					} else if len(content.Rows) == 0 {
						__e = __c.Init("tr"); if __e != nil { return }
						{
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("td"); if __e != nil { return }
							{
//line table.gox:176
								__e = __c.Set("class", "kisland-table-empty"); if __e != nil { return }
//line table.gox:176
								__e = __c.Set("colspan", span); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
								__e = __c.Text("No rows"); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
						__e = __c.Close(); if __e != nil { return }
					} else  {
//line table.gox:178
						for _, r := range content.Rows {
							__e = __c.Init("tr"); if __e != nil { return }
							{
								__e = __c.Submit(); if __e != nil { return }
//line table.gox:180
								for i, c := range r.Cells {
									__e = __c.Init("td"); if __e != nil { return }
									{
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:182
										if i == 0 && t.params.Clickable {
											__e = __c.Init("button"); if __e != nil { return }
											{
//line table.gox:184
												__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:185
												__e = __c.Set("class", "kisland-table-link"); if __e != nil { return }
//line table.gox:186
												__e = __c.Modify(doors.AClick{
												Scope: neui.IslandScope(ctx),
												On: func(ctx context.Context, _ doors.RequestPointer) bool {
													t.submit(ctx, tableSubmit{ID: r.ID})
													return false
												},
											}); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
//line table.gox:193
												__e = __c.Any(string(c)); if __e != nil { return }
											}
											__e = __c.Close(); if __e != nil { return }
										} else  {
//line table.gox:196
											__e = __c.Any(string(c)); if __e != nil { return }
										}
									}
									__e = __c.Close(); if __e != nil { return }
								}
//line table.gox:200
								if actions {
									__e = __c.Init("td"); if __e != nil { return }
									{
//line table.gox:201
										__e = __c.Set("class", "kisland-table-actions"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line table.gox:202
										for _, a := range r.Actions {
											__e = __c.Init("button"); if __e != nil { return }
											{
//line table.gox:204
												__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:205
												__e = __c.Set("class", "kisland-button kisland-button-sm kisland-button-" + cmp.Or(a.Style, "secondary")); if __e != nil { return }
//line table.gox:206
												__e = __c.Modify(doors.AClick{
												Scope: neui.IslandScope(ctx),
												On: func(ctx context.Context, _ doors.RequestPointer) bool {
													t.submit(ctx, tableSubmit{ID: r.ID, Value: a.Value})
													return false
												},
											}); if __e != nil { return }
												__e = __c.Submit(); if __e != nil { return }
//line table.gox:213
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
					}
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
//line table.gox:223
			if s.query == "" {
//line table.gox:224
			} else if content.Total != nil && pages > 1 {
				__e = __c.Init("nav"); if __e != nil { return }
				{
//line table.gox:225
					__e = __c.Set("class", "kisland-table-pager"); if __e != nil { return }
//line table.gox:225
					__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:226
					__e = __c.Any(t.pageButton("First", 1, page > 1)); if __e != nil { return }
//line table.gox:227
					__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:228
					for n := first; n <= last; n++ {
//line table.gox:229
						if n == page {
							__e = __c.Init("span"); if __e != nil { return }
							{
//line table.gox:230
								__e = __c.Set("class", "kisland-table-current"); if __e != nil { return }
//line table.gox:230
								__e = __c.Set("aria-current", "page"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line table.gox:230
								__e = __c.Any(n); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						} else  {
//line table.gox:232
							__e = __c.Any(t.pageButton(strconv.Itoa(n), n, true)); if __e != nil { return }
						}
					}
//line table.gox:235
					__e = __c.Any(t.pageButton("Next", page + 1, page < pages)); if __e != nil { return }
//line table.gox:236
					__e = __c.Any(t.pageButton("Last", pages, page < pages)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line table.gox:238
			} else if content.Total == nil && (page > 1 || len(content.Rows) == size) {
				__e = __c.Init("nav"); if __e != nil { return }
				{
//line table.gox:239
					__e = __c.Set("class", "kisland-table-pager"); if __e != nil { return }
//line table.gox:239
					__e = __c.Set("aria-label", "Pagination"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
//line table.gox:240
					__e = __c.Any(t.pageButton("Prev", page - 1, page > 1)); if __e != nil { return }
//line table.gox:241
					__e = __c.Any(t.pageButton("Next", page + 1, len(content.Rows) == size)); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:245
}

//line table.gox:247
func (t tableWidget) pageButton(label string, page int, enabled bool) gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
		__e = __c.Init("button"); if __e != nil { return }
		{
//line table.gox:249
			__e = __c.Set("type", "button"); if __e != nil { return }
//line table.gox:250
			__e = __c.Set("class", "kisland-button kisland-button-sm kisland-button-secondary"); if __e != nil { return }
//line table.gox:251
			__e = __c.Set("disabled", !enabled); if __e != nil { return }
//line table.gox:252
			__e = __c.Modify(doors.AClick{
			Scope: neui.IslandScope(ctx),
			On: func(ctx context.Context, _ doors.RequestPointer) bool {
				t.state.Mutate(ctx, func(s tableState) tableState {
					s.page = page
					return s
				})
				return false
			},
		}); if __e != nil { return }
			__e = __c.Submit(); if __e != nil { return }
//line table.gox:262
			__e = __c.Any(label); if __e != nil { return }
		}
		__e = __c.Close(); if __e != nil { return }
	return })
//line table.gox:264
}
