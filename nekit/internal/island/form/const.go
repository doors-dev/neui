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
const Desc = "Form the user fills and submits. Keep a form to what one step needs: split a long flow into forms shown one after another, and for complex flows let the user pick actions from a table or list, each opening the form it needs. A submit gives {\"data\": object, \"submit\": string}, data holding each filled field's value by name. Every rule in params, field types and validate functions included, is enforced before a submit reaches you, unless its button has novalidate, and the user sees each broken rule's error message: don't re-check what those rules cover, in submitted data or in values you prefill. A field with min 1 and max 1 holds exactly one item, read [0]; where rules are enforced, only optional fields can be missing. The form keeps what the user entered after a submit: to have the user fix something only the script can check, show why and call ui.draw() again. Inside ui.row, it takes the width the other items leave."
