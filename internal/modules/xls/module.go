package xls

import (
	"context"

	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const moduleDesc = `Excel workbooks, .xlsx only, workspace paths only — copy an uploaded workbook into the workspace first (files__copy), and share a finished report by copying it to agent_uploads/<name> and linking it. Read through the funnel: sheets maps the workbook, profile shows a sheet's headers, column types and row count, read pages values out — there is no whole-sheet dump; find locates cells by substring when profile is not enough. Build reports through the write tools: table lays data out as a new styled table, write puts values or formulas into empty cells, total adds a summary row under a plain range, chart, format and highlight decorate the result. Conventions: a data range (profile, read, total, chart) includes its header row, while format and highlight take the exact cells to style; columns are named by header, a letter like C also works; sheet references in results are quoted, like 'Data'!D4. Writes go only into empty cells and never change an existing table, autofilter, frozen pane or column width. Not supported, and refused: resizing a table or adding a total row to one, changing an autofilter, inserting or deleting rows or columns, sorting, pivots, data validation, VBA macros, links to other workbooks and dynamic-array functions (FILTER, SORT, UNIQUE, XLOOKUP and the like). Prefer formulas over computed values and keep to common functions (SUM, AVERAGE, COUNTA, IF, SUMIFS, COUNTIFS). For bulk or looping work, such as many blocks or one write per group, run a js script that calls xls__write instead of many separate calls. Run check before sharing anything with formulas.`

func Module(c Config) common.ModuleFactory {
	c.initDefaults()
	return func(context.Context) common.Module {
		m := tools.NewModule("xls", moduleDesc)
		add(m, c, "sheets", sheetsDesc, sheetsTool)
		add(m, c, "profile", profileDesc, profileTool)
		add(m, c, "read", readDesc, readTool)
		add(m, c, "find", findDesc, findTool)
		add(m, c, "table", tableDesc, tableTool)
		add(m, c, "write", writeDesc, writeTool)
		add(m, c, "chart", chartDesc, chartTool)
		add(m, c, "total", totalDesc, totalTool)
		add(m, c, "format", formatDesc, formatTool)
		add(m, c, "highlight", highlightDesc, highlightTool)
		add(m, c, "check", checkDesc, checkTool)
		return m
	}
}

func add[I any, O any](m tools.Module, c Config, name string, desc string, f func(ctx context.Context, input I) (O, error)) {
	scoped := func(ctx context.Context, input I) (O, error) {
		return f(context.WithValue(ctx, configKey{}, c), input)
	}
	if _, err := m.Add(name, desc, scoped); err != nil {
		panic(err)
	}
}
