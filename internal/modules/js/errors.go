package js

import "errors"

const scriptErrorMessage = "the script failed: it did not parse, threw, or ran past the memory limit; a thrown error is also appended to the script's log file; fix the script and rerun, with a larger memoryMb if it ran out of memory"
const runtimeErrorMessage = "the script runtime failed internally; the script itself may be valid, rerun it"

var errScript = errors.New(scriptErrorMessage)
var errRuntime = errors.New(runtimeErrorMessage)
var errStopped = errors.New("stopped before finishing")
var errBusy = errors.New("the script queue is full; retry shortly")
