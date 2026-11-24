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

package chart

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	"neui.dev/internal/schema"
)

func records(t *testing.T, raw string) []record {
	t.Helper()
	var rows []record
	if err := json.Unmarshal([]byte(raw), &rows); err != nil {
		t.Fatal(err)
	}
	return rows
}

func option(t *testing.T, p Params, raw string) (view, map[string]any) {
	t.Helper()
	if p.Format == "" {
		p.Format = "number"
	}
	shown, err := compile(p, records(t, raw))
	if err != nil {
		t.Fatalf("compile failed: %v", err)
	}
	data, err := json.Marshal(shown.Option)
	if err != nil {
		t.Fatal(err)
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatal(err)
	}
	return shown, out
}

func at(v any, path ...any) any {
	for _, step := range path {
		switch key := step.(type) {
		case string:
			m, _ := v.(map[string]any)
			v = m[key]
		case int:
			s, _ := v.([]any)
			if key >= len(s) {
				return nil
			}
			v = s[key]
		}
	}
	return v
}

func jsonOf(t *testing.T, v any) string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

func names(o map[string]any) string {
	var out []string
	for _, s := range at(o, "series").([]any) {
		out = append(out, fmt.Sprint(at(s, "name")))
	}
	return strings.Join(out, " ")
}

func TestNewRefusesInvalidParams(t *testing.T) {
	query := `() => ({ rows: [] })`
	cases := []struct {
		name string
		p    string
		want string
	}{
		{"no source", `{"type":"line","x":"m","y":"v"}`, sourceMessage},
		{"both sources", `{"type":"line","x":"m","y":"v","rows":[{"m":"a","v":1}],"query":"` + query + `"}`, sourceMessage},
		{"refresh without query", `{"type":"line","x":"m","y":"v","rows":[{"m":"a","v":1}],"refresh":30}`, refreshMessage},
		{"pie split", `{"type":"pie","x":"m","y":"v","splitBy":"g","rows":[{"m":"a","v":1,"g":"x"}]}`, "pie takes no splitBy: x already names each slice"},
		{"funnel split", `{"type":"funnel","x":"m","y":"v","splitBy":"g","rows":[{"m":"a","v":1,"g":"x"}]}`, "funnel takes no splitBy: x already names each stage"},
		{"line stack", `{"type":"line","x":"m","y":"v","stack":true,"rows":[{"m":"a","v":1}]}`, stackMessage},
		{"area stack", `{"type":"area","x":"m","y":"v","stack":true,"rows":[{"m":"a","v":1}]}`, stackMessage},
		{"pie stack", `{"type":"pie","x":"m","y":"v","stack":true,"rows":[{"m":"a","v":1}]}`, stackMessage},
		{"currency without unit", `{"type":"bar","x":"m","y":"v","format":"currency","rows":[{"m":"a","v":1}]}`, currencyUnitMessage},
		{"currency with unknown code", `{"type":"bar","x":"m","y":"v","format":"currency","unit":"ABC","rows":[{"m":"a","v":1}]}`, `unit "ABC" is not an ISO 4217 currency code`},
		{"currency with a word", `{"type":"bar","x":"m","y":"v","format":"currency","unit":"dollars","rows":[{"m":"a","v":1}]}`, `unit "dollars" is not an ISO 4217 currency code`},
		{"percent with unit", `{"type":"bar","x":"m","y":"v","format":"percent","unit":"%","rows":[{"m":"a","v":1}]}`, "unit is only for number and currency; percent values need none"},
		{"missing x", `{"type":"bar","x":"m","y":"v","rows":[{"m":"a","v":1},{"v":2}]}`, `rows[1] has no x field "m"`},
		{"null x", `{"type":"bar","x":"m","y":"v","rows":[{"m":null,"v":1}]}`, `rows[0]: x field "m" is null`},
		{"missing y", `{"type":"bar","x":"m","y":"v","rows":[{"m":"a"}]}`, `rows[0] has no y field "v"`},
		{"text y", `{"type":"bar","x":"m","y":"v","rows":[{"m":"a","v":"12"}]}`, `rows[0]: y field "v" is "12", not a number`},
		{"negative pie", `{"type":"pie","x":"m","y":"v","rows":[{"m":"a","v":1},{"m":"b","v":-1}]}`, `rows[1]: y field "v" is negative; pie takes values of 0 or more`},
		{"negative funnel", `{"type":"funnel","x":"m","y":"v","rows":[{"m":"a","v":-1}]}`, `funnel takes values of 0 or more`},
		{"scatter text x", `{"type":"scatter","x":"m","y":"v","rows":[{"m":"a","v":1}]}`, `rows[0]: x field "m" is not a number`},
		{"missing splitBy", `{"type":"line","x":"m","y":"v","splitBy":"g","rows":[{"m":"a","v":1}]}`, `rows[0] has no splitBy field "g"`},
		{"null splitBy", `{"type":"line","x":"m","y":"v","splitBy":"g","rows":[{"m":"a","v":1,"g":null}]}`, `rows[0]: splitBy field "g" is null`},
	}
	for _, c := range cases {
		var p Params
		if err := json.Unmarshal([]byte(c.p), &p); err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		_, err := New(context.Background(), p)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: New returned %v, want %q", c.name, err, c.want)
		}
	}
}

func TestNewAcceptsValidParams(t *testing.T) {
	cases := []string{
		`{"type":"bar","x":"m","y":"v","format":"currency","unit":"EUR","rows":[{"m":"a","v":1}]}`,
		`{"type":"bar","x":"m","y":"v","unit":"kg","stack":true,"splitBy":"g","rows":[{"m":"a","v":1,"g":"x"}]}`,
		`{"type":"line","x":"m","y":"v","rows":[{"m":"a","v":null}]}`,
		`{"type":"pie","x":"m","y":"v","rows":[{"m":"a","v":0}]}`,
		`{"type":"scatter","x":"m","y":"v","splitBy":"g","rows":[{"m":1.5,"v":1,"g":2}]}`,
		`{"type":"area","x":"m","y":"v","refresh":5,"query":"() => ({ rows: [] })"}`,
	}
	for _, c := range cases {
		var p Params
		if err := json.Unmarshal([]byte(c), &p); err != nil {
			t.Fatalf("%s: %v", c, err)
		}
		if _, err := New(context.Background(), p); err != nil {
			t.Errorf("New(%s) failed: %v", c, err)
		}
	}
}

func TestSchemaChecksRecordsAndPatches(t *testing.T) {
	params, err := schema.NewSchema[Params]()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := params.DecodeString(`{"type":"bar","x":"m","y":"v","rows":[{"m":"a","v":1,"n":null}]}`); err != nil {
		t.Errorf("flat records failed: %v", err)
	}
	if _, err := params.DecodeString(`{"type":"bar","x":"m","y":"v","rows":[]}`); err != nil {
		t.Errorf("no records failed: %v", err)
	}
	for _, bad := range []string{
		`{"type":"bar","x":"m","y":"v","rows":[{"m":"a","v":true}]}`,
		`{"type":"bar","x":"m","y":"v","rows":[{"m":"a","v":{"n":1}}]}`,
		`{"type":"combo","x":"m","y":"v","rows":[{"m":"a","v":1}]}`,
		`{"type":"bar","x":"m","y":"v","format":"euro","rows":[{"m":"a","v":1}]}`,
	} {
		if _, err := params.DecodeString(bad); err == nil {
			t.Errorf("params %s passed the schema", bad)
		}
	}
	patch, err := schema.NewSchema[Patch]()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := patch.DecodeString(`{"rows":[{"m":"a","v":1}]}`); err != nil {
		t.Errorf("a rows patch failed: %v", err)
	}
	if _, err := patch.DecodeString(`{"query":"() => ({ rows: [] })"}`); err != nil {
		t.Errorf("a query patch failed: %v", err)
	}
	for _, bad := range []string{`{}`, `{"rows":[{"m":"a","v":1}],"query":"() => ({ rows: [] })"}`, `{"refresh":10}`} {
		if _, err := patch.DecodeString(bad); err == nil {
			t.Errorf("patch %s passed the schema", bad)
		}
	}
}

func TestCompileDrawsNothingForNoRecords(t *testing.T) {
	shown, err := compile(Params{Type: "bar", X: "m", Y: "v"}, nil)
	if err != nil || shown.Option != nil || shown.Stale {
		t.Fatalf("compile of no records returned %+v, %v, want an empty view and no error", shown, err)
	}
}

func TestCompileFoldsGroupsPastEight(t *testing.T) {
	var rows []string
	for i := 1; i <= 10; i++ {
		rows = append(rows, fmt.Sprintf(`{"m":"a","g":"g%d","v":%d}`, i, i), fmt.Sprintf(`{"m":"b","g":"g%d","v":%d}`, i, i*10))
	}
	_, o := option(t, Params{Type: "bar", X: "m", Y: "v", SplitBy: "g", Stack: true}, "["+strings.Join(rows, ",")+"]")
	if got := names(o); got != "g4 g5 g6 g7 g8 g9 g10 Other" {
		t.Fatalf("series %q, want the 7 largest in row order, then Other", got)
	}
	if got := jsonOf(t, at(o, "series", 7, "data")); got != "[6,60]" {
		t.Fatalf("Other holds %s, want [6,60]: g1 to g3 added up", got)
	}
	if at(o, "series", 0, "stack") != "total" || at(o, "legend", "show") != true {
		t.Fatalf("stacked bars %s, want stack total and a legend", jsonOf(t, o))
	}
}

func TestCompileKeepsEightGroups(t *testing.T) {
	var rows []string
	for i := 1; i <= 8; i++ {
		rows = append(rows, fmt.Sprintf(`{"m":"a","g":"g%d","v":%d}`, i, i))
	}
	_, o := option(t, Params{Type: "line", X: "m", Y: "v", SplitBy: "g"}, "["+strings.Join(rows, ",")+"]")
	if got := names(o); got != "g1 g2 g3 g4 g5 g6 g7 g8" {
		t.Fatalf("series %q, want all 8 groups", got)
	}
}

func TestCompileFoldsPieSlicesPastSix(t *testing.T) {
	_, o := option(t, Params{Type: "pie", X: "m", Y: "v"}, `[
		{"m":"a","v":1},{"m":"b","v":7},{"m":"c","v":3},{"m":"d","v":9},
		{"m":"e","v":5},{"m":"f","v":2},{"m":"g","v":8},{"m":"b","v":1}
	]`)
	if got := jsonOf(t, at(o, "series", 0, "data")); got != `[{"name":"b","value":8},{"name":"c","value":3},{"name":"d","value":9},{"name":"e","value":5},{"name":"g","value":8},{"name":"Other","value":3}]` {
		t.Fatalf("pie slices %s, want the 5 largest in row order, b added up, then Other", got)
	}
	_, o = option(t, Params{Type: "pie", X: "m", Y: "v"}, `[{"m":"a","v":1},{"m":"b","v":null},{"m":"c","v":0}]`)
	if got := jsonOf(t, at(o, "series", 0, "data")); got != `[{"name":"a","value":1},{"name":"c","value":0}]` {
		t.Fatalf("pie slices %s, want the null slice left out", got)
	}
}

func TestCompileFunnelKeepsRowOrder(t *testing.T) {
	var rows []string
	for i := 1; i <= 9; i++ {
		rows = append(rows, fmt.Sprintf(`{"stage":"s%d","n":%d}`, i, i*i))
	}
	_, o := option(t, Params{Type: "funnel", X: "stage", Y: "n"}, "["+strings.Join(rows, ",")+"]")
	series := at(o, "series", 0)
	if at(series, "sort") != "none" || at(series, "max") != float64(81) || len(at(series, "data").([]any)) != 9 {
		t.Fatalf("funnel %s, want 9 stages unsorted with max 81", jsonOf(t, series))
	}
	if at(series, "data", 0, "name") != "s1" || at(series, "data", 8, "name") != "s9" {
		t.Fatalf("funnel stages %s, want row order", jsonOf(t, at(series, "data")))
	}
}

func TestCompileTimeAxis(t *testing.T) {
	shown, o := option(t, Params{Type: "line", X: "day", Y: "v"}, `[
		{"day":"2026-03-02","v":2},{"day":"2026-03-01","v":1},{"day":"2026-03-02","v":5}
	]`)
	if at(o, "xAxis", "type") != "time" || at(o, "xAxis", "minInterval") != float64(86400000) || at(o, "useUTC") != true || shown.Time != "date" {
		t.Fatalf("dates gave %s and time %q, want a UTC time axis of dates", jsonOf(t, o), shown.Time)
	}
	if got := jsonOf(t, at(o, "series", 0, "data")); got != "[[1772323200000,1],[1772409600000,7]]" {
		t.Fatalf("points %s, want sorted by time with the same day added up", got)
	}
	shown, o = option(t, Params{Type: "area", X: "at", Y: "v"}, `[{"at":"2026-03-01T10:00:00Z","v":1},{"at":"2026-03-01T09:30:00.5+01:00","v":2}]`)
	if at(o, "xAxis", "type") != "time" || at(o, "xAxis", "minInterval") != nil || at(o, "useUTC") != false || shown.Time != "datetime" {
		t.Fatalf("zoned date-times gave %s and time %q, want a local time axis", jsonOf(t, o), shown.Time)
	}
	if got := jsonOf(t, at(o, "series", 0, "data", 0)); got != "[1772353800500,2]" {
		t.Fatalf("first point %s, want the earlier instant", got)
	}
	shown, o = option(t, Params{Type: "line", X: "m", Y: "v"}, `[{"m":"2026-01","v":1},{"m":"2026-02","v":2}]`)
	if shown.Time != "month" || at(o, "xAxis", "minInterval") != float64(28*86400000) {
		t.Fatalf("months gave time %q and %s, want month ticks at least 28 days apart", shown.Time, jsonOf(t, at(o, "xAxis")))
	}
	_, o = option(t, Params{Type: "line", X: "m", Y: "v"}, `[{"m":"2026-03-01","v":1},{"m":"later","v":2}]`)
	if at(o, "xAxis", "type") != "category" || jsonOf(t, at(o, "xAxis", "data")) != `["2026-03-01","later"]` {
		t.Fatalf("mixed x gave %s, want a category axis in row order", jsonOf(t, at(o, "xAxis")))
	}
	_, o = option(t, Params{Type: "bar", X: "day", Y: "v"}, `[{"day":"2026-03-02","v":2},{"day":"2026-03-01","v":1}]`)
	if at(o, "xAxis", "type") != "category" || jsonOf(t, at(o, "xAxis", "data")) != `["2026-03-02","2026-03-01"]` {
		t.Fatalf("bar dates gave %s, want categories in row order", jsonOf(t, at(o, "xAxis")))
	}
}

func TestCompileStackedAreaAlignsTimes(t *testing.T) {
	_, o := option(t, Params{Type: "area", X: "day", Y: "v", SplitBy: "g"}, `[
		{"day":"2026-03-01","g":"a","v":1},{"day":"2026-03-02","g":"b","v":2},{"day":"2026-03-03","g":"a","v":null},{"day":"2026-03-03","g":"b","v":3}
	]`)
	if got := jsonOf(t, at(o, "series", 0, "data")); got != "[[1772323200000,1],[1772409600000,0],[1772496000000,null]]" {
		t.Fatalf("series a %s, want every time, 0 where a has no record and a gap where it is null", got)
	}
	if got := jsonOf(t, at(o, "series", 1, "data")); got != "[[1772323200000,0],[1772409600000,2],[1772496000000,3]]" {
		t.Fatalf("series b %s, want every time with 0 where b has no record", got)
	}
	if at(o, "series", 1, "stack") != "total" {
		t.Fatalf("area with splitBy %s, want it stacked", jsonOf(t, at(o, "series", 1)))
	}
	_, o = option(t, Params{Type: "line", X: "day", Y: "v", SplitBy: "g"}, `[
		{"day":"2026-03-01","g":"a","v":1},{"day":"2026-03-02","g":"b","v":2}
	]`)
	if got := jsonOf(t, at(o, "series", 0, "data")); got != "[[1772323200000,1],[1772409600000,null]]" || at(o, "series", 0, "stack") != nil {
		t.Fatalf("line a %s, want every time, a gap where a has no record, and no stack", jsonOf(t, at(o, "series", 0)))
	}
}

func TestCompileStackedCategoriesCountMissingAsZero(t *testing.T) {
	rows := `[
		{"m":"Jan","g":"a","v":1},{"m":"Jan","g":"b","v":2},{"m":"Feb","g":"b","v":3},{"m":"Mar","g":"a","v":null},{"m":"Mar","g":"b","v":4}
	]`
	_, o := option(t, Params{Type: "bar", X: "m", Y: "v", SplitBy: "g", Stack: true}, rows)
	if got := jsonOf(t, []any{at(o, "series", 0, "data"), at(o, "series", 1, "data")}); got != "[[1,0,null],[2,3,4]]" {
		t.Fatalf("stacked bars %s, want 0 where a has no record and a gap where it is null", got)
	}
	_, o = option(t, Params{Type: "area", X: "m", Y: "v", SplitBy: "g"}, rows)
	if got := jsonOf(t, at(o, "series", 0, "data")); got != "[1,0,null]" {
		t.Fatalf("stacked area %s, want 0 where a has no record and a gap where it is null", got)
	}
	for _, p := range []Params{{Type: "bar", X: "m", Y: "v", SplitBy: "g"}, {Type: "line", X: "m", Y: "v", SplitBy: "g"}} {
		_, o = option(t, p, rows)
		if got := jsonOf(t, at(o, "series", 0, "data")); got != "[1,null,null]" {
			t.Fatalf("%s %s, want gaps where a has no record or is null", p.Type, got)
		}
	}
}

func TestCompileSummarisesWhatItDraws(t *testing.T) {
	for _, c := range []struct {
		p    Params
		rows string
		want string
	}{
		{Params{Type: "line", X: "m", Y: "v", SplitBy: "g"}, `[{"m":"2026-03","g":"a","v":1},{"m":"2026-01","g":"b","v":2},{"m":"2026-02","g":"a","v":null}]`, `{"type":"","state":"","series":["b","a"],"points":3,"first":"2026-01","last":"2026-03"}`},
		{Params{Type: "bar", X: "m", Y: "v"}, `[{"m":"Jan","v":1},{"m":"Feb","v":2},{"m":"Jan","v":3}]`, `{"type":"","state":"","series":["v"],"points":2,"first":"Jan","last":"Feb"}`},
		{Params{Type: "scatter", X: "x", Y: "y"}, `[{"x":3,"y":1},{"x":-1.5,"y":2},{"x":9,"y":null}]`, `{"type":"","state":"","series":["y"],"points":2,"first":"-1.5","last":"3"}`},
		{Params{Type: "pie", X: "k", Y: "v"}, `[{"k":"a","v":1},{"k":"b","v":null},{"k":"c","v":2}]`, `{"type":"","state":"","series":["a","c"],"points":2}`},
		{Params{Type: "funnel", X: "k", Y: "v"}, `[{"k":"seen","v":9},{"k":"lost","v":null},{"k":"paid","v":2}]`, `{"type":"","state":"","series":["seen","paid"],"points":2}`},
	} {
		c.p.Format = "number"
		shown, err := compile(c.p, records(t, c.rows))
		if err != nil {
			t.Fatal(err)
		}
		if got := jsonOf(t, shown.Drawn); got != c.want {
			t.Errorf("%s: got %s, want %s", c.p.Type, got, c.want)
		}
	}
}

func TestCompileCategoriesAddUpAndGap(t *testing.T) {
	_, o := option(t, Params{Type: "bar", X: "m", Y: "v", SplitBy: "g"}, `[
		{"m":"Jan","g":"a","v":1},{"m":"Jan","g":"a","v":2},{"m":"Feb","g":"b","v":3},{"m":"Feb","g":"a","v":null},{"m":2026,"g":"b","v":4}
	]`)
	if got := jsonOf(t, at(o, "xAxis", "data")); got != `["Jan","Feb","2026"]` {
		t.Fatalf("categories %s, want Jan Feb 2026 in row order", got)
	}
	if got := jsonOf(t, []any{at(o, "series", 0, "data"), at(o, "series", 1, "data")}); got != "[[3,null,null],[null,3,4]]" {
		t.Fatalf("series data %s, want a's Jan added up and gaps elsewhere", got)
	}
	if at(o, "series", 0, "stack") != nil {
		t.Fatalf("unstacked bars %s, want no stack", jsonOf(t, at(o, "series", 0)))
	}
}

func TestCompileTurnsBarsHorizontal(t *testing.T) {
	var twelve, thirteen []string
	for i := 1; i <= 13; i++ {
		row := fmt.Sprintf(`{"m":"c%d","v":%d}`, i, i)
		if i <= 12 {
			twelve = append(twelve, row)
		}
		thirteen = append(thirteen, row)
	}
	shown, o := option(t, Params{Type: "bar", X: "m", Y: "v"}, "["+strings.Join(twelve, ",")+"]")
	if at(o, "xAxis", "type") != "category" || shown.Measure != "yAxis" {
		t.Fatalf("12 short categories gave x %v and measure %q, want vertical bars", at(o, "xAxis", "type"), shown.Measure)
	}
	shown, o = option(t, Params{Type: "bar", X: "m", Y: "v"}, "["+strings.Join(thirteen, ",")+"]")
	if at(o, "yAxis", "type") != "category" || at(o, "yAxis", "inverse") != true || shown.Measure != "xAxis" {
		t.Fatalf("13 categories gave %s and measure %q, want horizontal bars top down", jsonOf(t, at(o, "yAxis")), shown.Measure)
	}
	_, o = option(t, Params{Type: "bar", X: "m", Y: "v"}, `[{"m":"a label of 13c","v":1}]`)
	if at(o, "yAxis", "type") != "category" {
		t.Fatalf("a 13 character label gave %s, want horizontal bars", jsonOf(t, at(o, "yAxis")))
	}
	_, o = option(t, Params{Type: "line", X: "m", Y: "v"}, "["+strings.Join(thirteen, ",")+"]")
	if at(o, "xAxis", "type") != "category" {
		t.Fatalf("a line with 13 categories gave %s, want x along the bottom", jsonOf(t, at(o, "xAxis")))
	}
}

func TestCompileScatter(t *testing.T) {
	shown, o := option(t, Params{Type: "scatter", X: "price", Y: "sold", SplitBy: "shop"}, `[
		{"price":1.5,"sold":10,"shop":"north"},{"price":2,"sold":null,"shop":"north"},{"price":3,"sold":4,"shop":"south"}
	]`)
	if names(o) != "north south" || jsonOf(t, at(o, "series", 0, "data")) != "[[1.5,10]]" {
		t.Fatalf("scatter %s, want a series per shop and no point for null", jsonOf(t, at(o, "series")))
	}
	if at(o, "xAxis", "type") != "value" || at(o, "xAxis", "name") != "price" || shown.Measure != "yAxis" || at(o, "tooltip", "trigger") != "item" {
		t.Fatalf("scatter axes %s, want value axes named by the fields", jsonOf(t, o))
	}
}
