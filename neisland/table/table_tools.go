package table

import (
	"cmp"
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"

	"github.com/doors-dev/gox"
	"neui.dev"
)

type pageView struct {
	Columns  []column  `json:"columns" required:"true" nullable:"false" description:"the columns, in order"`
	Rows     []rowView `json:"rows" required:"true" nullable:"false" description:"the rows of the current page, in order"`
	Page     int       `json:"page" required:"true" description:"number of the current page, from 1"`
	Pages    *int      `json:"pages,omitempty" description:"number of pages; absent when the row count is unknown"`
	Sort     *sortSpec `json:"sort,omitempty" description:"the current sort; absent when unsorted"`
	Search   string    `json:"search,omitempty" description:"the current search text; absent when there is none"`
	Selected int       `json:"selected,omitempty" description:"how many rows are selected, on every page; absent when none"`
}

type rowView struct {
	ID       string       `json:"id" required:"true" description:"the row's id"`
	Cells    []cellText   `json:"cells" required:"true" nullable:"false" description:"one markdown text per column, in column order"`
	Actions  []actionView `json:"actions,omitempty" description:"the row's action buttons"`
	Selected bool         `json:"selected,omitempty" description:"true when the row is selected"`
}

type actionView struct {
	Text  string `json:"text" required:"true" description:"label shown on the button"`
	Value string `json:"value" required:"true" description:"the action's value"`
	Style string `json:"style" required:"true" description:"visual weight, as the user sees it"`
}

type pageArg struct {
	_    struct{} `additionalProperties:"false"`
	Page int      `json:"page" required:"true" minimum:"1" description:"page to show, counting from 1; past page 1, only a page that has rows"`
}

type searchArg struct {
	_    struct{} `additionalProperties:"false"`
	Text string   `json:"text" required:"true" maxLength:"100" description:"text to search for; empty clears the search"`
}

type selectArg struct {
	_        struct{} `additionalProperties:"false"`
	IDs      []string `json:"ids" required:"true" nullable:"false" minItems:"1" maxItems:"100" uniqueItems:"true" items.maxLength:"200" description:"ids of rows on the current page"`
	Selected bool     `json:"selected" required:"true" description:"true selects the rows, false unselects them"`
}

type selectedIDs struct {
	IDs []string `json:"ids" required:"true" nullable:"false" description:"ids of the selected rows, sorted"`
}

type rowArg struct {
	_  struct{} `additionalProperties:"false"`
	ID string   `json:"id" required:"true" maxLength:"200" description:"id of a row on the current page"`
}

type actionArg struct {
	_     struct{} `additionalProperties:"false"`
	ID    string   `json:"id" required:"true" maxLength:"200" description:"id of a row on the current page"`
	Value string   `json:"value" required:"true" maxLength:"1000" description:"value of one of the row's actions"`
}

var errNothingSelected = errors.New(nothingSelectedMessage)
var errNoDraw = errors.New(noDrawMessage)

func (t widget) tools() []gox.Comp {
	tools := []gox.Comp{
		neui.Tool[struct{}, pageView]{Name: getPageName, Desc: getPageDesc, Handler: t.getPage},
		neui.Tool[pageArg, struct{}]{Name: setPageName, Desc: setPageDesc, Handler: t.setPage},
		neui.Tool[actionArg, struct{}]{Name: clickActionName, Desc: clickActionDesc, Handler: t.clickAction},
	}
	if t.params.Search {
		tools = append(tools, neui.Tool[searchArg, struct{}]{Name: setSearchName, Desc: setSearchDesc, Handler: t.setSearch})
	}
	if slices.ContainsFunc(t.params.Columns, func(c column) bool { return c.Sort }) {
		tools = append(tools, neui.Tool[sortSpec, struct{}]{Name: setSortName, Desc: setSortDesc, Handler: t.setSort})
	}
	if t.params.Select != nil {
		tools = append(tools,
			neui.Tool[selectArg, struct{}]{Name: setSelectedName, Desc: setSelectedDesc, Handler: t.setSelected},
			neui.Tool[struct{}, selectedIDs]{Name: getSelectedName, Desc: getSelectedDesc, Handler: t.getSelected},
			neui.Tool[struct{}, struct{}]{Name: submitSelectedName, Desc: submitSelectedDesc, Handler: t.submitSelected},
			neui.Tool[struct{}, struct{}]{Name: clearSelectedName, Desc: clearSelectedDesc, Handler: t.clearSelected},
		)
	}
	if t.params.Clickable {
		tools = append(tools, neui.Tool[rowArg, struct{}]{Name: clickRowName, Desc: clickRowDesc, Handler: t.clickRow})
	}
	return tools
}

func load(ctx context.Context, s state) (result, error) {
	content, err := s.data(ctx)
	if err != nil {
		return result{}, fmt.Errorf(rowsFailedMessage, err)
	}
	return content, nil
}

func (t widget) view(s state, content result) pageView {
	v := pageView{
		Columns: t.params.Columns,
		Rows:    make([]rowView, len(content.Rows)),
		Page:    s.page,
		Sort:    s.sort,
		Search:  s.search,
	}
	if content.Total != nil {
		pages := max(1, (*content.Total+s.size-1)/s.size)
		v.Pages = &pages
	}
	if t.params.Select != nil {
		v.Selected = len(s.selected)
	}
	for i, r := range content.Rows {
		cells := make([]cellText, len(t.params.Columns))
		copy(cells, r.Cells)
		v.Rows[i] = rowView{ID: r.ID, Cells: cells}
		for _, a := range r.Actions {
			v.Rows[i].Actions = append(v.Rows[i].Actions, actionView{Text: a.Text, Value: a.Value, Style: cmp.Or(a.Style, "secondary")})
		}
		if t.params.Select != nil {
			_, v.Rows[i].Selected = s.selected[r.ID]
		}
	}
	return v
}

func (t widget) getPage(ctx context.Context, _ struct{}) (pageView, error) {
	current, ok := t.state.Read(ctx)
	if !ok {
		return pageView{}, context.Canceled
	}
	content, err := load(ctx, current)
	if err != nil {
		return pageView{}, err
	}
	return t.view(current, content), nil
}

func (t widget) setPage(ctx context.Context, in pageArg) (struct{}, error) {
	target, ok := t.state.Read(ctx)
	if !ok {
		return struct{}{}, context.Canceled
	}
	target.page = in.Page
	content, err := load(ctx, target)
	if err != nil {
		return struct{}{}, err
	}
	if in.Page > 1 && content.Total != nil {
		if last := max(1, (*content.Total+target.size-1)/target.size); in.Page > last {
			return struct{}{}, fmt.Errorf(pastLastPageMessage, in.Page, last)
		}
	}
	if in.Page > 1 && content.Total == nil && len(content.Rows) == 0 {
		return struct{}{}, fmt.Errorf(emptyPageMessage, in.Page)
	}
	t.showPage(ctx, in.Page)
	return struct{}{}, nil
}

func (t widget) setSearch(ctx context.Context, in searchArg) (struct{}, error) {
	t.search(ctx, in.Text, true)
	return struct{}{}, nil
}

func (t widget) setSort(ctx context.Context, in sortSpec) (struct{}, error) {
	if in.Column < 0 || in.Column >= len(t.params.Columns) || !t.params.Columns[in.Column].Sort {
		return struct{}{}, fmt.Errorf(notSortableMessage, in.Column)
	}
	t.sortBy(ctx, in.Column, in.Desc)
	return struct{}{}, nil
}

func (t widget) setSelected(ctx context.Context, in selectArg) (struct{}, error) {
	current, ok := t.state.Read(ctx)
	if !ok {
		return struct{}{}, context.Canceled
	}
	content, err := load(ctx, current)
	if err != nil {
		return struct{}{}, err
	}
	added := 0
	for _, id := range in.IDs {
		if !slices.ContainsFunc(content.Rows, func(r row) bool { return r.ID == id }) {
			return struct{}{}, fmt.Errorf(notOnPageMessage, id)
		}
		if _, ok := current.selected[id]; in.Selected && !ok {
			added++
		}
	}
	if len(current.selected)+added > 1000 {
		return struct{}{}, fmt.Errorf(selectionFullMessage, len(current.selected))
	}
	t.mark(ctx, in.IDs, in.Selected)
	return struct{}{}, nil
}

func (t widget) getSelected(ctx context.Context, _ struct{}) (selectedIDs, error) {
	selected, ok := t.selected.Read(ctx)
	if !ok {
		return selectedIDs{}, context.Canceled
	}
	ids := slices.Sorted(maps.Keys(selected))
	if ids == nil {
		ids = []string{}
	}
	return selectedIDs{IDs: ids}, nil
}

func (t widget) submitSelected(ctx context.Context, _ struct{}) (struct{}, error) {
	selected, ok := t.selected.Read(ctx)
	if !ok {
		return struct{}{}, context.Canceled
	}
	if len(selected) == 0 {
		return struct{}{}, errNothingSelected
	}
	if !t.submit(ctx, Submit{IDs: slices.Sorted(maps.Keys(selected))}) {
		return struct{}{}, errNoDraw
	}
	return struct{}{}, nil
}

func (t widget) clearSelected(ctx context.Context, _ struct{}) (struct{}, error) {
	t.selected.Update(ctx, nil)
	return struct{}{}, nil
}

func (t widget) clickRow(ctx context.Context, in rowArg) (struct{}, error) {
	current, ok := t.state.Read(ctx)
	if !ok {
		return struct{}{}, context.Canceled
	}
	content, err := load(ctx, current)
	if err != nil {
		return struct{}{}, err
	}
	if !slices.ContainsFunc(content.Rows, func(r row) bool { return r.ID == in.ID }) {
		return struct{}{}, fmt.Errorf(notOnPageMessage, in.ID)
	}
	if !t.submit(ctx, Submit{ID: in.ID}) {
		return struct{}{}, errNoDraw
	}
	return struct{}{}, nil
}

func (t widget) clickAction(ctx context.Context, in actionArg) (struct{}, error) {
	current, ok := t.state.Read(ctx)
	if !ok {
		return struct{}{}, context.Canceled
	}
	content, err := load(ctx, current)
	if err != nil {
		return struct{}{}, err
	}
	onPage, found := false, false
	for _, r := range content.Rows {
		if r.ID == in.ID {
			onPage = true
			found = found || slices.ContainsFunc(r.Actions, func(a cellButton) bool { return a.Value == in.Value })
		}
	}
	if !onPage {
		return struct{}{}, fmt.Errorf(notOnPageMessage, in.ID)
	}
	if !found {
		return struct{}{}, fmt.Errorf(noActionMessage, in.ID, in.Value)
	}
	if !t.submit(ctx, Submit{ID: in.ID, Value: in.Value}) {
		return struct{}{}, errNoDraw
	}
	return struct{}{}, nil
}
