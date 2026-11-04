package core

import "errors"

const slotNotFoundMessage = "no such slot: create one with ui.slot()"
const widgetNotFoundMessage = "no such widget: create it again"
const contentKindMessage = "slot content must be a widget, layout or slot"
const layoutNotFoundMessage = "no such layout in this island: island_spec lists its layouts"
const slotCycleMessage = "a slot cannot end up inside itself, directly or through the slots its content holds: place a different slot, or set the inner slot to something else first"
const itemLimitMessage = "too many slots, widgets and layouts alive: drop references you no longer need and remove unused ones from slots"
const opsLimitMessage = "too many set, update and notify calls since the last ui.draw: call it to send them"
const notificationsLimitMessage = "the ui.notify values since the last ui.draw would pass this size: send less, or call ui.draw between them"
const nameMessage = "a library name must be a JS identifier"
const nameReservedMessage = "a library name cannot be slot, draw, notify or root"
const nameTakenMessage = "a library name is already taken by another view, control, layout or fetch"
const widgetKindNotFoundMessage = "no such widget in this island: island_spec lists its views and controls"

var errSlotNotFound = errors.New(slotNotFoundMessage)
var errWidgetNotFound = errors.New(widgetNotFoundMessage)
var errContentKind = errors.New(contentKindMessage)
var errLayoutNotFound = errors.New(layoutNotFoundMessage)
var errSlotCycle = errors.New(slotCycleMessage)
var errItemLimit = errors.New(itemLimitMessage)
var errOpsLimit = errors.New(opsLimitMessage)
var errNotificationsLimit = errors.New(notificationsLimitMessage)
var errName = errors.New(nameMessage)
var errNameReserved = errors.New(nameReservedMessage)
var errNameTaken = errors.New(nameTakenMessage)
var errWidgetKindNotFound = errors.New(widgetKindNotFoundMessage)
