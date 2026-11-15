package island

import (
	"encoding/json"
	"errors"
	"fmt"
)

const unmountedMessage = "the island is no longer on the page: sync to see what the page has now"
const nothingToContinueMessage = "no app is running in this island: start one with island_start"
const nothingToStopMessage = "no app is running in this island"
const stoppedMessage = "the island's app was stopped"
const endedMessage = "the island's app ended: its script called ui.end()"
const overtakenMessage = "the island's app was replaced by a new one started with island_start"
const screenSpaceName = "screen"
const screenSpaceDesc = "what the island shows now: the app while one runs, otherwise its placeholder or end screen"
const noAppStatus = "no app is running in this island: start one with island_start"
const idleNote = "the frame has no controls, so the script waits here until you send a script with interrupt or stop the app"
const turnTimeMessage = "the script went that long without ui.draw, app function and tool calls included, and was stopped; the app lives on: keep the work between two draws short and avoid unbounded loops"
const queueFullMessage = "too many scripts are already queued: wait for the app to run them, or send this one with interrupt"
const fnTimeMessage = "the function ran that long and was stopped: keep it short and avoid unbounded loops"
const fnResultMessage = "the function returned something that is not JSON, such as a function or a symbol"
const fnReportsCap = 16
const scriptRejectedMessage = "the script was not queued and nothing ran: fix it and send it again"

const specNotation = `Schemas are JSON Schema. Pass an object even when nothing is needed: {}. What update(patch) does is up to each view or control: see the descriptions in its patch schema. A {"$ref":"#/definitions/X"} refers to X under Definitions at the end. A field with "function" takes the JavaScript source of a one-argument function: arg is what it receives, returns what it must return.`
const specRuntime = `- A control reports only while a script is inside ui.draw(), one action per draw; actions taken at other times, while a script runs or after it has ended, are dropped.
- When a script part completes execution, the app keeps its last drawn screen and waits for the next script. Buttons do nothing until you send the next script with island_continue. To close the app from a script, call ui.end() explicitly.
- A value that doesn't match its schema throws at the call; a failing app function throws its error, which may carry a value in e.value.
- Your other tools are globals here too, named <module>__<tool>(args), such as files__read or page__tool; they run synchronously and return their result.
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
var endedResult = json.RawMessage(`"` + endedMessage + `"`)
var overtakenResult = json.RawMessage(`"` + overtakenMessage + `"`)

func runningStatus(task uint) string {
	return fmt.Sprintf("the island runs an app as task %d: send it new code with island_continue, or stop it with island_stop", task)
}

func endedStatus(task uint) string {
	return fmt.Sprintf("the island's app, task %d, has ended and the island shows its end state: start a new one with island_start", task)
}

func failedStatus(task uint, err error) string {
	return fmt.Sprintf("the island's app, task %d, ended with an error and the island shows its error state: start a new one with island_start; the error: %v", task, err)
}
