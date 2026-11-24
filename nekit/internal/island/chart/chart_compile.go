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
	"cmp"
	"fmt"
	"slices"
	"strconv"
	"time"
	"unicode/utf8"
)

var timeKinds = []string{"month", "date", "datetime"}

var timeLayouts = []struct {
	layout string
	kind   int
	zoned  bool
}{
	{"2006-01", 0, false},
	{"2006-01-02", 1, false},
	{"2006-01-02T15:04", 2, false},
	{"2006-01-02T15:04:05", 2, false},
	{"2006-01-02T15:04Z07:00", 2, true},
	{"2006-01-02T15:04:05Z07:00", 2, true},
	{"2006-01-02 15:04", 2, false},
	{"2006-01-02 15:04:05", 2, false},
	{"2006-01-02 15:04:05Z07:00", 2, true},
}

func compile(p Params, rows []record) (view, error) {
	if len(rows) == 0 {
		return view{Format: p.Format, Unit: p.Unit}, nil
	}
	for i, r := range rows {
		x, ok := r[p.X]
		if !ok {
			return view{}, fmt.Errorf(missingFieldMessage, i, "x", p.X)
		}
		if x == nil {
			return view{}, fmt.Errorf(nullFieldMessage, i, "x", p.X)
		}
		if _, number := x.(float64); p.Type == "scatter" && !number {
			return view{}, fmt.Errorf(scatterXMessage, i, p.X)
		}
		y, ok := r[p.Y]
		if !ok {
			return view{}, fmt.Errorf(missingFieldMessage, i, "y", p.Y)
		}
		number, numeric := y.(float64)
		if y != nil && !numeric {
			return view{}, fmt.Errorf(numericYMessage, i, p.Y, fmt.Sprint(y))
		}
		if numeric && number < 0 && (p.Type == "pie" || p.Type == "funnel") {
			return view{}, fmt.Errorf(negativeMessage, i, p.Y, p.Type)
		}
		if p.SplitBy == "" {
			continue
		}
		split, ok := r[p.SplitBy]
		if !ok {
			return view{}, fmt.Errorf(missingFieldMessage, i, "splitBy", p.SplitBy)
		}
		if split == nil {
			return view{}, fmt.Errorf(nullFieldMessage, i, "splitBy", p.SplitBy)
		}
	}
	label := func(value any) string {
		if number, ok := value.(float64); ok {
			return strconv.FormatFloat(number, 'f', -1, 64)
		}
		return fmt.Sprint(value)
	}
	add := func(sum any, value any) any {
		number, ok := value.(float64)
		if !ok {
			return sum
		}
		if previous, ok := sum.(float64); ok {
			return previous + number
		}
		return number
	}
	shown := view{Format: p.Format, Unit: p.Unit, X: p.X, Y: p.Y}
	option := map[string]any{"aria": map[string]any{"enabled": true}}
	shown.Option = option
	var times []time.Time
	timeKind, zoned := 0, false
	if p.Type == "line" || p.Type == "area" {
		times = make([]time.Time, len(rows))
		for i, r := range rows {
			text, _ := r[p.X].(string)
			found := false
			for _, layout := range timeLayouts {
				if parsed, err := time.ParseInLocation(layout.layout, text, time.UTC); err == nil {
					times[i], timeKind, zoned, found = parsed, max(timeKind, layout.kind), zoned || layout.zoned, true
					break
				}
			}
			if !found {
				times = nil
				break
			}
		}
	}
	order := make([]int, len(rows))
	for i := range order {
		order[i] = i
	}
	if times != nil {
		slices.SortStableFunc(order, func(a int, b int) int {
			return times[a].Compare(times[b])
		})
	}
	if p.Type == "pie" {
		var names []string
		sums := map[string]any{}
		for _, i := range order {
			name := label(rows[i][p.X])
			if _, seen := sums[name]; !seen {
				names = append(names, name)
			}
			sums[name] = add(sums[name], rows[i][p.Y])
		}
		names = slices.DeleteFunc(names, func(name string) bool {
			return sums[name] == nil
		})
		if len(names) > 6 {
			ranked := slices.Clone(names)
			slices.SortStableFunc(ranked, func(a string, b string) int {
				return cmp.Compare(sums[b].(float64), sums[a].(float64))
			})
			for _, name := range ranked[5:] {
				if name != otherName {
					sums[otherName] = add(sums[otherName], sums[name])
				}
			}
			names = slices.DeleteFunc(names, func(name string) bool {
				return name == otherName || !slices.Contains(ranked[:5], name)
			})
			names = append(names, otherName)
		}
		data := []any{}
		for _, name := range names {
			data = append(data, map[string]any{"name": name, "value": sums[name]})
		}
		shown.Drawn = chartView{Series: names, Points: len(names)}
		option["tooltip"] = map[string]any{"trigger": "item", "confine": true}
		option["series"] = []any{map[string]any{
			"type":              "pie",
			"cursor":            "default",
			"name":              p.Y,
			"radius":            []string{"30%", "48%"},
			"avoidLabelOverlap": true,
			"itemStyle":         map[string]any{"borderWidth": 2, "borderRadius": 3},
			"label":             map[string]any{"show": true, "alignTo": "edge", "edgeDistance": 4, "minMargin": 4, "lineHeight": 15, "overflow": "break"},
			"labelLine":         map[string]any{"length": 10, "length2": 0, "maxSurfaceAngle": 80},
			"data":              data,
		}}
		return shown, nil
	}
	if p.Type == "funnel" {
		data := []any{}
		top := 0.0
		for _, i := range order {
			number, ok := rows[i][p.Y].(float64)
			if !ok {
				continue
			}
			top = max(top, number)
			data = append(data, map[string]any{"name": label(rows[i][p.X]), "value": number})
			shown.Drawn.Series = append(shown.Drawn.Series, label(rows[i][p.X]))
		}
		shown.Drawn.Points = len(data)
		option["tooltip"] = map[string]any{"trigger": "item", "confine": true}
		option["series"] = []any{map[string]any{
			"type":      "funnel",
			"cursor":    "default",
			"name":      p.Y,
			"sort":      "none",
			"min":       0,
			"max":       top,
			"minSize":   "4%",
			"gap":       2,
			"left":      0,
			"top":       8,
			"bottom":    8,
			"width":     "60%",
			"label":     map[string]any{"show": true, "position": "right", "lineHeight": 15},
			"labelLine": map[string]any{"length": 20},
			"data":      data,
		}}
		return shown, nil
	}
	groups := []string{p.Y}
	group := func(r record) string {
		return p.Y
	}
	if p.SplitBy != "" {
		groups = nil
		totals := map[string]float64{}
		for _, i := range order {
			name := label(rows[i][p.SplitBy])
			if _, seen := totals[name]; !seen {
				groups = append(groups, name)
			}
			number, _ := rows[i][p.Y].(float64)
			totals[name] += number
		}
		folded := map[string]bool{}
		if len(groups) > 8 {
			ranked := slices.Clone(groups)
			slices.SortStableFunc(ranked, func(a string, b string) int {
				return cmp.Compare(totals[b], totals[a])
			})
			for _, name := range ranked[7:] {
				folded[name] = true
			}
			groups = slices.DeleteFunc(groups, func(name string) bool {
				return folded[name] || name == otherName
			})
			groups = append(groups, otherName)
		}
		group = func(r record) string {
			name := label(r[p.SplitBy])
			if folded[name] {
				return otherName
			}
			return name
		}
	}
	option["legend"] = map[string]any{"show": len(groups) > 1, "type": "scroll", "top": 0, "left": 0, "icon": "roundRect", "itemWidth": 10, "itemHeight": 10, "itemGap": 16}
	top := 12
	if len(groups) > 1 {
		top = 40
	}
	option["grid"] = map[string]any{"left": 4, "right": 16, "top": top, "bottom": 4}
	valueAxis := map[string]any{"type": "value", "splitLine": map[string]any{"show": true}, "axisLine": map[string]any{"show": false}, "axisLabel": map[string]any{"hideOverlap": true}}
	series := []any{}
	if p.Type == "scatter" {
		points := map[string][]any{}
		least, most := 0.0, 0.0
		for _, i := range order {
			number, ok := rows[i][p.Y].(float64)
			if !ok {
				continue
			}
			x := rows[i][p.X].(float64)
			if shown.Drawn.Points == 0 || x < least {
				least = x
			}
			if shown.Drawn.Points == 0 || x > most {
				most = x
			}
			shown.Drawn.Points++
			name := group(rows[i])
			points[name] = append(points[name], []float64{x, number})
		}
		shown.Drawn.Series = groups
		if shown.Drawn.Points > 0 {
			shown.Drawn.First, shown.Drawn.Last = label(least), label(most)
		}
		for _, name := range groups {
			series = append(series, map[string]any{
				"type":       "scatter",
				"cursor":     "default",
				"name":       name,
				"symbolSize": 9,
				"itemStyle":  map[string]any{"borderWidth": 1, "opacity": 0.85},
				"data":       append([]any{}, points[name]...),
			})
		}
		valueAxis["scale"] = true
		valueAxis["name"] = p.Y
		valueAxis["nameLocation"] = "middle"
		valueAxis["nameGap"] = 48
		option["xAxis"] = map[string]any{"type": "value", "scale": true, "splitLine": map[string]any{"show": true}, "axisLine": map[string]any{"show": false}, "axisLabel": map[string]any{"hideOverlap": true}, "name": p.X, "nameLocation": "middle", "nameGap": 28}
		option["yAxis"] = valueAxis
		option["tooltip"] = map[string]any{"trigger": "item", "confine": true}
		option["series"] = series
		shown.Measure = "yAxis"
		return shown, nil
	}
	newSeries := func(name string, data []any) map[string]any {
		if p.Type == "bar" {
			return map[string]any{"type": "bar", "cursor": "default", "name": name, "data": data, "barMaxWidth": 24}
		}
		s := map[string]any{
			"type":       "line",
			"silent":     true,
			"name":       name,
			"data":       data,
			"symbol":     "circle",
			"symbolSize": 8,
			"showSymbol": len(data) <= 24,
			"lineStyle":  map[string]any{"width": 2},
			"itemStyle":  map[string]any{"borderWidth": 2},
		}
		if p.Type == "area" && p.SplitBy != "" {
			s["stack"] = "total"
			s["areaStyle"] = map[string]any{"opacity": 0.3}
		} else if p.Type == "area" {
			s["areaStyle"] = map[string]any{"opacity": 0.1}
		}
		return s
	}
	shown.Measure = "yAxis"
	option["yAxis"] = valueAxis
	option["tooltip"] = map[string]any{"trigger": "axis", "confine": true, "axisPointer": map[string]any{"type": "line"}}
	stacked := p.Type == "area" && p.SplitBy != "" || p.Type == "bar" && p.Stack
	shown.Drawn.Series = groups
	if times != nil {
		var stamps []int64
		seen := map[int64]bool{}
		values := map[string]map[int64]any{}
		for _, i := range order {
			stamp := times[i].UnixMilli()
			name := group(rows[i])
			if values[name] == nil {
				values[name] = map[int64]any{}
			}
			values[name][stamp] = add(values[name][stamp], rows[i][p.Y])
			if !seen[stamp] {
				seen[stamp] = true
				stamps = append(stamps, stamp)
			}
		}
		for _, name := range groups {
			data := make([]any, len(stamps))
			for j, stamp := range stamps {
				value, found := values[name][stamp]
				if stacked && !found {
					value = 0.0
				}
				data[j] = []any{stamp, value}
			}
			series = append(series, newSeries(name, data))
		}
		shown.Drawn.Points = len(stamps)
		shown.Drawn.First, shown.Drawn.Last = label(rows[order[0]][p.X]), label(rows[order[len(order)-1]][p.X])
		timeAxis := map[string]any{"type": "time", "axisTick": map[string]any{"show": false}, "axisLabel": map[string]any{"hideOverlap": true}}
		if timeKind < 2 {
			timeAxis["minInterval"] = []int64{28, 1}[timeKind] * 24 * time.Hour.Milliseconds()
		}
		option["xAxis"] = timeAxis
		option["useUTC"] = !zoned
		option["series"] = series
		shown.Time = timeKinds[timeKind]
		return shown, nil
	}
	var categories []string
	index := map[string]int{}
	for _, i := range order {
		name := label(rows[i][p.X])
		if _, ok := index[name]; !ok {
			index[name] = len(categories)
			categories = append(categories, name)
		}
	}
	values := map[string][]any{}
	found := map[string][]bool{}
	for _, name := range groups {
		values[name] = make([]any, len(categories))
		found[name] = make([]bool, len(categories))
	}
	for _, i := range order {
		name := group(rows[i])
		at := index[label(rows[i][p.X])]
		values[name][at] = add(values[name][at], rows[i][p.Y])
		found[name][at] = true
	}
	shown.Drawn.Points, shown.Drawn.First, shown.Drawn.Last = len(categories), categories[0], categories[len(categories)-1]
	horizontal := p.Type == "bar" && (len(categories) > 12 || slices.ContainsFunc(categories, func(c string) bool {
		return utf8.RuneCountInString(c) > 12
	}))
	for _, name := range groups {
		for at, ok := range found[name] {
			if stacked && !ok {
				values[name][at] = 0.0
			}
		}
		s := newSeries(name, values[name])
		if p.Type == "bar" && p.Stack {
			s["stack"] = "total"
			s["itemStyle"] = map[string]any{"borderWidth": 1}
		} else if p.Type == "bar" && horizontal {
			s["itemStyle"] = map[string]any{"borderRadius": []int{0, 4, 4, 0}}
		} else if p.Type == "bar" {
			s["itemStyle"] = map[string]any{"borderRadius": []int{4, 4, 0, 0}}
		}
		series = append(series, s)
	}
	categoryAxis := map[string]any{"type": "category", "data": categories, "axisTick": map[string]any{"show": false}, "axisLabel": map[string]any{"hideOverlap": true}}
	if p.Type != "bar" {
		categoryAxis["boundaryGap"] = false
	}
	option["xAxis"] = categoryAxis
	if p.Type == "bar" {
		option["tooltip"] = map[string]any{"trigger": "axis", "confine": true, "axisPointer": map[string]any{"type": "shadow"}}
	}
	if horizontal {
		categoryAxis["inverse"] = true
		categoryAxis["axisLabel"] = map[string]any{"width": 120, "overflow": "truncate"}
		valueAxis["splitNumber"] = 3
		option["xAxis"], option["yAxis"] = valueAxis, categoryAxis
		shown.Measure = "xAxis"
	}
	option["series"] = series
	return shown, nil
}
