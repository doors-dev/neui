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
	Part uint   `json:"part,omitempty" description:"the part to read, from 1, when the spec comes in parts; absent reads part 1"`
}

type syncArg struct {
	Path string `json:"path" description:"space path: child names from the root joined with dots; \"\" is the page root"`
}

type treeArg struct {
	Path  string `json:"path,omitempty" description:"the space to start from: child names from the root joined with dots; absent or \"\" is the page root"`
	Depth *uint  `json:"depth,omitempty" description:"levels of child spaces to show below it, 3 when absent; 0 shows the space alone"`
}

type tool struct {
	Name  string          `json:"name"`
	Desc  string          `json:"desc"`
	Input json.RawMessage `json:"inputSchema" description:"JSON schema of the tool call arg"`
	call  func(ctx context.Context, arg json.RawMessage) (any, error)
	raw   json.RawMessage
	hash  uint64
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
	Inputs   []input `json:"inputs,omitempty" description:"interactive inputs, address them with the input tool"`
	Data     []data  `json:"data,omitempty" description:"information the developer attached for you: anything that helps serve the user here — objects, hints, notes; it is context for you, not a copy of the screen, and may describe things the page does not display"`
	Children []child `json:"children,omitempty" description:"child spaces; a child's path is this path and its name joined with a dot"`
}

type child struct {
	Name   string `json:"name"`
	Desc   string `json:"desc"`
	Island bool   `json:"island,omitempty" description:"this space is an island: run an app in it with the island tools"`
}

type node struct {
	child
	Children []node `json:"children,omitempty" description:"child spaces in name order"`
	More     int    `json:"more,omitempty" description:"this many children not listed; call tree on this space to go deeper, or sync it to see all its children"`
}

type report struct {
	Diff    *diff    `json:"diff,omitempty" description:"what changed in the called space during the call; absent when nothing did, or when it and changes would not fit in a result"`
	Changes []change `json:"changes,omitempty" description:"other spaces that changed during the call. When many spaces change at once, the densest groups are folded into their common ancestor; spaces counts the changed spaces it stands for, so sync it to look inside. Newly added spaces stay listed unless a whole region re-rendered."`
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
	Added   []input `json:"added,omitempty" description:"inputs that appeared, or came back with a new id after a re-render"`
	Removed []uint  `json:"removed,omitempty" description:"ids no longer on the page"`
}

type childrenDiff struct {
	Added   []child  `json:"added,omitempty" description:"child spaces that appeared"`
	Removed []string `json:"removed,omitempty" description:"names of child spaces that went"`
}

type change struct {
	Path    string   `json:"path"`
	Changed []string `json:"changed,omitempty" description:"the parts of the space that changed: tools, inputs, data, children"`
	Added   bool     `json:"added,omitempty" description:"the space appeared"`
	Desc    string   `json:"desc,omitempty"`
	Removed bool     `json:"removed,omitempty" description:"the space went"`
	Spaces  int      `json:"spaces,omitempty"`
}

type inputRef struct {
	Path string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	ID   uint   `json:"id" required:"true" description:"input id from the space inputs list"`
}

type writeArg struct {
	Path  string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	ID    uint   `json:"id" required:"true" description:"input id from the space inputs list"`
	Value any    `json:"value" required:"true" description:"string for text, boolean for checkbox, option value for select and radio, array of option values for multiple select"`
}

type readResult struct {
	Value    any            `json:"value,omitempty" description:"the current value for input, select and radio kinds, the form value for buttons"`
	Label    string         `json:"label,omitempty" description:"text the user sees on it, buttons and links only"`
	Href     string         `json:"href,omitempty" description:"link target, link only"`
	Options  []selectOption `json:"options,omitempty" description:"choices, select and radio only"`
	Disabled bool           `json:"disabled,omitempty" description:"present and true when the element is disabled"`
}

type hookResult struct {
	Hooks int `json:"hooks" description:"how many server handlers the call triggered; the client-side action is performed regardless, 0 only means no server handler listened"`
	report
}

type toolResult struct {
	Output any `json:"output" description:"what the page tool returned"`
	report
}

type selectOption struct {
	Value    string `json:"value" description:"pass to the input action to choose this option"`
	Label    string `json:"label" description:"text the user sees"`
	Selected bool   `json:"selected" description:"whether this option is currently chosen"`
	Disabled bool   `json:"disabled,omitempty" description:"present and true when this option is disabled"`
}

type inputArg struct {
	Action string `json:"action"`
	Value  any    `json:"value,omitempty"`
}

type islandStartArg struct {
	Path     string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	Revision string `json:"revision" required:"true" description:"the revision from the first line of island_spec"`
	File     string `json:"file,omitempty" description:"workspace path of the script file; set exactly one of file and code"`
	Code     string `json:"code,omitempty" description:"script source; set exactly one of file and code"`
	Comment  string `json:"comment" description:"short description of what the app does; shown to the user while it runs and echoed in its task records"`
}

type islandContinueArg struct {
	Path      string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
	File      string `json:"file,omitempty" description:"workspace path of the script file; set exactly one of file and code"`
	Code      string `json:"code,omitempty" description:"script source; set exactly one of file and code"`
	Interrupt bool   `json:"interrupt,omitempty" description:"drop queued scripts and stop the running one at its next ui.draw; otherwise this script runs after the queued ones"`
}

type islandStopArg struct {
	Path string `json:"path" required:"true" description:"space path: child names from the root joined with dots; \"\" is the page root"`
}

type islandRunResult struct {
	Task   uint `json:"task" description:"id of the app's task: its task_notify records and its task_result carry it"`
	Script uint `json:"script" description:"id of this script: task_notify records from it carry it as script, and an error that ends the app names it"`
}

type refusal struct {
	Refused string `json:"refused"`
	Diff    *diff  `json:"diff,omitempty"`
	Space   *state `json:"space,omitempty"`
}

func (r refusal) Error() string {
	return r.Refused
}

func (r refusal) Content() api.UserContent {
	raw, err := json.Marshal(r)
	if err != nil {
		return api.ContentText{Text: r.Refused}
	}
	return api.ContentText{Text: string(raw)}
}

func (r refusal) ContentType() string {
	return "text"
}
