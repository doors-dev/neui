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
	"neui.dev"
)

// Params is what a script passes to ui.stat.
type Params struct {
	_              struct{}                        `additionalProperties:"false"`
	Label          string                          `json:"label" required:"true" minLength:"1" maxLength:"80" description:"what the number is, such as Revenue or Open tickets"`
	Value          *float64                        `json:"value,omitempty" description:"the number, when the script already has it; it changes only when you update the tile"`
	Reference      *float64                        `json:"reference,omitempty" description:"the number that value is compared with: the previous period, the same period last year, or a target; the change from it shows in %; only with value"`
	Query          neui.IslandCall[struct{}, data] `json:"query,omitempty" description:"returns the number and its reference, for a number that must stay current; it runs each time the tile shows"`
	Refresh        int                             `json:"refresh,omitempty" minimum:"0" maximum:"86400" description:"seconds between query runs; less than 3 counts as 3; omit for none; only with query"`
	ReferenceLabel string                          `json:"referenceLabel,omitempty" maxLength:"40" description:"says which reference the change is against, such as vs last month or vs target"`
	Format         string                          `json:"format,omitempty" enum:"number,currency,percent,duration,bytes" default:"number" description:"how the number reads: number; currency; percent, from a fraction: 0.12 shows 12%; duration, from seconds: 3900 shows 1h 5m; bytes, from a byte count: 1536 shows 1.5 KB"`
	Unit           string                          `json:"unit,omitempty" maxLength:"20" description:"with format number, a suffix such as users or orders; with currency, required: its ISO 4217 code, such as USD or EUR; the other formats ignore it"`
	Good           string                          `json:"good,omitempty" enum:"up,down" default:"up" description:"which direction of change is good news: up for revenue, down for costs or error rates"`
}

type data struct {
	_         struct{} `additionalProperties:"false"`
	Value     float64  `json:"value" required:"true" description:"the number to show"`
	Reference *float64 `json:"reference,omitempty" description:"the number that value is compared with, as the reference param; omit for none"`
}

// Patch is what a script passes to update a stat view.
type Patch struct {
	_         struct{}                        `additionalProperties:"false"`
	Value     *float64                        `json:"value,omitempty" description:"new number, for a tile made with value; every update of such a tile needs it"`
	Reference *float64                        `json:"reference,omitempty" description:"new reference, only with value; omitted drops the current one, so send it again to keep the change"`
	Query     neui.IslandCall[struct{}, data] `json:"query,omitempty" description:"new query, for a tile made with query; it runs right away"`
}

type statView struct {
	Label     string   `json:"label" required:"true" description:"the tile's label"`
	State     string   `json:"state" required:"true" enum:"shown,no data" description:"shown, or no data: the query failed, so no number shows"`
	Value     *float64 `json:"value,omitempty" description:"the number shown; absent with no data"`
	Reference *float64 `json:"reference,omitempty" description:"the number value is compared with; absent when there is none"`
	Change    *float64 `json:"change,omitempty" description:"(value - reference) / |reference|, such as 0.2 for a rise of 20%; absent without a reference or when it is 0"`
}
