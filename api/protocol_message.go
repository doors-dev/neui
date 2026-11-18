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

// Role is the author of a message. Only user and assistant exist: the system
// prompt is request configuration, and a tool result is a content block that an
// adapter places in whichever message its provider expects.
type Role string

// The two roles.
const (
	RoleUser      Role = "user"
	RoleAssistant Role = "assistant"
)

// Message is one turn of a conversation sent in a Request.
type Message struct {
	Role    Role     `json:"role"`    // who wrote the turn
	Content Contents `json:"content"` // blocks in order; tool results go in user messages
}
