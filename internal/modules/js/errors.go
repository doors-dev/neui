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

package js

import "errors"

const scriptErrorMessage = "the script failed: it did not parse, threw, or ran past the memory limit; a thrown error is also appended to the log file named in this error; fix the script and rerun, with a larger memoryMb if it ran out of memory"
const runtimeErrorMessage = "the script runtime failed internally; the script itself may be valid, rerun it"

var errScript = errors.New(scriptErrorMessage)
var errRuntime = errors.New(runtimeErrorMessage)
var errStopped = errors.New("stopped before finishing")
var errBusy = errors.New("the script queue is full; retry shortly")
