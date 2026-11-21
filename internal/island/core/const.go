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

package core

import (
	"errors"
	"fmt"
	"strings"
)

const slotNotFoundMessage = "no such slot: create one with ui.slot()"
const widgetNotFoundMessage = "no such view or control: create it again"
const contentKindMessage = "slot content must be a view, control, layout or slot"
const layoutNotFoundMessage = "no such layout in this island: island_spec lists its layouts"
const slotCycleMessage = "a slot cannot end up inside itself, directly or through the slots its content holds: place a different slot, or set the inner slot to something else first"
const itemLimitMessage = "too many slots, views, controls and layouts alive: reuse them instead of creating new ones"
const opsLimitMessage = "too many set, update and notify calls since the last ui.draw: call it to send them"
const notificationsLimitMessage = "the ui.notify values since the last ui.draw would pass this size: send less, or call ui.draw between them"
const nameMessage = "a library name must be a JS identifier"
const nameTakenMessage = "a library name is already taken by another view, control, layout or fetch"
const widgetKindNotFoundMessage = "no such view or control in this island: island_spec lists them"
const idleNote = "the screen has no control, so the part waits in this ui.draw() until you send the next part with island_interrupt or end the app with island_end"
const emptyNote = "the screen is empty: ui.root holds no view or control, so the user sees nothing while the part waits in this ui.draw(); send a part that sets ui.root with island_interrupt, or end the app with island_end"
const drawFallbackNote = "Your script read %[1]s on what ui.draw() returned, but the control's fields are under .output. The script kept running and got .output%[1]s instead. In future scripts, read .output.<field>. You won't get any more notifications of this type this run."
const unknownItemMessage = "no such item in the island's spec: island_spec lists them"
const definitionClashMessage = "two different types share this schema definition name even with its content hash appended: rename one of them"

var errSlotNotFound = errors.New(slotNotFoundMessage)
var errWidgetNotFound = errors.New(widgetNotFoundMessage)
var errContentKind = errors.New(contentKindMessage)
var errLayoutNotFound = errors.New(layoutNotFoundMessage)
var errSlotCycle = errors.New(slotCycleMessage)
var errItemLimit = errors.New(itemLimitMessage)
var errOpsLimit = errors.New(opsLimitMessage)
var errNotificationsLimit = errors.New(notificationsLimitMessage)
var errName = errors.New(nameMessage)
var errNameReserved = fmt.Errorf("a library name cannot be %s or %s", strings.Join(reserved[:len(reserved)-1], ", "), reserved[len(reserved)-1])
var errNameTaken = errors.New(nameTakenMessage)
var errWidgetKindNotFound = errors.New(widgetKindNotFoundMessage)
var errDefinitionClash = errors.New(definitionClashMessage)
var errUnknownItem = errors.New(unknownItemMessage)
