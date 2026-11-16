package island

import (
	"encoding/json"
	"errors"
	"fmt"
)

const unmountedMessage = "the island is no longer on the page: call tree to see the current spaces"
const nothingToContinueMessage = "no app is running in this island: begin one with island_begin"
const nothingToStopMessage = "no app is running in this island"
const stoppedMessage = "the island's app was ended with island_end"
const endedMessage = "the island's app ended: its script called ui.end()"
const overtakenMessage = "the island's app was replaced by a new one begun with island_begin"
const screenSpaceName = "screen"
const screenSpaceDesc = "what the island shows now: the app while one runs, otherwise its placeholder or end screen"
const noAppStatus = "no app is running in this island: begin one with island_begin"
const idleNote = "the screen has no control, so the part waits in this ui.draw() until you send the next part with island_interrupt or end the app with island_end"
const scriptErrorNote = "the part stopped at this error and any parts queued behind it were dropped, listed in dropped; the app runs on: send what should still run with island_append"
const scriptErrorActionNote = "the part stopped at this error while handling the user's action in last_action, which was not carried out; any parts queued behind it were dropped, listed in dropped; the app runs on: send the fix with island_append, and in it carry out that action or ask the user to redo it"
const callErrorNote = "fix the source and pass it again, by an update or in a new view or control"
const widgetErrorNote = "the view or control keeps running: update or replace it"
const turnTimeMessage = "the part went that long without ui.draw, app function and tool calls included, and was stopped: keep the work between two draws short and avoid unbounded loops"
const queueFullMessage = "too many parts are already queued: wait for the app to run them, or send this one with island_interrupt"
const fnTimeMessage = "the function ran that long and was stopped: keep it short and avoid unbounded loops"
const fnResultMessage = "the function returned something that is not JSON, such as a function or a symbol: return plain JSON values"
const fnReportsCap = 16
const scriptRejectedMessage = "the part was refused: nothing ran, and nothing was queued, replaced or dropped; fix it and send it again"

const specReference = `
Reading the reference
Schemas are JSON Schema; a {"$ref":"#/definitions/X"} refers to X under Definitions at the end. A field with "function" takes the source of a one-argument JavaScript function as a string, such as "(q) => app.find({ q })" or String(fn): arg is what it receives and returns what it must return. The view or control calls it on its own, at any time, apart from the script: it sees only its argument, app.* and console, so write script data into the source, such as "(v) => " + JSON.stringify(taken) + ".includes(v) ? 'taken' : ''". It must be synchronous, and built-ins are frozen: assigning over an inherited property such as err.name throws.
`

var errUnmounted = errors.New(unmountedMessage)
var errNothingToContinue = errors.New(nothingToContinueMessage)
var errNothingToStop = errors.New(nothingToStopMessage)
var errStopped = errors.New(stoppedMessage)
var errEnded = errors.New(endedMessage)
var errOvertaken = errors.New(overtakenMessage)
var errTurnTime = errors.New(turnTimeMessage)
var errFnTime = errors.New(fnTimeMessage)
var errFnResult = errors.New(fnResultMessage)
var errQueueFull = errors.New(queueFullMessage)
var errScriptRejected = errors.New(scriptRejectedMessage)

var stoppedResult = json.RawMessage(`"` + stoppedMessage + `"`)
var overtakenResult = json.RawMessage(`"` + overtakenMessage + `"`)

func runningStatus(task uint) string {
	return fmt.Sprintf("the island runs an app as task %d: send its next part with island_append or island_interrupt, or end it with island_end", task)
}

func endedStatus(task uint) string {
	return fmt.Sprintf("the island's app, task %d, has ended and the island shows its end screen: begin a new one with island_begin", task)
}

func failedStatus(task uint, err error) string {
	return fmt.Sprintf("the island's app, task %d, ended with an error and the island shows its error screen: begin a new one with island_begin; the error: %v", task, err)
}
