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
	"github.com/swaggest/jsonschema-go"
	"neui.dev"
)

// Params is what a script passes to ui.chart.
type Params struct {
	_        struct{}                          `additionalProperties:"false"`
	Type     string                            `json:"type" required:"true" enum:"line,area,bar,pie,funnel,scatter" description:"line: a trend along x; area: a trend whose amount matters, with splitBy series stacked into a total; bar: values compared by category; pie: shares of a whole; funnel: stages that narrow; scatter: how y follows x"`
	Rows     []record                          `json:"rows,omitempty" maxItems:"2000" description:"fixed records, in the order to draw them; [] when there are none"`
	Query    neui.IslandCall[struct{}, result] `json:"query,omitempty" description:"returns the records to draw, for records that change; it runs each time the chart shows"`
	Refresh  int                               `json:"refresh,omitempty" minimum:"0" maximum:"86400" description:"seconds between query runs; less than 3 counts as 3; omit for none; only with query"`
	X        string                            `json:"x" required:"true" minLength:"1" maxLength:"100" description:"field of each record that places it. line and area: categories, or dates such as 2024-05, 2024-05-17 or 2024-05-17T09:30Z, drawn on a time axis in time order when every x is one; bar: categories; pie: slice names, and past 6 slices the 5 largest stay and the rest add up as Other; funnel: stage names; scatter: numbers, and the user sees the field name as the axis label. Records with the same x and splitBy value add up, except in funnel and scatter, where each record is a stage or point of its own"`
	Y        string                            `json:"y" required:"true" minLength:"1" maxLength:"100" description:"field of each record with the measure, a number; null leaves a gap; pie and funnel take no negative values; the user sees its name, so make it readable, such as Revenue"`
	SplitBy  string                            `json:"splitBy,omitempty" minLength:"1" maxLength:"100" description:"field naming each record's series, for long records such as {month, region, sales}; past 8 series, the 7 largest by total stay and the rest add up as Other; not for pie and funnel. To compare measures of one unit kept in one record, such as {month, plan, actual}, reshape it in the script into a record per measure, such as {month, kind, value}, and split by kind"`
	Stack    bool                              `json:"stack,omitempty" description:"true stacks the splitBy bars of each x into one bar instead of placing them side by side; only for bar"`
	Format   string                            `json:"format,omitempty" enum:"number,currency,percent,duration,bytes" default:"number" description:"how values show: number; currency; percent, from a fraction: 0.12 shows 12%; duration, from seconds; bytes, from a byte count"`
	Unit     string                            `json:"unit,omitempty" minLength:"1" maxLength:"20" description:"for number, an optional suffix such as kg or req/s; for currency, the required ISO 4217 code, such as USD; none for other formats"`
	Title    string                            `json:"title,omitempty" maxLength:"120" description:"heading of the chart"`
	Subtitle string                            `json:"subtitle,omitempty" maxLength:"200" description:"line under the title, such as the period or the source"`
}

// Patch is what a script passes to update a chart.
type Patch struct {
	_     struct{}                          `additionalProperties:"false" minProperties:"1" maxProperties:"1"`
	Rows  []record                          `json:"rows,omitempty" maxItems:"2000" description:"new records, as the rows param, for a chart made with rows"`
	Query neui.IslandCall[struct{}, result] `json:"query,omitempty" description:"new function, as the query param, for a chart made with query; it runs right away, and refresh goes on with it"`
}

type result struct {
	_    struct{} `additionalProperties:"false"`
	Rows []record `json:"rows" required:"true" nullable:"false" maxItems:"2000" description:"records to draw, as the rows param"`
}

type record map[string]any

func (record) PrepareJSONSchema(s *jsonschema.Schema) error {
	value := jsonschema.Schema{}
	value.WithType(jsonschema.Type{SliceOfSimpleTypeValues: []jsonschema.SimpleType{jsonschema.String, jsonschema.Number, jsonschema.Null}})
	s.WithAdditionalProperties(value.ToSchemaOrBool())
	s.WithMaxProperties(50)
	return nil
}

type view struct {
	Option  map[string]any `json:"option,omitempty"`
	Format  string         `json:"format"`
	Unit    string         `json:"unit,omitempty"`
	Measure string         `json:"measure,omitempty"`
	Time    string         `json:"time,omitempty"`
	X       string         `json:"x"`
	Y       string         `json:"y"`
	Stale   bool           `json:"stale,omitempty"`
	Drawn   chartView      `json:"-"`
}

type chartView struct {
	Type   string   `json:"type" required:"true" description:"the chart type"`
	Title  string   `json:"title,omitempty" description:"the chart's title; absent when it has none"`
	State  string   `json:"state" required:"true" enum:"shown,out of date,no data" description:"shown: the latest data is drawn; out of date: the latest query run or update failed, so the data before it stays drawn; no data: nothing is drawn"`
	Series []string `json:"series,omitempty" description:"the series names; for pie the slices, for funnel the stages"`
	Points int      `json:"points,omitempty" description:"how many x values are drawn: categories or times, the points of a scatter, the slices of a pie, the stages of a funnel"`
	First  string   `json:"first,omitempty" description:"the first x as drawn: in time order on a time axis, else in row order, and the smallest x of a scatter; absent for pie and funnel"`
	Last   string   `json:"last,omitempty" description:"the last x as drawn, in the same order as first; absent for pie and funnel"`
}
