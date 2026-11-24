// Managed by GoX v0.4.0

//line chart.gox:1
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
	_ "embed"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"golang.org/x/text/currency"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/island"
)

//go:embed _echarts/echarts.js
var echartsJS []byte

var refreshFloor = 3 * time.Second

var backoffCeiling = 5 * time.Minute

type widget struct {
	params Params
	mu sync.Mutex
	update chan struct{}
	rows []record
	query neui.IslandCall[struct{}, result]
	stale bool
	drawn chartView
}

func New(ctx context.Context, p Params) (api.Widget[Patch], error) {
	if p.Format == "" {
		p.Format = "number"
	}
	switch {
	case (p.Rows == nil) == (p.Query == ""):
		return nil, errors.New(sourceMessage)
	case p.Refresh > 0 && p.Query == "":
		return nil, errors.New(refreshMessage)
	case p.SplitBy != "" && p.Type == "pie":
		return nil, fmt.Errorf(splitByMessage, p.Type, "slice")
	case p.SplitBy != "" && p.Type == "funnel":
		return nil, fmt.Errorf(splitByMessage, p.Type, "stage")
	case p.Stack && p.Type != "bar":
		return nil, errors.New(stackMessage)
	case p.Format == "currency" && p.Unit == "":
		return nil, errors.New(currencyUnitMessage)
	case p.Unit != "" && p.Format != "number" && p.Format != "currency":
		return nil, fmt.Errorf(unitMessage, p.Format)
	}
	if _, err := currency.ParseISO(p.Unit); p.Format == "currency" && err != nil {
		return nil, fmt.Errorf(currencyCodeMessage, p.Unit)
	}
	if p.Rows != nil {
		if _, err := compile(p, p.Rows); err != nil {
			return nil, err
		}
	}
	return &widget{params: p, rows: p.Rows, query: p.Query}, nil
}

func (w *widget) Update(ctx context.Context, patch Patch) {
	switch {
	case patch.Rows != nil && w.params.Query != "":
		neui.IslandReport(ctx, errors.New(rowsPatchMessage))
		return
	case patch.Query != "" && w.params.Query == "":
		neui.IslandReport(ctx, errors.New(queryPatchMessage))
		return
	}
	var err error
	if patch.Rows != nil {
		_, err = compile(w.params, patch.Rows)
	}
	if err != nil {
		neui.IslandReport(ctx, err)
	}
	w.mu.Lock()
	switch {
	case patch.Query != "":
		w.query = patch.Query
	case err != nil:
		w.stale = true
	default:
		w.rows, w.stale = patch.Rows, false
	}
	update := w.update
	w.mu.Unlock()
	select {
	case update <- struct{}{}:
	default:
	}
}

func (w *widget) getChart(ctx context.Context, _ struct{}) (chartView, error) {
	w.mu.Lock()
	drawn := w.drawn
	w.mu.Unlock()
	drawn.Type, drawn.Title = w.params.Type, w.params.Title
	return drawn, nil
}

func (w *widget) loop(ctx context.Context, update chan struct{}) {
	base := time.Duration(w.params.Refresh) * time.Second
	if base > 0 {
		base = max(base, refreshFloor)
	}
	period := base
	for {
		var tick <-chan time.Time
		if period > 0 {
			tick = time.After(period)
		}
		select {
		case <-ctx.Done():
			return
		case <-tick:
		case <-update:
		}
		w.mu.Lock()
		rows, query, stale := w.rows, w.query, w.stale
		w.mu.Unlock()
		shown := view{Format: w.params.Format, Unit: w.params.Unit, Stale: true}
		if w.params.Query == "" {
			shown, _ = compile(w.params, rows)
			shown.Stale = stale
		} else if res, err := query.Call(ctx, struct{}{}); err != nil && (ctx.Err() != nil || island.Ended(ctx)) {
			return
		} else if err == nil {
			if compiled, err := compile(w.params, res.Rows); err != nil {
				neui.IslandReport(ctx, fmt.Errorf(queryMessage, err))
			} else {
				shown = compiled
			}
		}
		if ctx.Err() != nil {
			return
		}
		for range doors.Call(ctx, doors.ActionEmit[any]{Name: emitName, Arg: shown}) {
		}
		if ctx.Err() != nil {
			return
		}
		w.mu.Lock()
		switch {
		case shown.Option != nil:
			w.drawn = shown.Drawn
			w.drawn.State = "shown"
			if shown.Stale {
				w.drawn.State = "out of date"
			}
		case !shown.Stale:
			w.drawn = chartView{State: "no data"}
		case w.drawn.State != "no data":
			w.drawn.State = "out of date"
		}
		w.mu.Unlock()
		if shown.Stale {
			period = min(period * 2, max(base, backoffCeiling))
		} else {
			period = base
		}
	}
}

//line chart.gox:185
func (w *widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line chart.gox:187
		w.mu.Lock()
	update := make(chan struct{}, 1)
	w.update = update
	rows, query, stale := w.rows, w.query, w.stale
	w.mu.Unlock()
	shown := view{Format: w.params.Format, Unit: w.params.Unit, Stale: true}
	if w.params.Query == "" {
		shown, _ = compile(w.params, rows)
		shown.Stale = stale
	} else if res, err := query.Call(ctx, struct{}{}); err == nil {
		if compiled, err := compile(w.params, res.Rows); err != nil {
			neui.IslandReport(ctx, fmt.Errorf(queryMessage, err))
		} else {
			shown = compiled
		}
	}
	drawn := chartView{State: "no data"}
	if shown.Option != nil {
		drawn = shown.Drawn
		drawn.State = "shown"
		if shown.Stale {
			drawn.State = "out of date"
		}
	}
	w.mu.Lock()
	w.drawn = drawn
	w.mu.Unlock()

//line chart.gox:215
		__e = __c.Any(neui.Tool[struct{}, chartView]{Name: getChartName, Desc: getChartDesc, Handler: w.getChart}); if __e != nil { return }
//line chart.gox:216
		__e = (new(doors.Door)).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line chart.gox:216
				__e = __c.Set("class", "neisland-chart-card"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line chart.gox:217
				if w.params.Title != "" || w.params.Subtitle != "" {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line chart.gox:218
						__e = __c.Set("class", "neisland-chart-head"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line chart.gox:219
						if w.params.Title != "" {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line chart.gox:220
								__e = __c.Set("class", "neisland-chart-title"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line chart.gox:220
								__e = __c.Any(w.params.Title); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
//line chart.gox:222
						if w.params.Subtitle != "" {
							__e = __c.Init("p"); if __e != nil { return }
							{
//line chart.gox:223
								__e = __c.Set("class", "neisland-chart-subtitle"); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line chart.gox:223
								__e = __c.Any(w.params.Subtitle); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
						}
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Init("div"); if __e != nil { return }
				{
//line chart.gox:227
					__e = __c.Set("class", "neisland-chart-box"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("div"); if __e != nil { return }
					{
//line chart.gox:228
						__e = __c.Set("class", "neisland-chart-plot"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("p"); if __e != nil { return }
					{
//line chart.gox:229
						__e = __c.Set("class", "neisland-chart-empty"); if __e != nil { return }
						__e = __c.Set("hidden", true); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("No data"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("p"); if __e != nil { return }
				{
//line chart.gox:231
					__e = __c.Set("class", "neisland-chart-stale"); if __e != nil { return }
//line chart.gox:231
					__e = __c.Set("role", "status"); if __e != nil { return }
					__e = __c.Set("hidden", true); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Text("Out of date"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("span"); if __e != nil { return }
				{
//line chart.gox:232
					__e = __c.Set("class", "neisland-chart-palette"); if __e != nil { return }
//line chart.gox:232
					__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
					__e = __c.Init("i"); if __e != nil { return }
					{
						__e = __c.Submit(); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
				__e = __c.Init("script"); if __e != nil { return }
				{
//line chart.gox:234
					__e = __c.Set("data:echarts", doors.ResourceBytes(echartsJS).Script().Module().Raw()); if __e != nil { return }
//line chart.gox:235
					__e = __c.Set("data:chart", shown); if __e != nil { return }
//line chart.gox:236
					__e = __c.Modify(doors.AHook[struct{}]{
				Name: "ready",
				On: func(ctx context.Context, _ doors.RequestHook[struct{}]) (any, bool) {
					go w.loop(doors.DetachedContext(ctx), update)
					return nil, true
				},
			}); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Raw("const root = document.currentScript.parentElement\n\t\t\tconst plot = root.querySelector(\".neisland-chart-plot\")\n\t\t\tconst empty = root.querySelector(\".neisland-chart-empty\")\n\t\t\tconst stale = root.querySelector(\".neisland-chart-stale\")\n\t\t\tconst palette = root.querySelector(\".neisland-chart-palette\")\n\t\t\tconst still = matchMedia(\"(prefers-reduced-motion: reduce)\")\n\t\t\tconst echarts = await import($data(\"echarts\"))\n\t\t\tconst pixel = document\n\t\t\t\t.createElement(\"canvas\")\n\t\t\t\t.getContext(\"2d\", { willReadFrequently: true })\n\t\t\tconst rgba = (color, alpha) => {\n\t\t\t\tpixel.clearRect(0, 0, 1, 1)\n\t\t\t\tpixel.fillStyle = \"#000\"\n\t\t\t\tpixel.fillStyle = color\n\t\t\t\tpixel.fillRect(0, 0, 1, 1)\n\t\t\t\tconst [r, g, b, a] = pixel.getImageData(0, 0, 1, 1).data\n\t\t\t\treturn `rgba(${r}, ${g}, ${b}, ${((a / 255) * alpha).toFixed(3)})`\n\t\t\t}\n\t\t\tconst escape = (text) =>\n\t\t\t\tString(text).replace(/[&<>\"']/g, (c) => `&#${c.charCodeAt(0)};`)\n\t\t\tconst formatter = (format, unit, compact) => {\n\t\t\t\tconst number = (options) => new Intl.NumberFormat(undefined, options)\n\t\t\t\tlet show\n\t\t\t\tif (format === \"currency\") {\n\t\t\t\t\tconst f = number(\n\t\t\t\t\t\tcompact\n\t\t\t\t\t\t\t? {\n\t\t\t\t\t\t\t\t\tstyle: \"currency\",\n\t\t\t\t\t\t\t\t\tcurrency: unit,\n\t\t\t\t\t\t\t\t\tnotation: \"compact\",\n\t\t\t\t\t\t\t\t\tmaximumFractionDigits: 1,\n\t\t\t\t\t\t\t  }\n\t\t\t\t\t\t\t: { style: \"currency\", currency: unit },\n\t\t\t\t\t)\n\t\t\t\t\tshow = (v) => f.format(v)\n\t\t\t\t} else if (format === \"percent\") {\n\t\t\t\t\tconst f = number({ style: \"percent\", maximumFractionDigits: 1 })\n\t\t\t\t\tshow = (v) => f.format(v)\n\t\t\t\t} else if (format === \"bytes\") {\n\t\t\t\t\tconst f = number({ maximumFractionDigits: compact ? 1 : 2 })\n\t\t\t\t\tconst units = [\"B\", \"KB\", \"MB\", \"GB\", \"TB\", \"PB\"]\n\t\t\t\t\tshow = (v) => {\n\t\t\t\t\t\tlet size = Math.abs(v)\n\t\t\t\t\t\tlet step = 0\n\t\t\t\t\t\twhile (size >= 1024 && step < units.length - 1) {\n\t\t\t\t\t\t\tsize /= 1024\n\t\t\t\t\t\t\tstep++\n\t\t\t\t\t\t}\n\t\t\t\t\t\treturn `${f.format(Math.sign(v) * size)} ${units[step]}`\n\t\t\t\t\t}\n\t\t\t\t} else if (format === \"duration\") {\n\t\t\t\t\tconst steps = [\n\t\t\t\t\t\t[\"day\", 86400],\n\t\t\t\t\t\t[\"hour\", 3600],\n\t\t\t\t\t\t[\"minute\", 60],\n\t\t\t\t\t\t[\"second\", 1],\n\t\t\t\t\t].map(([unit, size]) => [\n\t\t\t\t\t\tsize,\n\t\t\t\t\t\tnumber({\n\t\t\t\t\t\t\tstyle: \"unit\",\n\t\t\t\t\t\t\tunit,\n\t\t\t\t\t\t\tunitDisplay: compact ? \"short\" : \"narrow\",\n\t\t\t\t\t\t\tmaximumFractionDigits: compact ? 1 : 0,\n\t\t\t\t\t\t}),\n\t\t\t\t\t])\n\t\t\t\t\tconst seconds = number({\n\t\t\t\t\t\tstyle: \"unit\",\n\t\t\t\t\t\tunit: \"second\",\n\t\t\t\t\t\tunitDisplay: \"short\",\n\t\t\t\t\t\tmaximumFractionDigits: 2,\n\t\t\t\t\t})\n\t\t\t\t\tshow = (v) => {\n\t\t\t\t\t\tif (compact) {\n\t\t\t\t\t\t\tconst [size, f] =\n\t\t\t\t\t\t\t\tsteps.find(([size]) => Math.abs(v) >= size) ?? steps[3]\n\t\t\t\t\t\t\treturn f.format(v / size)\n\t\t\t\t\t\t}\n\t\t\t\t\t\tif (Math.abs(v) < 60) return seconds.format(v)\n\t\t\t\t\t\tlet rest = Math.round(Math.abs(v))\n\t\t\t\t\t\tconst parts = []\n\t\t\t\t\t\tfor (const [size, f] of steps) {\n\t\t\t\t\t\t\tconst count = Math.floor(rest / size)\n\t\t\t\t\t\t\trest -= count * size\n\t\t\t\t\t\t\tif (count) parts.push(f.format(count))\n\t\t\t\t\t\t}\n\t\t\t\t\t\treturn (v < 0 ? \"-\" : \"\") + parts.join(\" \")\n\t\t\t\t\t}\n\t\t\t\t} else {\n\t\t\t\t\tconst f = number(\n\t\t\t\t\t\tcompact\n\t\t\t\t\t\t\t? { notation: \"compact\", maximumFractionDigits: 1 }\n\t\t\t\t\t\t\t: { maximumFractionDigits: 2 },\n\t\t\t\t\t)\n\t\t\t\t\tshow = (v) => (unit ? `${f.format(v)} ${unit}` : f.format(v))\n\t\t\t\t}\n\t\t\t\treturn (v) => (v == null || Number.isNaN(v) ? \"–\" : show(v))\n\t\t\t}\n\t\t\tconst slots = new Map()\n\t\t\tlet chart = null\n\t\t\tlet shape = \"\"\n\t\t\tlet last = null\n\t\t\tconst render = () => {\n\t\t\t\tconst spec = last\n\t\t\t\tconst style = getComputedStyle(root)\n\t\t\t\tconst text = rgba(style.color, 1)\n\t\t\t\tconst muted = rgba(style.color, 0.72)\n\t\t\t\tconst rule = rgba(style.color, 0.24)\n\t\t\t\tconst faint = rgba(style.color, 0.1)\n\t\t\t\tconst surface = rgba(getComputedStyle(palette).backgroundColor, 1)\n\t\t\t\tconst colors = Array.from(palette.children, (e) =>\n\t\t\t\t\trgba(getComputedStyle(e).color, 1),\n\t\t\t\t)\n\t\t\t\tconst full = formatter(spec.format, spec.unit, false)\n\t\t\t\tconst compact = formatter(spec.format, spec.unit, true)\n\t\t\t\tconst plain = formatter(\"number\", \"\", false)\n\t\t\t\tconst plainCompact = formatter(\"number\", \"\", true)\n\t\t\t\tconst share = formatter(\"percent\", \"\", false)\n\t\t\t\tconst zone = spec.option.useUTC ? { timeZone: \"UTC\" } : {}\n\t\t\t\tconst when = new Intl.DateTimeFormat(undefined, {\n\t\t\t\t\t...(spec.time === \"month\"\n\t\t\t\t\t\t? { year: \"numeric\", month: \"short\" }\n\t\t\t\t\t\t: { dateStyle: \"medium\" }),\n\t\t\t\t\t...(spec.time === \"datetime\" ? { timeStyle: \"short\" } : {}),\n\t\t\t\t\t...zone,\n\t\t\t\t})\n\t\t\t\tconst day = new Intl.DateTimeFormat(undefined, {\n\t\t\t\t\tmonth: \"short\",\n\t\t\t\t\tday: \"numeric\",\n\t\t\t\t\t...zone,\n\t\t\t\t})\n\t\t\t\tconst month = new Intl.DateTimeFormat(undefined, { month: \"short\", ...zone })\n\t\t\t\tconst year = new Intl.DateTimeFormat(undefined, {\n\t\t\t\t\tmonth: \"short\",\n\t\t\t\t\tyear: \"numeric\",\n\t\t\t\t\t...zone,\n\t\t\t\t})\n\t\t\t\tconst option = structuredClone(spec.option)\n\t\t\t\tconst series = option.series\n\t\t\t\tconst kind = series[0]?.type\n\t\t\t\tconst names =\n\t\t\t\t\tkind === \"pie\"\n\t\t\t\t\t\t? series[0].data.map((d) => d.name)\n\t\t\t\t\t\t: series.map((s) => s.name)\n\t\t\t\tfor (const name of slots.keys()) {\n\t\t\t\t\tif (!names.includes(name)) slots.delete(name)\n\t\t\t\t}\n\t\t\t\tconst used = new Set(slots.values())\n\t\t\t\tfor (const name of names) {\n\t\t\t\t\tif (slots.has(name)) continue\n\t\t\t\t\tlet slot = 0\n\t\t\t\t\twhile (used.has(slot)) slot++\n\t\t\t\t\tslots.set(name, slot)\n\t\t\t\t\tused.add(slot)\n\t\t\t\t}\n\t\t\t\tconst color = (name) => colors[slots.get(name) % colors.length]\n\t\t\t\toption.backgroundColor = \"transparent\"\n\t\t\t\toption.animation = !still.matches\n\t\t\t\toption.textStyle = {\n\t\t\t\t\tcolor: muted,\n\t\t\t\t\tfontFamily: style.fontFamily,\n\t\t\t\t\tfontSize: 12,\n\t\t\t\t}\n\t\t\t\tfor (const axis of [option.xAxis, option.yAxis]) {\n\t\t\t\t\tif (!axis) continue\n\t\t\t\t\taxis.axisLine = { ...axis.axisLine, lineStyle: { color: rule } }\n\t\t\t\t\taxis.splitLine = { ...axis.splitLine, lineStyle: { color: faint } }\n\t\t\t\t\taxis.axisLabel = { ...axis.axisLabel, color: muted }\n\t\t\t\t\taxis.nameTextStyle = { color: muted }\n\t\t\t\t\tif (axis.type === \"value\")\n\t\t\t\t\t\taxis.axisLabel.formatter =\n\t\t\t\t\t\t\taxis === option[spec.measure] ? compact : plainCompact\n\t\t\t\t\tif (axis.type === \"time\" && spec.time === \"date\")\n\t\t\t\t\t\taxis.axisLabel.formatter = (v) => day.format(v)\n\t\t\t\t\tif (axis.type === \"time\" && spec.time === \"month\")\n\t\t\t\t\t\taxis.axisLabel.formatter = (v, i) =>\n\t\t\t\t\t\t\t(i === 0 ||\n\t\t\t\t\t\t\tnew Date(v)[spec.option.useUTC ? \"getUTCMonth\" : \"getMonth\"]() === 0\n\t\t\t\t\t\t\t\t? year\n\t\t\t\t\t\t\t\t: month\n\t\t\t\t\t\t\t).format(v)\n\t\t\t\t}\n\t\t\t\tif (option.legend) {\n\t\t\t\t\toption.legend = {\n\t\t\t\t\t\t...option.legend,\n\t\t\t\t\t\ttextStyle: { color: muted },\n\t\t\t\t\t\tinactiveColor: rgba(style.color, 0.3),\n\t\t\t\t\t\tpageTextStyle: { color: muted },\n\t\t\t\t\t\tpageIconColor: muted,\n\t\t\t\t\t\tpageIconInactiveColor: faint,\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\toption.tooltip = {\n\t\t\t\t\t...option.tooltip,\n\t\t\t\t\tbackgroundColor: surface,\n\t\t\t\t\tborderColor: rule,\n\t\t\t\t\tborderWidth: 1,\n\t\t\t\t\tpadding: [6, 10],\n\t\t\t\t\ttextStyle: { color: text, fontFamily: style.fontFamily, fontSize: 12 },\n\t\t\t\t\textraCssText: \"box-shadow: none;\",\n\t\t\t\t}\n\t\t\t\tif (option.tooltip.axisPointer) {\n\t\t\t\t\toption.tooltip.axisPointer = {\n\t\t\t\t\t\t...option.tooltip.axisPointer,\n\t\t\t\t\t\tlineStyle: { color: rule },\n\t\t\t\t\t\tshadowStyle: { color: rgba(style.color, 0.06) },\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tconst line = (marker, name, value) =>\n\t\t\t\t\t`<div style=\"display:flex;gap:16px;justify-content:space-between\"><span>${marker}${escape(\n\t\t\t\t\t\tname,\n\t\t\t\t\t)}</span><b style=\"font-variant-numeric:tabular-nums\">${escape(\n\t\t\t\t\t\tvalue,\n\t\t\t\t\t)}</b></div>`\n\t\t\t\tif (option.tooltip.trigger === \"axis\") {\n\t\t\t\t\toption.tooltip.formatter = (items) => {\n\t\t\t\t\t\tconst head = spec.time\n\t\t\t\t\t\t\t? when.format(items[0].axisValue)\n\t\t\t\t\t\t\t: items[0].axisValueLabel\n\t\t\t\t\t\treturn (\n\t\t\t\t\t\t\t`<div style=\"margin-bottom:4px\">${escape(head)}</div>` +\n\t\t\t\t\t\t\titems\n\t\t\t\t\t\t\t\t.map((p) =>\n\t\t\t\t\t\t\t\t\tline(\n\t\t\t\t\t\t\t\t\t\tp.marker,\n\t\t\t\t\t\t\t\t\t\tp.seriesName,\n\t\t\t\t\t\t\t\t\t\tfull(Array.isArray(p.value) ? p.value[1] : p.value),\n\t\t\t\t\t\t\t\t\t),\n\t\t\t\t\t\t\t\t)\n\t\t\t\t\t\t\t\t.join(\"\")\n\t\t\t\t\t\t)\n\t\t\t\t\t}\n\t\t\t\t} else if (kind === \"pie\") {\n\t\t\t\t\toption.tooltip.formatter = (p) =>\n\t\t\t\t\t\tline(p.marker, p.name, `${full(p.value)} · ${share(p.percent / 100)}`)\n\t\t\t\t\tseries[0].label = {\n\t\t\t\t\t\t...series[0].label,\n\t\t\t\t\t\tformatter: (p) => `${p.name}\\n${share(p.percent / 100)}`,\n\t\t\t\t\t}\n\t\t\t\t} else if (kind === \"funnel\") {\n\t\t\t\t\tconst data = series[0].data\n\t\t\t\t\tconst first = data[0]?.value\n\t\t\t\t\toption.tooltip.formatter = (p) => {\n\t\t\t\t\t\tconst before = data[p.dataIndex - 1]?.value\n\t\t\t\t\t\treturn (\n\t\t\t\t\t\t\tline(p.marker, p.name, full(p.value)) +\n\t\t\t\t\t\t\tline(\"\", \"of first\", first ? share(p.value / first) : \"–\") +\n\t\t\t\t\t\t\t(p.dataIndex > 0\n\t\t\t\t\t\t\t\t? line(\"\", \"of previous\", before ? share(p.value / before) : \"–\")\n\t\t\t\t\t\t\t\t: \"\")\n\t\t\t\t\t\t)\n\t\t\t\t\t}\n\t\t\t\t\tseries[0].label = {\n\t\t\t\t\t\t...series[0].label,\n\t\t\t\t\t\tformatter: (p) =>\n\t\t\t\t\t\t\t`${p.name}\\n${compact(p.value)} · ${\n\t\t\t\t\t\t\t\tfirst ? share(p.value / first) : \"–\"\n\t\t\t\t\t\t\t}`,\n\t\t\t\t\t}\n\t\t\t\t} else if (kind === \"scatter\") {\n\t\t\t\t\toption.tooltip.formatter = (p) =>\n\t\t\t\t\t\t`<div style=\"margin-bottom:4px\">${p.marker}${escape(\n\t\t\t\t\t\t\tp.seriesName,\n\t\t\t\t\t\t)}</div>` +\n\t\t\t\t\t\tline(\"\", spec.x, plain(p.value[0])) +\n\t\t\t\t\t\tline(\"\", spec.y, full(p.value[1]))\n\t\t\t\t}\n\t\t\t\tfor (const s of series) {\n\t\t\t\t\tconst c = kind === \"funnel\" ? colors[0] : color(s.name)\n\t\t\t\t\ts.itemStyle = { ...s.itemStyle, borderColor: surface }\n\t\t\t\t\tif (kind === \"pie\") {\n\t\t\t\t\t\ts.data = s.data.map((d) => ({\n\t\t\t\t\t\t\t...d,\n\t\t\t\t\t\t\titemStyle: { color: color(d.name) },\n\t\t\t\t\t\t}))\n\t\t\t\t\t} else {\n\t\t\t\t\t\ts.itemStyle.color = c\n\t\t\t\t\t}\n\t\t\t\t\tif (s.type === \"line\") s.lineStyle = { ...s.lineStyle, color: c }\n\t\t\t\t\ts.label = { ...s.label, color: text }\n\t\t\t\t\ts.labelLine = { ...s.labelLine, lineStyle: { color: rule } }\n\t\t\t\t}\n\t\t\t\tconst listed = (items, newest) =>\n\t\t\t\t\t(items.length <= 10\n\t\t\t\t\t\t? items\n\t\t\t\t\t\t: newest\n\t\t\t\t\t\t\t? [`${items.length - 10} earlier`, ...items.slice(-10)]\n\t\t\t\t\t\t\t: [...items.slice(0, 10), `${items.length - 10} more`]\n\t\t\t\t\t).join(\"; \")\n\t\t\t\tif (kind === \"pie\") {\n\t\t\t\t\tconst data = series[0].data\n\t\t\t\t\tconst total = data.reduce((sum, d) => sum + d.value, 0)\n\t\t\t\t\toption.aria.label = {\n\t\t\t\t\t\tdescription: `Pie chart, ${data.length} slices: ${listed(\n\t\t\t\t\t\t\tdata.map(\n\t\t\t\t\t\t\t\t(d) => `${d.name} is ${full(d.value)} (${share(total ? d.value / total : 0)})`,\n\t\t\t\t\t\t\t),\n\t\t\t\t\t\t)}.`,\n\t\t\t\t\t}\n\t\t\t\t} else if (kind === \"funnel\") {\n\t\t\t\t\tconst data = series[0].data\n\t\t\t\t\tconst first = data[0]?.value\n\t\t\t\t\toption.aria.label = {\n\t\t\t\t\t\tdescription: `Funnel chart, ${data.length} stages: ${listed(\n\t\t\t\t\t\t\tdata.map(\n\t\t\t\t\t\t\t\t(d) =>\n\t\t\t\t\t\t\t\t\t`${d.name} is ${full(d.value)} (${\n\t\t\t\t\t\t\t\t\t\tfirst ? share(d.value / first) : \"–\"\n\t\t\t\t\t\t\t\t\t} of first)`,\n\t\t\t\t\t\t\t),\n\t\t\t\t\t\t)}.`,\n\t\t\t\t\t}\n\t\t\t\t} else {\n\t\t\t\t\tconst categories = [option.xAxis, option.yAxis].find(\n\t\t\t\t\t\t(axis) => axis.type === \"category\",\n\t\t\t\t\t)?.data\n\t\t\t\t\tconst type = kind === \"line\" && series[0].areaStyle ? \"area\" : kind\n\t\t\t\t\toption.aria.label = {\n\t\t\t\t\t\tdescription:\n\t\t\t\t\t\t\t`${type[0].toUpperCase()}${type.slice(1)} chart, ${series.length} series.` +\n\t\t\t\t\t\t\tseries\n\t\t\t\t\t\t\t\t.map(\n\t\t\t\t\t\t\t\t\t(s) =>\n\t\t\t\t\t\t\t\t\t\t` ${s.name}: ${listed(\n\t\t\t\t\t\t\t\t\t\t\ts.data.map((d, i) =>\n\t\t\t\t\t\t\t\t\t\t\t\tkind === \"scatter\"\n\t\t\t\t\t\t\t\t\t\t\t\t\t? `${spec.x} ${plain(d[0])}, ${spec.y} ${full(d[1])}`\n\t\t\t\t\t\t\t\t\t\t\t\t\t: spec.time\n\t\t\t\t\t\t\t\t\t\t\t\t\t\t? `${when.format(d[0])} is ${full(d[1])}`\n\t\t\t\t\t\t\t\t\t\t\t\t\t\t: `${categories[i]} is ${full(d)}`,\n\t\t\t\t\t\t\t\t\t\t\t),\n\t\t\t\t\t\t\t\t\t\t\t!!spec.time,\n\t\t\t\t\t\t\t\t\t\t)}.`,\n\t\t\t\t\t\t\t\t)\n\t\t\t\t\t\t\t\t.join(\"\"),\n\t\t\t\t\t}\n\t\t\t\t}\n\t\t\t\tconst next = JSON.stringify([\n\t\t\t\t\tkind,\n\t\t\t\t\toption.xAxis?.type,\n\t\t\t\t\toption.yAxis?.type,\n\t\t\t\t\tseries.length,\n\t\t\t\t\tspec.time,\n\t\t\t\t\tspec.option.useUTC,\n\t\t\t\t])\n\t\t\t\tif (!chart) {\n\t\t\t\t\tchart = echarts.init(plot, null, { renderer: \"svg\" })\n\t\t\t\t\tchart.on(\"mousemove\", \"series\", () => chart.getZr().setCursorStyle(\"default\"))\n\t\t\t\t}\n\t\t\t\tchart.setOption(\n\t\t\t\t\toption,\n\t\t\t\t\tnext === shape ? { replaceMerge: [\"series\"] } : { notMerge: true },\n\t\t\t\t)\n\t\t\t\tshape = next\n\t\t\t}\n\t\t\tconst draw = (spec) => {\n\t\t\t\tif (spec.option) {\n\t\t\t\t\tlast = spec\n\t\t\t\t\trender()\n\t\t\t\t} else if (!spec.stale) {\n\t\t\t\t\tlast = null\n\t\t\t\t\tshape = \"\"\n\t\t\t\t\tchart?.clear()\n\t\t\t\t}\n\t\t\t\tempty.hidden = !!last\n\t\t\t\tstale.hidden = !last || !spec.stale\n\t\t\t}\n\t\t\tconst resize = new ResizeObserver(() => chart?.resize())\n\t\t\tlet queued = false\n\t\t\tconst repaint = () => {\n\t\t\t\tif (queued) return\n\t\t\t\tqueued = true\n\t\t\t\trequestAnimationFrame(() => {\n\t\t\t\t\tqueued = false\n\t\t\t\t\tif (last) render()\n\t\t\t\t})\n\t\t\t}\n\t\t\tresize.observe(plot)\n\t\t\tpalette.addEventListener(\"transitionend\", repaint)\n\t\t\t$sys.clean(() => {\n\t\t\t\tresize.disconnect()\n\t\t\t\tchart?.dispose()\n\t\t\t})\n\t\t\tdraw($data(\"chart\"))\n\t\t\t$on(\"neisland_chart\", draw)\n\t\t\tawait $hook(\"ready\", {})"); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
	return })
//line chart.gox:630
}
