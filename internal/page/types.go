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
	Path string          `json:"path" required:"true"`
	Tool string          `json:"tool" required:"true" description:"a name from the space's tools"`
	Arg  json.RawMessage `json:"arg" description:"matching the tool's inputSchema"`
}

type islandSpecArg struct {
	Path string `json:"path" required:"true"`
	Page uint   `json:"page,omitempty" description:"from 1; absent reads page 1"`
}

type islandSpecSchemaArg struct {
	Path     string   `json:"path" required:"true"`
	Revision string   `json:"revision" required:"true" description:"from the first line of island_spec"`
	Items    []string `json:"items" required:"true" minItems:"1" description:"names from the spec's catalog, such as ui.form or app.orders"`
}

type syncArg struct {
	Path string `json:"path" description:"absent is the page root"`
}

type treeArg struct {
	Path  string `json:"path,omitempty" description:"the space to start from; absent is the page root"`
	Depth *uint  `json:"depth,omitempty" description:"levels of child spaces to show below it, 3 when absent; 0 shows the space alone"`
}

type tool struct {
	Name   string          `json:"name"`
	Desc   string          `json:"desc"`
	Input  json.RawMessage `json:"inputSchema"`
	decode func(arg json.RawMessage) (any, error)
	call   func(ctx context.Context, input any) (any, error)
	raw    json.RawMessage
	lean   json.RawMessage
	hash   uint64
}

func (t tool) MarshalJSON() ([]byte, error) {
	return t.raw, nil
}

type data struct {
	Text   string `json:"text,omitempty"`
	Object any    `json:"object,omitempty"`
	raw    json.RawMessage
	hash   uint64
}

func (d data) MarshalJSON() ([]byte, error) {
	return d.raw, nil
}

type state struct {
	_        struct{} `description:"inputs: a button or link takes click, an input or radio takes write, a radio standing for its whole same-name group; rights RW allow every call, R only read and focus, W only click, write and focus. data: what the application tells you about the space, not a copy of the screen: it may describe things the page does not display"`
	Path     string   `json:"path"`
	Desc     string   `json:"desc"`
	Island   bool     `json:"island,omitempty"`
	Tools    []tool   `json:"tools,omitempty"`
	Inputs   []input  `json:"inputs,omitempty"`
	Data     []data   `json:"data,omitempty"`
	Children []child  `json:"children,omitempty"`
}

type child struct {
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Island bool   `json:"island,omitempty"`
}

type node struct {
	child
	Tools    int      `json:"tools,omitempty"`
	Inputs   int      `json:"inputs,omitempty"`
	Data     int      `json:"data,omitempty"`
	Goes     []string `json:"goes,omitempty"`
	Children []node   `json:"children,omitempty" description:"in name order"`
	More     int      `json:"more,omitempty"`
	_        struct{} `description:"goes: where a navigation space's links lead, as their descriptions. more: how many of the space's children are not listed; call tree on it to go deeper, or sync it to see them all"`
}

type report struct {
	Diff    *diff    `json:"diff,omitempty" description:"absent only when it and changes would not fit in a result; your read then stays as it was before the call"`
	Changes []change `json:"changes,omitempty" description:"when many spaces change at once, the densest groups are folded into their common ancestor; added spaces stay listed one by one unless very many appeared at once"`
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
	Handled bool   `json:"handled"`
	Note    string `json:"note,omitempty"`
	report
	_ struct{} `description:"handled: false is not a failure: the action was done on the page, and the application may use it later"`
}

type inputRef struct {
	Path string `json:"path" required:"true"`
	ID   string `json:"id" required:"true" description:"an id from the space's inputs"`
}

type writeArg struct {
	Path  string `json:"path" required:"true"`
	ID    string `json:"id" required:"true" description:"an id from the space's inputs"`
	Value any    `json:"value" required:"true" description:"string for text, boolean for checkbox, an option's value from read for select and radio, array of option values for multiple select"`
}

type readResult struct {
	Value    any            `json:"value,omitempty"`
	Label    string         `json:"label,omitempty"`
	Href     string         `json:"href,omitempty"`
	Options  []selectOption `json:"options,omitempty"`
	Files    []string       `json:"files,omitzero"`
	Disabled bool           `json:"disabled,omitempty"`
}

type toolResult struct {
	Output any `json:"output,omitempty"`
	report
	_ struct{} `description:"output: what the page tool returned, or on a failed call what its error carried"`
}

type selectOption struct {
	Value    string `json:"value"`
	Label    string `json:"label"`
	Selected bool   `json:"selected"`
	Disabled bool   `json:"disabled,omitempty"`
}

type inputArg struct {
	Action string `json:"action"`
	Value  any    `json:"value,omitempty"`
}

type islandBeginArg struct {
	Path     string `json:"path" required:"true"`
	Revision string `json:"revision" required:"true" description:"from the first line of island_spec"`
	File     string `json:"file,omitempty" description:"workspace path of a file holding the part"`
	Code     string `json:"code,omitempty"`
	Comment  string `json:"comment" description:"short description of what the app does; shown to the user while it runs and echoed in its task records"`
}

type islandNextArg struct {
	Path string `json:"path" required:"true"`
	File string `json:"file,omitempty" description:"workspace path of a file holding the part"`
	Code string `json:"code,omitempty"`
}

type islandEndArg struct {
	Path string `json:"path" required:"true"`
}

type islandRunResult struct {
	Task   uint     `json:"task"`
	Script uint     `json:"script"`
	_      struct{} `description:"task identifies the app in its task_notify records and its task_result; script identifies this part in its notifications_list and script_error records and in an error that ends the app"`
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
