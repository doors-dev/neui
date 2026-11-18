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

package table

import (
	"cmp"
	"context"
	"errors"
	"maps"
	"slices"
	"strings"
	"unicode/utf8"

	"github.com/swaggest/jsonschema-go"
	"neui.dev"
	"neui.dev/internal/markdown"
)

type state struct {
	columns   []column
	query     neui.IslandCall[page, result]
	rows      []row
	size      int
	page      int
	sort      *sortSpec
	search    string
	searchSeq int
	seq       int
	selected  map[string]struct{}
}

func (t *state) sortable(column int) bool {
	return column >= 0 && column < len(t.columns) && t.columns[column].Sort
}

func (t *state) isDataSource() bool {
	return t.rows != nil
}

func (t *state) isQuerySource() bool {
	return t.query != ""
}

func (t *state) data(ctx context.Context) (result, error) {
	switch {
	case t.isDataSource():
		return t.rowsData(), nil
	case t.isQuerySource():
		return t.queryData(ctx)
	default:
		return result{}, errors.New("unknown source")
	}
}

func (t *state) rowsData() result {
	rows := slices.Clone(t.rows)
	if t.search != "" {
		needle := strings.ToLower(t.search)
		rows = slices.DeleteFunc(rows, func(r row) bool {
			return !slices.ContainsFunc(r.plain, func(c string) bool {
				return strings.Contains(strings.ToLower(c), needle)
			})
		})
	}
	if t.sort != nil {
		t.sortRows(rows)
	}
	total := len(rows)
	start := min((t.page-1)*t.size, total)
	return result{Rows: rows[start:min(start+t.size, total)], Total: &total}
}

func (t *state) sortRows(rows []row) {
	col := t.sort.Column
	slices.SortStableFunc(rows, func(x row, y row) int {
		a, b := "", ""
		if col < len(x.plain) {
			a = strings.ToLower(x.plain[col])
		}
		if col < len(y.plain) {
			b = strings.ToLower(y.plain[col])
		}
		order := 0
		for order == 0 && a != "" && b != "" {
			da, db := len(a)-len(strings.TrimLeft(a, "0123456789")), len(b)-len(strings.TrimLeft(b, "0123456789"))
			if da > 0 && db > 0 {
				na, nb := strings.TrimLeft(a[:da], "0"), strings.TrimLeft(b[:db], "0")
				order = cmp.Or(cmp.Compare(len(na), len(nb)), strings.Compare(na, nb))
				a, b = a[da:], b[db:]
				continue
			}
			ra, sa := utf8.DecodeRuneInString(a)
			rb, sb := utf8.DecodeRuneInString(b)
			order = cmp.Compare(ra, rb)
			a, b = a[sa:], b[sb:]
		}
		order = cmp.Or(order, cmp.Compare(len(a), len(b)))
		if t.sort.Desc {
			return -order
		}
		return order
	})
}

func (t *state) queryData(ctx context.Context) (result, error) {
	res, err := t.query.Call(ctx, page{Offset: (t.page - 1) * t.size, Limit: t.size, Sort: t.sort, Search: t.search})
	if err != nil {
		return result{}, err
	}
	res.Rows = res.Rows[:min(len(res.Rows), t.size)]
	return res, nil
}

func stateEqual(old state, new state) bool {
	if old.seq != new.seq || old.searchSeq != new.searchSeq {
		return false
	}
	if !maps.Equal(old.selected, new.selected) {
		return false
	}
	return true
}

// Params is what a script passes to ui.table.
type Params struct {
	_         struct{}                      `additionalProperties:"false"`
	Columns   []column                      `json:"columns" required:"true" nullable:"false" minItems:"1" maxItems:"12" description:"column headers, in order"`
	Query     neui.IslandCall[page, result] `json:"query,omitempty" description:"returns the rows for a page, called whenever the table needs rows"`
	Rows      []row                         `json:"rows,omitempty" maxItems:"100" description:"fixed rows; the table pages, sorts and searches them itself"`
	Sort      *sortSpec                     `json:"sort,omitempty" description:"sort when the table shows; ignored when the column is out of range or not sortable"`
	Search    bool                          `json:"search,omitempty" description:"true lets the user search; with rows, only rows with a cell containing the text show, case-insensitive"`
	PageSize  int                           `json:"pageSize,omitempty" minimum:"1" maximum:"100" default:"20" description:"rows per page"`
	Clickable bool                          `json:"clickable,omitempty" description:"true makes each row's first cell clickable: the way to let the user pick one row"`
	Selected  []string                      `json:"selected,omitempty" maxItems:"1000" uniqueItems:"true" items.maxLength:"200" description:"ids of the rows selected when the table shows"`
	Select    *selection                    `json:"select,omitempty" description:"lets the user select several rows and submit them together with this button; the selection survives page changes and reloads"`
}

type selection struct {
	_     struct{} `additionalProperties:"false"`
	Text  string   `json:"text,omitempty" minLength:"1" maxLength:"80" default:"Submit" description:"label of the button"`
	Style string   `json:"style,omitempty" enum:"primary,secondary,success,danger" default:"primary" description:"visual weight"`
}

func (p *Params) initDefault() {
	if p.PageSize == 0 {
		p.PageSize = 20
	}
	if p.Select != nil && p.Select.Text == "" {
		p.Select.Text = "Submit"
	}
	if p.Select != nil && p.Select.Style == "" {
		p.Select.Style = "primary"
	}
	if p.Sort != nil && (p.Sort.Column < 0 || p.Sort.Column >= len(p.Columns) || !p.Columns[p.Sort.Column].Sort) {
		p.Sort = nil
	}
}

func (p *Params) state() state {
	selected := make(map[string]struct{}, len(p.Selected))
	for _, id := range p.Selected {
		selected[id] = struct{}{}
	}
	return state{
		columns:  p.Columns,
		query:    p.Query,
		rows:     rowsPlain(p.Rows),
		size:     p.PageSize,
		page:     1,
		sort:     p.Sort,
		selected: selected,
	}
}

type column struct {
	_     struct{} `additionalProperties:"false"`
	Title string   `json:"title" required:"true" minLength:"1" maxLength:"80" description:"header text"`
	Sort  bool     `json:"sort,omitempty" description:"true lets the user sort by this column"`
}

type page struct {
	_      struct{}  `additionalProperties:"false"`
	Offset int       `json:"offset" required:"true" minimum:"0" description:"index of the first row to return"`
	Limit  int       `json:"limit" required:"true" minimum:"1" description:"how many rows to return at most"`
	Sort   *sortSpec `json:"sort,omitempty" description:"column and direction to sort the rows by before paging; omitted means the query's own order"`
	Search string    `json:"search,omitempty" description:"text the user typed in the search input; return only rows that match it; omitted when empty"`
}

type sortSpec struct {
	_      struct{} `additionalProperties:"false"`
	Column int      `json:"column" required:"true" minimum:"0" description:"0-based index of the column to sort by"`
	Desc   bool     `json:"desc,omitempty" description:"true sorts descending, otherwise ascending"`
}

type result struct {
	_     struct{} `additionalProperties:"false"`
	Rows  []row    `json:"rows" required:"true" nullable:"false" maxItems:"100" description:"rows from offset on, at most limit of them"`
	Total *int     `json:"total,omitempty" minimum:"0" description:"number of rows in the whole table; omit when unknown"`
}

type row struct {
	_       struct{}     `additionalProperties:"false"`
	ID      string       `json:"id" required:"true" minLength:"1" maxLength:"200" description:"identifies the row in what a click submits"`
	Cells   []cellText   `json:"cells" required:"true" nullable:"false" description:"one text per column, in column order; inline markdown: **bold**, *italic*, ~~strike~~, code in backquotes, [links](https://…) and [name](file:///path) downloads of workspace files, path relative to the workspace root; search and sort use the text as shown, numbers in it by value; for another order, such as priorities, use query"`
	Actions []cellButton `json:"actions,omitempty" maxItems:"5" description:"action buttons of the row"`
	plain   []string
}

func rowsPlain(rows []row) []row {
	for i := range rows {
		rows[i].plain = make([]string, len(rows[i].Cells))
		for j, c := range rows[i].Cells {
			rows[i].plain[j] = markdown.Plain(markdown.Inline(string(c)))
		}
	}
	return rows
}

type cellText string

func (cellText) PrepareJSONSchema(s *jsonschema.Schema) error {
	s.WithMaxLength(500)
	return nil
}

type cellButton struct {
	_     struct{} `additionalProperties:"false"`
	Value string   `json:"value" required:"true" minLength:"1" maxLength:"1000" description:"data the click submits with the row id"`
	Style string   `json:"style,omitempty" enum:"primary,secondary,success,danger" default:"secondary" description:"visual weight: primary for the main action, danger for destructive ones"`
	Text  string   `json:"text" required:"true" minLength:"1" maxLength:"80" description:"label shown on the button"`
}

// Submit is what a table hands the script waiting in ui.draw().
type Submit struct {
	Row   string   `json:"row,omitempty" description:"id of the row the user acted on: alone for a click on the row's first cell, with value for a row action; empty for a selection submit"`
	Value string   `json:"value,omitempty" description:"value of the row action button clicked; empty for a first-cell click and a selection submit"`
	Rows  []string `json:"rows,omitempty" maxItems:"1000" description:"ids of the selected rows, sorted; set only for a selection submit"`
}

// Patch is what a script passes to update a table.
type Patch struct {
	_        struct{}                      `additionalProperties:"false" description:"{} reloads the rows: the current page of query, or the rows as they are"`
	Columns  []column                      `json:"columns,omitempty" minItems:"1" maxItems:"12" description:"new column headers, in order; the sort stays when its column is still sortable"`
	Query    neui.IslandCall[page, result] `json:"query,omitempty" description:"new function that returns the rows; replaces rows, and the table reloads from the first page with the sort and search kept"`
	Rows     []row                         `json:"rows,omitempty" maxItems:"100" description:"new fixed rows; replaces query, and the table goes back to the first page"`
	Sort     *sortSpec                     `json:"sort,omitempty" description:"new sort; ignored when the column is out of range or not sortable; the table goes back to the first page and keeps the selection"`
	PageSize int                           `json:"pageSize,omitempty" minimum:"1" maximum:"100" description:"new rows per page; the table goes back to the first page"`
	Selected []string                      `json:"selected,omitempty" maxItems:"1000" uniqueItems:"true" items.maxLength:"200" description:"new selection by row id: omit to keep it, [] clears it, a list replaces it; otherwise a query patch clears the selection, a rows patch keeps only ids still in the rows, and pageSize keeps it"`
}
