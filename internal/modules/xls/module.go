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

package xls

import (
	"context"

	"neui.dev/internal/common"
	"neui.dev/internal/tools"
)

func moduleDesc(light bool) string {
	if light {
		return `Excel workbooks (.xlsx) at workspace paths; a .csv file is plain text, read it as text. Copy an uploaded workbook into the workspace before using it, and copy a finished one to agent_uploads/<name> to link it for the user. To read a workbook: sheets, then read; find locates a value. To build a report, start with table, which writes records under a header row with a number format per column; write adds other cells, such as a title or a totals row like =SUM(C2:C20); chart plots a range. Writes go only into empty cells: existing cells cannot be changed and rows cannot be inserted, deleted or sorted, so put corrected or sorted data on a new sheet. Prefer formulas to numbers you computed, and keep to common functions such as SUM, AVERAGE, IF, SUMIFS and COUNTIFS. Formulas are calculated when the user opens the file, so read shows their cells as empty until then.`
	}
	return `Excel workbooks, .xlsx only, at workspace paths only. Share a finished report by copying it to agent_uploads/<name> and linking it. Read in steps: sheets, then profile, then read — there is no whole-sheet dump; use find when profile is not enough. Build a report as a table rather than ad-hoc cell writes, then add chart, format and highlight. Sheet references in results are quoted, like 'Data'!D4. Writes go only into empty cells and leave existing tables, autofilters, frozen panes and column widths as they are, but every edit rewrites the sheet and may drop features the xlsx library does not model — work on a copy when the original matters. Not supported, and refused: resizing a table, changing an autofilter, inserting or deleting rows or columns, sorting, pivots, data validation, VBA macros, links outside the workbook (other workbooks, WEBSERVICE, FILTERXML, IMAGE, RTD, CALL, REGISTER.ID, DDE), dynamic-array functions (FILTER, SORT, UNIQUE, XLOOKUP and the like), LAMBDA and its helpers (LET, MAP, REDUCE, SCAN, BYROW, BYCOL, MAKEARRAY, ISOMITTED), GROUPBY, PIVOTBY, PERCENTOF and the REGEX functions. Plain functions newer than Excel 2007, such as IFS, SWITCH, MAXIFS, MINIFS, TEXTJOIN, CONCAT, IFNA and STDEV.S, work; their _xlfn. prefix is added for you. Prefer formulas over computed values and keep to common functions (SUM, AVERAGE, COUNTA, IF, SUMIFS, COUNTIFS). For bulk or looping work, such as many blocks or one write per group, run a js script that calls xls__write instead of many separate calls. Formulas these tools write are calculated when the user opens the file, on purpose: until then read and find see those cells as empty and raw read shows the formula, so don't replace them with computed values.`
}

func Module(c Config) common.ModuleFactory {
	c.initDefaults()
	return func(ctx context.Context) common.Module {
		light := common.LightMode(ctx)
		m := tools.NewModule("xls", moduleDesc(light))
		add(m, c, "sheets", sheetsDesc, sheetsTool)
		if light {
			add(m, c, "read", readDesc(light), func(ctx context.Context, arg lightReadArg) (readResult, error) {
				return readTool(ctx, readArg{Path: arg.Path, Sheet: arg.Sheet, Range: arg.Range, Offset: arg.Offset})
			})
		} else {
			add(m, c, "read", readDesc(light), readTool)
		}
		add(m, c, "find", findDesc, findTool)
		add(m, c, "table", tableDesc(light), tableTool)
		add(m, c, "write", writeDesc(light), writeTool)
		add(m, c, "chart", chartDesc, chartTool)
		if !light {
			add(m, c, "profile", profileDesc, profileTool)
			add(m, c, "total", totalDesc, totalTool)
			add(m, c, "format", formatDesc, formatTool)
			add(m, c, "highlight", highlightDesc, highlightTool)
		}
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
