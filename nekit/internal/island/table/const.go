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

package table

// Desc describes the table control to the agent, for Library.Control.
const Desc = "For records the user acts on, many rows, or rows loaded on demand; a short table only to read belongs in markdown. A row's action buttons and clickable first cell give you its id, to change that record or open its form; then update the table to show the result. Set either query or rows. Inside ui.row, it takes the width the other items leave."

const getPageName = "get_page"
const getPageDesc = "Read the table as the user sees it."
const setPageName = "set_page"
const setPageDesc = "Turn the user's table to another page."
const setSearchName = "set_search"
const setSearchDesc = "Search the user's table; a new search starts at page 1."
const setSortName = "set_sort"
const setSortDesc = "Sort the user's table by one of its sortable columns, which goes back to page 1."
const setSelectedName = "set_selected"
const setSelectedDesc = "Select or unselect rows for the user. It does not submit them: submit_selected does."
const getSelectedName = "get_selected"
const getSelectedDesc = "Read which rows are selected, on every page."
const submitSelectedName = "submit_selected"
const submitSelectedDesc = "Press the selection's submit button for the user. Returns once the script draws again or its part ends; a success means the script was waiting for the user's action."
const clearSelectedName = "clear_selected"
const clearSelectedDesc = "Unselect all rows, on every page."
const clickRowName = "click_row"
const clickRowDesc = "Click a row's first cell for the user. Returns once the script draws again or its part ends; a success means the script was waiting for the user's action."
const clickActionName = "click_action"
const clickActionDesc = "Press one of a row's action buttons for the user. Returns once the script draws again or its part ends; a success means the script was waiting for the user's action."

const rowsFailedMessage = "the rows could not be loaded: %w"
const pastLastPageMessage = "no page %d: the last is %d"
const emptyPageMessage = "no page %d: it has no rows"
const notSortableMessage = "column %d is not sortable"
const notOnPageMessage = "row %q is not on the current page"
const noActionMessage = "row %q has no action %q"
const selectionFullMessage = "the selection holds at most 1000 rows, and %d are selected"
const nothingSelectedMessage = "nothing is selected"
const noDrawMessage = "not submitted: the script is not waiting in ui.draw()"
