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

package layout

// ColumnParams is what a script passes to ui.column.
type ColumnParams struct {
	_    struct{} `additionalProperties:"false"`
	Gap  string   `json:"gap" enum:"sm,md,lg" default:"md" description:"space between children"`
	Fill int      `json:"fill,omitempty" minimum:"1" maximum:"6" description:"only when this column is directly inside a row: share of the line's leftover space, split by weight"`
}

// RowParams is what a script passes to ui.row.
type RowParams struct {
	_       struct{} `additionalProperties:"false"`
	Gap     string   `json:"gap" enum:"sm,md,lg" default:"md" description:"space between children"`
	Align   string   `json:"align" enum:"start,center,end" default:"start" description:"vertical alignment of children within a line; center lines up a row of controls of different heights"`
	Justify string   `json:"justify" enum:"start,end,between" default:"start" description:"horizontal distribution of children along a line"`
}

// GridParams is what a script passes to ui.grid.
type GridParams struct {
	_    struct{} `additionalProperties:"false"`
	Gap  string   `json:"gap" enum:"sm,md,lg" default:"md" description:"space between cells"`
	Item string   `json:"item" enum:"sm,md,lg" default:"md" description:"minimum cell width"`
}
