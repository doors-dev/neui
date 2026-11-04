package xls

import (
	"context"

	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

const moduleDesc = `Excel workbooks, .xlsx only, workspace paths only — copy an uploaded workbook into the workspace first (files__copy), and share a finished report by copying it to agent_uploads/<name> and linking it. Read through the funnel: sheets maps the workbook, profile shows a sheet's headers, column types and row count, read pages values out — there is no whole-sheet dump. Build reports through the write tools: table lays data out as a styled table, column appends computed columns, total adds a summary row, chart, format and highlight decorate the result; find locates cells by substring when profile is not enough. Keep formulas to common functions (SUM, AVERAGE, COUNT, IF, SUMIFS). Run check before sharing anything with formulas.`

func Module(c Config) common.ModuleFactory {
	c.initDefaults()
	return func(context.Context) common.Module {
		m := tools.NewModule("xls", moduleDesc)
		add(m, c, "sheets", sheetsDesc, sheetsTool)
		add(m, c, "profile", profileDesc, profileTool)
		add(m, c, "read", readDesc, readTool)
		add(m, c, "find", findDesc, findTool)
		add(m, c, "table", tableDesc, tableTool)
		add(m, c, "chart", chartDesc, chartTool)
		add(m, c, "column", columnDesc, columnTool)
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
