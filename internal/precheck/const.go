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

package precheck

const findingFormat = "line %d, column %d: TypeError: %s"
const uiMemberMessage = "%s: ui has only the views, controls and layouts the island's spec lists, plus root, slot, draw, notify and end"
const appMemberMessage = "%s: app has only the functions the island's spec lists"
const widgetMemberMessage = "%s: a view or control (%s) has only update(patch) and id"
const widgetSetMessage = "%s: a view or control (%s) has no set, slots do: to change what it shows, call update(patch)"
const slotMemberMessage = "%s: a slot has only set(...content)"
const slotUpdateMessage = "%s: a slot has no update, views and controls do: to change what it shows, set it again"
const layoutMemberMessage = "%s: a layout (%s) has only name, params and children, and no update: to change it, set its slot again"
const drawMemberMessage = "%s: what ui.draw() returns has only id and output: read the control's field as %s"
const undefinedMessage = "%s is not defined: no part declared it, and the script has no such tool or built-in"
const undefinedToolMessage = "%s is not defined: the script's tools are its modules' tools except page__* and js__*: it reaches the page only through app functions"
const noPatchMessage = "%s: update needs a patch object that matches the view's or control's patch schema"
const schemaMessage = "%s: %s"
const shapesMessage = "matches none of the allowed shapes"
const paramsWhat = "ui.%s params"
const patchWhat = "ui.%s patch"
const atPointer = " at '/%s'"
const widgetName = "ui.%s"
const widgetNames = " or ui."
const outputField = "%s.output.%s"
const outputIndex = "%s.output[%s]"
