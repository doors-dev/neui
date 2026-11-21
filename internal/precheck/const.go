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
const uiMemberMessage = "%s: no view, control or layout named %s in the island's spec"
const appMemberMessage = "%s: no app function named %s in the island's spec"
const widgetMemberMessage = "%s: %s has no %s; views and controls have only update(patch) and id"
const widgetSetMessage = "%s: %s has no set; set is for slots: to change what it shows, call update(patch)"
const slotMemberMessage = "%s: a slot has no %s; slots have only set(...content)"
const slotUpdateMessage = "%s: a slot has no update; update is for views and controls: to change what a slot shows, set it again"
const layoutMemberMessage = "%s: %s is a layout and has no %s; to change it, set its slot again"
const drawMemberMessage = "%s: ui.draw() returns only id and output; read the control's field as %s"
const outputMemberMessage = "%s: no control's output has %s"
const outputFieldsMessage = "; the outputs have only %s"
const idLiteralMessage = "%s: a control's id is a number set when it is created, never a literal such as %s: compare it with that control, such as form.id, kept in a variable the loop can reach"
const emptyScreenMessage = "%s: the screen is empty, as no part sets ui.root: set it to what the user should see, such as ui.root.set(form), before you draw"
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
