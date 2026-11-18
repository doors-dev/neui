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

// Request is one call to a model. Field order follows the cache hierarchy:
// tools, then system, then messages. A change at one level invalidates that
// level and every level after it.
//
// Model, the response cap, and sampling and reasoning settings are not fields
// here. They belong to the provider's own configuration; a caller wanting one
// model at two settings configures the provider twice.
type Request struct {
	Tools      []Tool     `json:"tools"`      // tools the caller runs; empty offers none
	System     string     `json:"system"`     // system prompt; empty sends none
	Messages   []Message  `json:"messages"`   // the conversation, oldest first
	ToolChoice ToolChoice `json:"toolChoice"` // empty leaves the choice to the provider
}
