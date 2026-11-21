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

package form

// Desc describes the form control to the agent, for Library.Control.
const Desc = "Form the user fills and submits: one record or one answer per form. One field is a fine form: it is how to ask for a single value. Every field is editable, with no hidden or disabled ones: show the record's context in a markdown view beside the form and keep its id in a script variable. A list field is for entries the user adds, such as an order's lines, not for records you loaded. Every rule in params, field types and validate functions included, is enforced before a submit reaches you, unless its button has novalidate, and the user sees each broken rule's error message: don't re-check what those rules cover, in submitted data or in values you prefill. A field with min 1 and max 1 holds exactly one item, read [0]; where rules are enforced, only optional fields can be missing. The form keeps what the user entered after a submit: to have the user fix something only the script can check, show why in a markdown view in the danger style next to the form, usually above it, and call ui.draw() again. Inside ui.row, it takes the width the other items leave."
