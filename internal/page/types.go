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

package page

import (
	"context"
	"encoding/json"

	"neui.dev/api"
)

type toolArg struct {
	Path string          `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	Tool string          `json:"tool" required:"true" description:"tool name from the space tools list"`
	Arg  json.RawMessage `json:"arg" description:"argument matching the tool inputSchema"`
}

type islandSpecArg struct {
	Path string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	Page uint   `json:"page,omitempty" description:"the page to read, from 1, when the spec comes in pages; absent reads page 1"`
}

type syncArg struct {
	Path string `json:"path" description:"space path: child names from the root joined with dots; \"\" is the page root"`
}

type treeArg struct {
	Path  string `json:"path,omitempty" description:"the space to start from: child names from the root joined with dots; absent or \"\" is the page root"`
	Depth *uint  `json:"depth,omitempty" description:"levels of child spaces to show below it, 3 when absent; 0 shows the space alone"`
}

type tool struct {
	Name   string          `json:"name"`
	Desc   string          `json:"desc"`
	Input  json.RawMessage `json:"inputSchema" description:"JSON schema of the tool call arg"`
	decode func(arg json.RawMessage) (any, error)
	call   func(ctx context.Context, input any) (any, error)
	raw    json.RawMessage
	hash   uint64
}

func (t tool) MarshalJSON() ([]byte, error) {
	return t.raw, nil
}

type data struct {
	Text   string `json:"text,omitempty" description:"the developer's note in plain words, on its own or telling what the object is"`
	Object any    `json:"object,omitempty" description:"structured payload, shape is the developer's choice"`
	raw    json.RawMessage
	hash   uint64
}

func (d data) MarshalJSON() ([]byte, error) {
	return d.raw, nil
}

type state struct {
	Path     string  `json:"path" description:"pass to every other page call"`
	Desc     string  `json:"desc"`
	Island   bool    `json:"island,omitempty" description:"this space is an island: run an app in it with the island tools"`
	Tools    []tool  `json:"tools,omitempty" description:"tools callable in this space"`
	Inputs   []input `json:"inputs,omitempty" description:"interactive elements in this space"`
	Data     []data  `json:"data,omitempty" description:"context the developer attached for you, such as objects, hints and notes; not a copy of the screen, it may describe things the page does not display"`
	Children []child `json:"children,omitempty" description:"child spaces"`
}

type child struct {
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Island bool   `json:"island,omitempty" description:"this space is an island: run an app in it with the island tools"`
}

type node struct {
	child
	Tools    int    `json:"tools,omitempty"`
	Inputs   int    `json:"inputs,omitempty"`
	Data     int    `json:"data,omitempty"`
	Children []node `json:"children,omitempty" description:"child spaces in name order"`
	More     int    `json:"more,omitempty" description:"this many children not listed; call tree on this space to go deeper, or sync it to see all its children"`
}

type report struct {
	Diff    *diff    `json:"diff,omitempty" description:"what changed in the called space while the call ran, whatever caused it: this call, the user or the application; {} when nothing did. It brings the read of the space up to date, so no sync is needed after the call. Absent only when it and changes would not fit in a result; the read then stays as it was before the call."`
	Changes []change `json:"changes,omitempty" description:"other spaces that changed while the call ran, whatever caused it. When many spaces change at once, the densest groups are folded into their common ancestor; added spaces stay listed one by one unless very many appeared at once."`
}

type diff struct {
	Removed  bool          `json:"removed,omitempty" description:"the called space itself went during the call"`
	Tools    *toolsDiff    `json:"tools,omitempty"`
	Inputs   *inputsDiff   `json:"inputs,omitempty"`
	Data     []data        `json:"data,omitzero" description:"the space's whole data list, sent when it changed"`
	Children *childrenDiff `json:"children,omitempty"`
}

type toolsDiff struct {
	Added   []tool   `json:"added,omitempty" description:"tools that appeared or changed"`
	Removed []string `json:"removed,omitempty" description:"names of tools that went or changed"`
}

type inputsDiff struct {
	Added   []input  `json:"added,omitempty" description:"inputs that appeared or changed"`
	Removed []string `json:"removed,omitempty" description:"ids of inputs that went or changed"`
}

type childrenDiff struct {
	Added   []child  `json:"added,omitempty" description:"child spaces that appeared"`
	Removed []string `json:"removed,omitempty" description:"names of child spaces that went"`
}

type change struct {
	Path    string   `json:"path"`
	Changed []string `json:"changed,omitempty" description:"the parts of the space that changed: tools, inputs, data; a folded entry adds children"`
	Added   bool     `json:"added,omitempty" description:"the space appeared"`
	Desc    string   `json:"desc,omitempty"`
	Removed bool     `json:"removed,omitempty" description:"the space went"`
	Spaces  int      `json:"spaces,omitempty" description:"on a folded entry, how many changed spaces it stands for: sync it to look inside"`
}

type actionReport struct {
	Handled bool   `json:"handled" description:"whether a server handler ran for the action. false is not a failure: the action was done on the page, and the application may use it later."`
	Note    string `json:"note,omitempty" description:"set when the action's server handlers ran in part: some of them failed or were canceled"`
	report
}

type inputRef struct {
	Path string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	ID   string `json:"id" required:"true" description:"input id from the space inputs list"`
}

type writeArg struct {
	Path  string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	ID    string `json:"id" required:"true" description:"input id from the space inputs list"`
	Value any    `json:"value" required:"true" description:"string for text, boolean for checkbox, an option's value from read for select and radio, array of option values for multiple select"`
}

type readResult struct {
	Value    any            `json:"value,omitempty" description:"the current value for input (select included) and radio kinds, the form value for buttons"`
	Label    string         `json:"label,omitempty" description:"its text or its label"`
	Href     string         `json:"href,omitempty" description:"link target, link only"`
	Options  []selectOption `json:"options,omitempty" description:"choices, select and radio only"`
	Files    []string       `json:"files,omitzero" description:"names of the chosen files, file input only; empty when none is chosen"`
	Disabled bool           `json:"disabled,omitempty" description:"present and true when the element is disabled"`
}

type toolResult struct {
	Output any `json:"output,omitempty" description:"what the page tool returned; on a failed call, what its error carried"`
	report
}

type selectOption struct {
	Value    string `json:"value" description:"the option's value"`
	Label    string `json:"label" description:"text the user sees"`
	Selected bool   `json:"selected" description:"whether this option is currently chosen"`
	Disabled bool   `json:"disabled,omitempty" description:"present and true when this option is disabled"`
}

type inputArg struct {
	Action string `json:"action"`
	Value  any    `json:"value,omitempty"`
}

type islandBeginArg struct {
	Path     string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	Revision string `json:"revision" required:"true" description:"the revision from the first line of island_spec"`
	File     string `json:"file,omitempty" description:"workspace path of a file holding this part; set exactly one of file and code"`
	Code     string `json:"code,omitempty" description:"source of this part; set exactly one of file and code"`
	Comment  string `json:"comment" description:"short description of what the app does; shown to the user while it runs and echoed in its task records"`
}

type islandNextArg struct {
	Path string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	File string `json:"file,omitempty" description:"workspace path of a file holding this part; set exactly one of file and code"`
	Code string `json:"code,omitempty" description:"source of this part; set exactly one of file and code"`
}

type islandEndArg struct {
	Path string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
}

type islandRunResult struct {
	Task   uint `json:"task" description:"id of the app's task: its task_notify records and its task_result carry it"`
	Script uint `json:"script" description:"id of this part: its notifications_list and script_error records carry it as script, and an error that ends the app names it"`
}

type refusal struct {
	reason string
	Diff   *diff  `json:"diff,omitempty"`
	Space  *state `json:"space,omitempty"`
}

func (r refusal) Error() string {
	return r.reason
}

func (r refusal) Content() api.UserContent {
	raw, err := json.Marshal(r)
	if err != nil {
		return api.ContentText{Text: r.reason}
	}
	return api.ContentText{Text: string(raw)}
}

func (r refusal) ContentType() string {
	return "text"
}

type failure struct {
	result any
	err    error
}

func (f failure) Error() string {
	return f.err.Error()
}

func (f failure) Unwrap() error {
	return f.err
}

func (f failure) MarshalJSON() ([]byte, error) {
	return json.Marshal(f.result)
}

func (f failure) Content() api.UserContent {
	raw, err := json.Marshal(f.result)
	if err != nil {
		return api.ContentText{Text: f.err.Error()}
	}
	return api.ContentText{Text: string(raw)}
}

func (f failure) ContentType() string {
	return "text"
}
