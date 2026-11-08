package ui

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
)

type tableState struct {
	query    neui.IslandCall[tablePage, tableResult]
	rows     []tableRow
	size     int
	page     int
	sort     *tableSort
	search   string
	seq      int
	selected map[string]struct{}
}

func (t *tableState) isDataSource() bool {
	return t.rows != nil
}

func (t *tableState) isQuerySource() bool {
	return t.query != ""
}

func (t *tableState) data(ctx context.Context) (tableResult, error) {
	switch {
	case t.isDataSource():
		return t.rowsData(), nil
	case t.isQuerySource():
		return t.queryData(ctx)
	default:
		return tableResult{}, errors.New("uknown source")
	}
}

func (t *tableState) rowsData() tableResult {
	needle := strings.ToLower(t.search)
	rows := slices.DeleteFunc(slices.Clone(t.rows), func(r tableRow) bool {
		return !slices.ContainsFunc(r.Cells, func(c cellText) bool {
			return strings.Contains(strings.ToLower(string(c)), needle)
		})
	})
	if t.sort == nil {
		return tableResult{Rows: rows}
	}
	col := t.sort.Column
	slices.SortStableFunc(rows, func(x tableRow, y tableRow) int {
		if col >= len(x.Cells) || col >= len(y.Cells) {
			return 0
		}
		a, b := strings.ToLower(string(x.Cells[col])), strings.ToLower(string(y.Cells[col]))
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
	return tableResult{Rows: rows}
}

func (t *tableState) queryData(ctx context.Context) (tableResult, error) {
	result, err := t.query.Call(ctx, tablePage{Offset: (t.page - 1) * t.size, Limit: t.size, Sort: t.sort, Search: t.search})
	if err != nil {
		return tableResult{}, err
	}
	result.Rows = result.Rows[:min(len(result.Rows), t.size)]
	return result, nil
}

func tableStateEqual(old tableState, new tableState) bool {
	if old.seq != new.seq {
		return false
	}
	if !maps.Equal(old.selected, new.selected) {
		return false
	}
	return true
}

type table struct {
	_          struct{}                                `additionalProperties:"false"`
	Columns    []tableColumn                           `json:"columns" required:"true" nullable:"false" minItems:"1" maxItems:"12" description:"column headers, in order"`
	Query      neui.IslandCall[tablePage, tableResult] `json:"query,omitempty" description:"returns the rows for a page, called when the table shows and on every page change; set either query or rows"`
	Rows       []tableRow                              `json:"rows,omitempty" maxItems:"100" description:"fixed rows, all shown with no pages; set either query or rows; the table sorts and searches them itself"`
	Sort       *tableSort                              `json:"sort,omitempty" description:"sort when the table shows; ignored when the column is out of range or not sortable"`
	Search     bool                                    `json:"search,omitempty" description:"true adds a search input above the table; with query its text goes to every query call, with rows the table shows only rows with a cell containing it, case-insensitive"`
	PageSize   int                                     `json:"pageSize,omitempty" minimum:"1" maximum:"100" default:"20" description:"rows per page for query"`
	Clickable  bool                                    `json:"clickable,omitempty" description:"true makes each row's first cell clickable; a click submits the row id with no value"`
	Selectable bool                                    `json:"selectable,omitempty" description:"true adds a checkbox to each row and, while rows are selected, a bar with the count, a clear control and a button that submits the selected ids; the selection survives page changes and reloads"`
	Selected   []string                                `json:"selected,omitempty" maxItems:"1000" uniqueItems:"true" items.maxLength:"200" description:"ids of the rows selected when the table shows"`
}

func (p *table) initDefault() {
	if p.PageSize == 0 {
		p.PageSize = 20
	}
	if p.Sort != nil && (p.Sort.Column < 0 || p.Sort.Column >= len(p.Columns) || !p.Columns[p.Sort.Column].Sort) {
		p.Sort = nil
	}
}

func (p *table) state() tableState {
	selected := make(map[string]struct{}, len(p.Selected))
	for _, id := range p.Selected {
		selected[id] = struct{}{}
	}
	return tableState{
		query:    p.Query,
		rows:     p.Rows,
		size:     p.PageSize,
		page:     1,
		sort:     p.Sort,
		selected: selected,
	}
}

type tableColumn struct {
	_     struct{} `additionalProperties:"false"`
	Title string   `json:"title" required:"true" minLength:"1" maxLength:"80" description:"header text"`
	Sort  bool     `json:"sort,omitempty" description:"true makes the header a control that sorts by this column: ascending first, then flipping on each click"`
}

type tablePage struct {
	_      struct{}   `additionalProperties:"false"`
	Offset int        `json:"offset" required:"true" minimum:"0" description:"index of the first row to return"`
	Limit  int        `json:"limit" required:"true" minimum:"1" description:"how many rows to return at most"`
	Sort   *tableSort `json:"sort,omitempty" description:"column and direction to sort the rows by before paging; omitted means the query's own order"`
	Search string     `json:"search,omitempty" description:"text the user typed in the search input; return only rows that match it; omitted when empty"`
}

type tableSort struct {
	_      struct{} `additionalProperties:"false"`
	Column int      `json:"column" required:"true" minimum:"0" description:"0-based index of the column to sort by"`
	Desc   bool     `json:"desc,omitempty" description:"true sorts descending, otherwise ascending"`
}

type tableResult struct {
	_     struct{}   `additionalProperties:"false"`
	Rows  []tableRow `json:"rows" required:"true" nullable:"false" maxItems:"100" description:"rows from offset on, at most limit of them"`
	Total *int       `json:"total,omitempty" minimum:"0" description:"number of rows in the whole table; omit when unknown, and the table offers only previous and next"`
}

type tableRow struct {
	_       struct{}     `additionalProperties:"false"`
	ID      string       `json:"id" required:"true" minLength:"1" maxLength:"200" description:"identifies the row in what a click submits"`
	Cells   []cellText   `json:"cells" required:"true" nullable:"false" description:"one text per column, in column order"`
	Actions []cellButton `json:"actions,omitempty" maxItems:"5" description:"buttons shown at the end of the row"`
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

type tableSubmit struct {
	ID    string   `json:"id,omitempty" description:"id of the row the user acted on: alone for a click on the row's first cell, with value for a row action; empty for a selection submit"`
	Value string   `json:"value,omitempty" description:"value of the row action button clicked; empty for a first-cell click and a selection submit"`
	IDs   []string `json:"ids,omitempty" maxItems:"1000" description:"ids of the selected rows in the order they were selected; set only when the selection button was clicked"`
}

type tablePatch struct {
	_        struct{}                                `additionalProperties:"false" description:"an empty patch reloads the rows: the current page of query, or the rows as they are"`
	Query    neui.IslandCall[tablePage, tableResult] `json:"query,omitempty" description:"new function that returns the rows; replaces rows, and the table reloads from the first page with the sort and search kept"`
	Rows     []tableRow                              `json:"rows,omitempty" maxItems:"100" description:"new fixed rows; replaces query, and the table shows them all with no pages"`
	Sort     *tableSort                              `json:"sort,omitempty" description:"new sort; ignored when the column is out of range or not sortable; the table goes back to the first page and keeps the selection"`
	PageSize int                                     `json:"pageSize,omitempty" minimum:"1" maximum:"100" description:"new rows per page for query; the table goes back to the first page"`
	Selected []string                                `json:"selected,omitempty" maxItems:"1000" uniqueItems:"true" items.maxLength:"200" description:"new selection by row id: omit to keep it, [] clears it, a list replaces it; otherwise a query patch clears the selection, a rows patch keeps only ids still in the rows, and pageSize keeps it"`
}
