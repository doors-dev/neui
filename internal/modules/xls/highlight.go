package xls

import (
	"context"
	"fmt"
	"strconv"

	"github.com/xuri/excelize/v2"
	"neui.dev/internal/common"
)

const highlightDesc = `Add conditional formatting over a range: a threshold rule (gt, lt), top or bottom N, a contains match, a color scale or data bars.`

type highlightRule struct {
	Type  string `json:"type" required:"true" description:"gt, lt, top, bottom, contains, colorScale or dataBar"`
	Value string `json:"value" description:"threshold for gt/lt or count for top/bottom, as a string like \"500\"; substring for contains; ignored for colorScale and dataBar"`
}

type highlightArg struct {
	Path  string        `json:"path" required:"true"`
	Sheet string        `json:"sheet" required:"true"`
	Range string        `json:"range" required:"true" description:"cells to format, like B2:B20 — usually a column's data cells, header left out"`
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
	case "gt":
		opt.Type, opt.Criteria = "cell", ">"
	case "lt":
		opt.Type, opt.Criteria = "cell", "<"
	case "top", "bottom":
		if n, err := strconv.Atoi(rule.Value); err != nil || n <= 0 {
			return excelize.ConditionalFormatOptions{}, fmt.Errorf("rule %q needs an integer count, got %q", rule.Type, rule.Value)
		}
		opt.Type, opt.Criteria = rule.Type, "="
	case "contains":
		opt.Type, opt.Criteria = "text", "containing"
	default:
		return excelize.ConditionalFormatOptions{}, fmt.Errorf("rule type %q: pick gt, lt, top, bottom, contains, colorScale or dataBar", rule.Type)
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
