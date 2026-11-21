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
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const highlightDesc = `Add a conditional formatting rule to a range.`

type highlightRule struct {
	Type  string `json:"type" required:"true" enum:"gt,lt,top,bottom,contains,colorScale,dataBar"`
	Value string `json:"value" description:"threshold for gt and lt, count for top and bottom, as a string like \"500\"; substring for contains; unused by colorScale and dataBar"`
}

type highlightArg struct {
	Path  string        `json:"path" required:"true"`
	Sheet string        `json:"sheet" required:"true"`
	Range string        `json:"range" required:"true" description:"like B2:B20, usually a column's data cells without the header"`
	Rule  highlightRule `json:"rule" required:"true"`
}

func highlightOpt(f *excelize.File, rule highlightRule) (excelize.ConditionalFormatOptions, error) {
	switch rule.Type {
	case "colorScale":
		return excelize.ConditionalFormatOptions{
			Type:     "3_color_scale",
			Criteria: "=",
			MinType:  "min",
			MidType:  "percentile",
			MaxType:  "max",
			MinColor: "#F8696B",
			MidColor: "#FFEB84",
			MaxColor: "#63BE7B",
		}, nil
	case "dataBar":
		return excelize.ConditionalFormatOptions{
			Type:     "data_bar",
			Criteria: "=",
			MinType:  "min",
			MaxType:  "max",
			BarColor: "#638EC6",
		}, nil
	}
	if rule.Value == "" {
		return excelize.ConditionalFormatOptions{}, fmt.Errorf("rule %q needs a value", rule.Type)
	}
	format, err := f.NewConditionalStyle(&excelize.Style{
		Font: &excelize.Font{Color: "9A0511"},
		Fill: excelize.Fill{Type: "pattern", Color: []string{"FEC7CE"}, Pattern: 1},
	})
	if err != nil {
		return excelize.ConditionalFormatOptions{}, err
	}
	opt := excelize.ConditionalFormatOptions{Value: rule.Value, Format: &format}
	switch rule.Type {
	case "gt", "lt":
		n, err := strconv.ParseFloat(strings.TrimSpace(rule.Value), 64)
		if err != nil || math.IsNaN(n) || math.IsInf(n, 0) {
			return excelize.ConditionalFormatOptions{}, fmt.Errorf("rule %q needs a plain number like 500 or -2.5, got %q", rule.Type, rule.Value)
		}
		opt.Type, opt.Criteria, opt.Value = "cell", ">", strconv.FormatFloat(n, 'f', -1, 64)
		if rule.Type == "lt" {
			opt.Criteria = "<"
		}
	case "top", "bottom":
		if n, err := strconv.Atoi(rule.Value); err != nil || n <= 0 {
			return excelize.ConditionalFormatOptions{}, fmt.Errorf("rule %q needs an integer count, got %q", rule.Type, rule.Value)
		}
		opt.Type, opt.Criteria = rule.Type, "="
	case "contains":
		opt.Type, opt.Criteria = "text", "containing"
	}
	return opt, nil
}

func highlightTool(ctx context.Context, arg highlightArg) (struct{}, error) {
	fs := common.GetController(ctx).FS()
	f, _, release, err := openWrite(ctx, fs, arg.Path, false)
	if err != nil {
		return struct{}{}, err
	}
	defer release()
	opt, err := highlightOpt(f, arg.Rule)
	if err != nil {
		return struct{}{}, err
	}
	if err := f.SetConditionalFormat(arg.Sheet, arg.Range, []excelize.ConditionalFormatOptions{opt}); err != nil {
		return struct{}{}, err
	}
	return struct{}{}, save(ctx, fs, f, arg.Path)
}
