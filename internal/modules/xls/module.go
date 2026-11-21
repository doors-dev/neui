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

func moduleDesc(tier common.ModelTier) string {
	read, finish := "sheets, then profile, then read", "chart, format and highlight"
	if tier == common.TierAssistant {
		read, finish = "sheets, then read", "chart"
	}
	return `Excel workbooks (.xlsx) at workspace paths: copy an uploaded workbook into the workspace before using it; a .csv file is plain text, read it as text. To read a workbook: ` + read + `; find locates a value. To build a report, start with table, then write other cells, such as a title or a totals row like =SUM(C2:C20), and finish with ` + finish + `. In write and table rows, a string starting with = is a formula, and {row} in it becomes the row number of the cell it lands in. Writes go only into empty cells: existing cells cannot be changed, tables and autofilters keep their ranges, and rows and columns cannot be inserted, deleted or sorted, so put corrected or sorted data on a new sheet. Prefer formulas over computed values and keep to classic functions such as SUM, AVERAGE, COUNTA, IF, SUMIFS, COUNTIFS and INDEX/MATCH; dynamic-array and LAMBDA functions, such as XLOOKUP, FILTER and LET, are refused. Formulas are calculated when the user opens the file, so until then read and find show their cells as empty; don't replace them with computed values.`
}

func Module(c Config) common.ModuleFactory {
	c.initDefaults()
	return func(ctx context.Context) common.Module {
		tier := common.Tier(ctx)
		m := tools.NewModule("xls", moduleDesc(tier))
		add(m, c, "sheets", sheetsDesc, sheetsTool)
		if tier == common.TierAssistant {
			add(m, c, "read", readDesc, func(ctx context.Context, arg assistantReadArg) (readResult, error) {
				return readTool(ctx, readArg{Path: arg.Path, Sheet: arg.Sheet, Range: arg.Range, Offset: arg.Offset})
			})
		} else {
			add(m, c, "read", readDesc, readTool)
		}
		add(m, c, "find", findDesc, findTool)
		add(m, c, "table", tableDesc, tableTool)
		add(m, c, "write", writeDesc, writeTool)
		add(m, c, "chart", chartDesc, chartTool)
		if tier != common.TierAssistant {
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
