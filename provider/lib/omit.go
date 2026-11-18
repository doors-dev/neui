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

package lib

import (
	"fmt"
	"log/slog"
	"sync"

	"neui.dev/api"
)

// EmptyResult is the text an adapter sends for a tool result that carries no
// blocks, so the model reads that the call succeeded and returned nothing
// rather than an empty output or a note about content that never existed.
const EmptyResult = "(the tool returned no output)"

// Omitted returns the placeholder text an adapter sends in place of content c
// that this wire cannot carry, such as "[image omitted]", so the turn keeps its
// shape instead of failing. It logs nothing: an omission is a fixed consequence
// of the stored thread and recurs on every turn. why says at the call site why
// the content cannot go and never reaches the model.
func Omitted(c api.Content, why string) string {
	switch c.(type) {
	case api.ContentImage:
		return "[image omitted]"
	case api.ContentDocument:
		return "[document omitted]"
	case api.ContentReasoning:
		return "[reasoning omitted]"
	}
	return "[content omitted]"
}

var unknownSeen sync.Map

// Unknown returns the placeholder an adapter sends in place of content c whose
// type it has no case for, and logs that at Error once per provider and type
// per process: a missing type is a bug in the adapter, and the stored thread
// would otherwise repeat the record on every turn. message is the index of the
// api.Message holding c.
func Unknown(provider string, message int, c api.Content) string {
	typ := fmt.Sprintf("%T", c)
	if _, seen := unknownSeen.LoadOrStore(provider+"\x00"+typ, struct{}{}); !seen {
		slog.Default().Error("unknown content type",
			"provider", provider,
			"message", message,
			"content", typ)
	}
	return "[content omitted]"
}
