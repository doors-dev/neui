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

// Desc describes the stat view to the agent, for Library.View.
const Desc = "One key number, such as revenue, sign-ups or an error rate, with its change against a reference. Set either value or query. Put several in ui.grid or ui.row; for a trend, put a line chart beside it."

const getStatName = "get_stat"
const getStatDesc = "Read what the user's tile shows."

const valueOrQueryMessage = "set exactly one of value and query"
const refreshWithoutQueryMessage = "refresh needs query: drop refresh, or read the number with query instead of value"
const referenceWithoutValueMessage = "reference needs value: with query, return reference from the query"
const currencyUnitMessage = "format currency needs unit set to an ISO 4217 code, such as USD or EUR"
const currencyCodeMessage = "unit %q is not an ISO 4217 currency code, such as USD or EUR"
const unitMessage = "unit is only for number and currency; %s values need none"
const valuePatchMessage = "this tile was made with value, so an update takes value and an optional reference"
const queryPatchMessage = "this tile was made with query, so an update takes query only"
