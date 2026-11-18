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

package api

// RecordID identifies a record within its thread.
type RecordID int

// Record is a thread-level entry produced by code: Origin is the round
// whose action created it, and Pin is the round whose user message
// renders it on replay (NoRoundID while unpinned).
//
// The model reads a record as an element named Kind with Attrs as attributes
// and Data, escaped or JSON-encoded, as its body.
type Record struct {
	ID     RecordID       `json:"id"`     // identifies the record within its thread
	Origin RoundID        `json:"origin"` // round whose action created it
	Pin    RoundID        `json:"pin"`    // round that renders it; NoRoundID while unpinned
	Kind   string         `json:"kind"`   // element name, e.g. "task_notify"
	Attrs  map[string]any `json:"attrs"`  // element attributes; may be nil
	Data   any            `json:"data"`   // body: a string, json.RawMessage or any JSON-encodable value
}
