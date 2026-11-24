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

// Desc describes the chart view to the agent, for Library.View.
const Desc = "Draws one measure of records, for when the shape of the numbers matters more than each value; values the user must read exactly belong in a table. A measure in another unit needs a chart of its own. Set either rows or query. Inside ui.row, it takes the width the other items leave."

const getChartName = "get_chart"
const getChartDesc = "Read what the user's chart shows."

const emitName = "neisland_chart"
const otherName = "Other"

const sourceMessage = "set exactly one of rows and query"
const refreshMessage = "refresh needs query: load the records with query, or leave refresh out"
const splitByMessage = "%s takes no splitBy: x already names each %s"
const stackMessage = "stack is only for bar; area with splitBy stacks on its own"
const currencyUnitMessage = "currency needs unit set to an ISO 4217 code, such as USD or EUR"
const currencyCodeMessage = "unit %q is not an ISO 4217 currency code, such as USD or EUR"
const unitMessage = "unit is only for number and currency; %s values need none"
const missingFieldMessage = "rows[%d] has no %s field %q"
const nullFieldMessage = "rows[%d]: %s field %q is null; every row needs one"
const numericYMessage = "rows[%d]: y field %q is %q, not a number; convert it with Number(), or use null for a gap"
const scatterXMessage = "rows[%d]: x field %q is not a number, as scatter needs; convert it with Number()"
const negativeMessage = "rows[%d]: y field %q is negative; %s takes values of 0 or more"
const queryMessage = "query result: %w"
const rowsPatchMessage = "this chart was made with query: patch it with query, not rows"
const queryPatchMessage = "this chart was made with rows: patch it with rows, not query"
