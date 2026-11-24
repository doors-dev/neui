// Managed by GoX v0.4.0

//line stat.gox:1
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

package stat

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"golang.org/x/text/currency"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/internal/island"
)

var refreshFloor = 3 * time.Second

var backoffCeiling = 5 * time.Minute

type widget struct {
	params Params
	currency currency.Unit
	scale int
	refresh time.Duration
	mu sync.Mutex
	query neui.IslandCall[struct{}, data]
	value float64
	reference *float64
	door *doors.Door
	failed bool
	reset chan struct{}
	shown statView
}

func New(ctx context.Context, p Params) (api.Widget[Patch], error) {
	switch {
	case (p.Value == nil) == (p.Query == ""):
		return nil, errors.New(valueOrQueryMessage)
	case p.Refresh != 0 && p.Query == "":
		return nil, errors.New(refreshWithoutQueryMessage)
	case p.Reference != nil && p.Value == nil:
		return nil, errors.New(referenceWithoutValueMessage)
	case p.Format == "currency" && p.Unit == "":
		return nil, errors.New(currencyUnitMessage)
	case p.Unit != "" && p.Format != "" && p.Format != "number" && p.Format != "currency":
		return nil, fmt.Errorf(unitMessage, p.Format)
	}
	if p.Format == "" {
		p.Format = "number"
	}
	if p.Good == "" {
		p.Good = "up"
	}
	w := &widget{params: p, query: p.Query, reference: p.Reference, reset: make(chan struct{}, 1), shown: statView{Label: p.Label, State: "no data"}}
	if p.Format == "currency" {
		code, err := currency.ParseISO(p.Unit)
		if err != nil || code == currency.XXX {
			return nil, fmt.Errorf(currencyCodeMessage, p.Unit)
		}
		w.currency = code
		w.scale, _ = currency.Standard.Rounding(code)
	}
	if w.refresh = time.Duration(p.Refresh) * time.Second; w.refresh > 0 {
		w.refresh = max(w.refresh, refreshFloor)
	}
	if p.Value != nil {
		w.value = *p.Value
	}
	return w, nil
}

func (w *widget) Update(ctx context.Context, patch Patch) {
	if w.params.Query != "" && (patch.Query == "" || patch.Value != nil || patch.Reference != nil) {
		neui.IslandReport(ctx, errors.New(queryPatchMessage))
		return
	}
	if w.params.Query == "" && (patch.Query != "" || patch.Value == nil) {
		neui.IslandReport(ctx, errors.New(valuePatchMessage))
		return
	}
	w.mu.Lock()
	if patch.Query != "" {
		w.query = patch.Query
	} else {
		w.value, w.reference = *patch.Value, patch.Reference
	}
	door := w.door
	w.mu.Unlock()
	if door != nil {
		door.Reload(ctx)
	}
	if patch.Query != "" {
		select {
		case w.reset <- struct{}{}:
		default:
		}
	}
}

func (w *widget) getStat(ctx context.Context, _ struct{}) (statView, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.shown, nil
}

func (w *widget) loop(ctx context.Context, door *doors.Door) {
	delay := w.refresh
	for {
		select {
		case <-ctx.Done():
			return
		case <-w.reset:
			delay = w.refresh
			continue
		case <-time.After(delay):
		}
		if island.Ended(ctx) {
			return
		}
		<-door.Reload(ctx)
		w.mu.Lock()
		failed := w.failed
		w.mu.Unlock()
		if failed {
			delay = max(w.refresh, min(2 * delay, backoffCeiling))
		} else {
			delay = w.refresh
		}
	}
}

//line stat.gox:151
func (w *widget) Main() gox.Elem {
	return gox.Elem(func(__c gox.Cursor) (__e error) {
		ctx := __c.Context(); _ = ctx
//line stat.gox:153
		door := new(doors.Door)
	w.mu.Lock()
	w.door = door
	w.mu.Unlock()

//line stat.gox:158
		__e = __c.Any(neui.Tool[struct{}, statView]{Name: getStatName, Desc: getStatDesc, Handler: w.getStat}); if __e != nil { return }
//line stat.gox:159
		__e = (door).Proxy(__c, gox.Elem(func(__c gox.Cursor) (__e error) {
			ctx := __c.Context(); _ = ctx
			__e = __c.Init("div"); if __e != nil { return }
			{
//line stat.gox:159
				__e = __c.Set("class", "neisland-stat"); if __e != nil { return }
				__e = __c.Submit(); if __e != nil { return }
//line stat.gox:161
				tag, _ := doors.DeriveBeam(doors.BrowserLocale(ctx), func(locale doors.Locale) language.Tag {
			return locale.Language
		}).Effect(ctx)
		if tag == language.Und {
			tag = language.AmericanEnglish
		}
		format := numberFormat{kind: w.params.Format, unit: w.params.Unit, scale: w.scale, printer: message.NewPrinter(tag)}
		if w.params.Format == "currency" {
			format.symbol = format.printer.Sprint(currency.Symbol(w.currency))
		}
		w.mu.Lock()
		query, value, reference := w.query, w.value, w.reference
		w.mu.Unlock()
		shown := true
		if w.params.Query != "" {
			result, err := query.Call(ctx, struct{}{})
			w.mu.Lock()
			if ctx.Err() == nil {
				w.failed = err != nil
			}
			w.mu.Unlock()
			shown, value, reference = err == nil, result.Value, result.Reference
		}
		view := statView{Label: w.params.Label, State: "no data"}
		if shown {
			view.State, view.Value, view.Reference = "shown", &value, reference
			if reference != nil && *reference != 0 {
				change := (value - *reference) / math.Abs(*reference)
				view.Change = &change
			}
		}
		w.mu.Lock()
		if ctx.Err() == nil {
			w.shown = view
		}
		w.mu.Unlock()

				__e = __c.Init("div"); if __e != nil { return }
				{
//line stat.gox:198
					__e = __c.Set("class", "neisland-stat-head"); if __e != nil { return }
					__e = __c.Submit(); if __e != nil { return }
					__e = __c.Init("span"); if __e != nil { return }
					{
//line stat.gox:199
						__e = __c.Set("class", "neisland-stat-label"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line stat.gox:199
						__e = __c.Any(w.params.Label); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				}
				__e = __c.Close(); if __e != nil { return }
//line stat.gox:201
				if !shown {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line stat.gox:202
						__e = __c.Set("class", "neisland-stat-value neisland-stat-empty"); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
						__e = __c.Text("No data"); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
				} else  {
					__e = __c.Init("div"); if __e != nil { return }
					{
//line stat.gox:204
						__e = __c.Set("class", "neisland-stat-value"); if __e != nil { return }
//line stat.gox:204
						__e = __c.Set("title", format.exact(value)); if __e != nil { return }
						__e = __c.Submit(); if __e != nil { return }
//line stat.gox:204
						__e = __c.Any(format.show(value)); if __e != nil { return }
					}
					__e = __c.Close(); if __e != nil { return }
//line stat.gox:205
					if reference != nil {
//line stat.gox:207
						text, direction := format.change(value, *reference)
				tone := ""
				arrow := ""
				switch {
				case direction > 0:
					arrow = "▲"
				case direction < 0:
					arrow = "▼"
				}
				switch {
				case direction == 0:
				case (direction > 0) == (w.params.Good == "up"):
					tone = " neisland-stat-good"
				default:
					tone = " neisland-stat-bad"
				}

						__e = __c.Init("div"); if __e != nil { return }
						{
//line stat.gox:224
							__e = __c.Set("class", "neisland-stat-change"); if __e != nil { return }
							__e = __c.Submit(); if __e != nil { return }
							__e = __c.Init("span"); if __e != nil { return }
							{
//line stat.gox:225
								__e = __c.Set("class", "neisland-stat-delta" + tone); if __e != nil { return }
								__e = __c.Submit(); if __e != nil { return }
//line stat.gox:226
								if arrow != "" {
									__e = __c.Init("span"); if __e != nil { return }
									{
//line stat.gox:227
										__e = __c.Set("class", "neisland-stat-arrow"); if __e != nil { return }
//line stat.gox:227
										__e = __c.Set("aria-hidden", "true"); if __e != nil { return }
										__e = __c.Submit(); if __e != nil { return }
//line stat.gox:227
										__e = __c.Any(arrow); if __e != nil { return }
									}
									__e = __c.Close(); if __e != nil { return }
//line stat.gox:228
									__e = __c.Any(" "); if __e != nil { return }
								}
//line stat.gox:230
								__e = __c.Any(text); if __e != nil { return }
							}
							__e = __c.Close(); if __e != nil { return }
//line stat.gox:232
							if w.params.ReferenceLabel != "" {
//line stat.gox:233
								__e = __c.Any(" "); if __e != nil { return }
								__e = __c.Init("span"); if __e != nil { return }
								{
//line stat.gox:234
									__e = __c.Set("class", "neisland-stat-reference"); if __e != nil { return }
									__e = __c.Submit(); if __e != nil { return }
//line stat.gox:234
									__e = __c.Any(w.params.ReferenceLabel); if __e != nil { return }
								}
								__e = __c.Close(); if __e != nil { return }
							}
						}
						__e = __c.Close(); if __e != nil { return }
					}
				}
			}
			__e = __c.Close(); if __e != nil { return }
		return })); if __e != nil { return }
//line stat.gox:240
		if w.refresh > 0 {
//line stat.gox:241
			__e = __c.Any(doors.Go(func(ctx context.Context) {
			w.loop(ctx, door)
		})); if __e != nil { return }
		}
	return })
//line stat.gox:245
}
