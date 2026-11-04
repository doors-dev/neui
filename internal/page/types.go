package page

import "encoding/json"

type toolArg struct {
	Path string          `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
	Seq  uint            `json:"seq" required:"true" description:"the space's seq: from sync's space.seq, or the seq of the last write, click or tool result on this space; never a pageSeq"`
	Tool string          `json:"tool" required:"true" description:"tool name from the space tools list"`
	Arg  json.RawMessage `json:"arg" description:"argument matching the tool inputSchema"`
}

type islandSpecArg struct {
	Path string `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
}

type islandSpecResult struct {
	Seq  uint   `json:"seq" description:"the island space's seq, the same one sync reports; pass it to island_start"`
	Spec string `json:"spec"`
}

type syncArg struct {
	Path  string `json:"path" description:"space path to sync from, empty string is the page root"`
	Depth uint   `json:"depth" description:"levels of children to describe fully, 0 reads the target space with previews of its direct children"`
}

type syncResult struct {
	PageSeq uint       `json:"pageSeq" description:"page version, only for the changes tool's sincePageSeq; never pass it as a space's seq, use space.seq for that"`
	Space   *syncSpace `json:"space"`
}

type changesArg struct {
	SincePageSeq uint   `json:"sincePageSeq" required:"true" description:"pageSeq from a previous sync, changes, write, click or tool result; only spaces changed after it are returned"`
	Path         string `json:"path,omitempty" description:"limit to this space's subtree, empty string is the whole page"`
}

type changesResult struct {
	PageSeq uint         `json:"pageSeq" description:"pass as sincePageSeq next time"`
	Spaces  []*syncSpace `json:"spaces" description:"spaces changed since sincePageSeq, each with its own seq, tools, inputs and children previews; a space whose children changed is listed even if its own seq did not change: its children list is the current truth, a child missing from it was removed; empty means nothing changed"`
}

type syncSpace struct {
	Path     string                `json:"path" description:"pass to every other page call"`
	Seq      uint                  `json:"seq,omitempty" description:"the space's seq, pass it to every call on this space; it changes only with the space's own tools, inputs and data, or a re-render, not when children come or go; previews omit it, so sync the path to get it"`
	Desc     string                `json:"desc"`
	Tools    []syncTool            `json:"tools,omitempty" description:"tools callable in this space"`
	Inputs   []input               `json:"inputs,omitempty" description:"interactive inputs, address them with the input tool"`
	Data     []syncData            `json:"data,omitempty" description:"information the developer attached for you: anything that helps serve the user here — objects, hints, notes; it is context for you, not a copy of the screen, and may describe things the page does not display"`
	Island   bool                  `json:"island,omitempty" description:"this space is an island: run an app in it with the island tools"`
	Children map[string]*syncSpace `json:"children,omitempty" description:"children spaces; a space showing only path and desc is a preview beyond depth, sync its path to open it"`
}

type syncData struct {
	Text   string `json:"text,omitempty" description:"the developer's note in plain words, on its own or telling what the object is"`
	Object any    `json:"object,omitempty" description:"structured payload, shape is the developer's choice"`
}

type inputRef struct {
	Path string `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
	Seq  uint   `json:"seq" required:"true" description:"the space's seq: from sync's space.seq, or the seq of the last write, click or tool result on this space; never a pageSeq"`
	ID   uint   `json:"id" required:"true" description:"input id from the space inputs list"`
}

type writeArg struct {
	Path  string `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
	Seq   uint   `json:"seq" required:"true" description:"the space's seq: from sync's space.seq, or the seq of the last write, click or tool result on this space; never a pageSeq"`
	ID    uint   `json:"id" required:"true" description:"input id from the space inputs list"`
	Value any    `json:"value" required:"true" description:"string for text, boolean for checkbox, option value for select and radio, array of option values for multiple select"`
}

type readResult struct {
	Value   any            `json:"value,omitempty" description:"the current value for input, select and radio kinds, the form value for buttons"`
	Label   string         `json:"label,omitempty" description:"text the user sees on it, buttons and links only"`
	Href    string         `json:"href,omitempty" description:"link target, link only"`
	Options []selectOption `json:"options,omitempty" description:"choices, select and radio only"`
}

type hookResult struct {
	Hooks   int           `json:"hooks" description:"how many server handlers the call triggered; the client-side action is performed regardless, 0 only means no server handler listened"`
	Seq     uint          `json:"seq" description:"the called space's seq after this call; pass it to the next call on this space; 0 means the call removed or re-rendered the space, so sync its path"`
	Changes changesResult `json:"changes" description:"other spaces changed since just before this call, possibly by other causes too, with their current seqs, so no sync is needed; the called space is listed too if its own tools, inputs or data changed"`
}

type toolResult[O any] struct {
	Output  O             `json:"output" description:"what the page tool returned"`
	Seq     uint          `json:"seq" description:"the called space's seq after this call; pass it to the next call on this space; 0 means the call removed or re-rendered the space, so sync its path"`
	Changes changesResult `json:"changes" description:"other spaces changed since just before this call, possibly by other causes too, with their current seqs, so no sync is needed; the called space is listed too if its own tools, inputs or data changed"`
}

type selectOption struct {
	Value    string `json:"value" description:"pass to the input action to choose this option"`
	Label    string `json:"label" description:"text the user sees"`
	Selected bool   `json:"selected" description:"whether this option is currently chosen"`
}

type inputArg struct {
	Action string `json:"action"`
	Value  any    `json:"value,omitempty"`
}

type syncTool struct {
	Name  string          `json:"name"`
	Desc  string          `json:"desc"`
	Input json.RawMessage `json:"inputSchema" description:"JSON schema of the tool call arg"`
}

type islandStartArg struct {
	Path    string `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
	Seq     uint   `json:"seq" required:"true" description:"the island space's seq, from sync or island_spec; never a pageSeq"`
	File    string `json:"file,omitempty" description:"workspace path of the script file; set exactly one of file and code"`
	Code    string `json:"code,omitempty" description:"script source; set exactly one of file and code"`
	Comment string `json:"comment" description:"short description of what the app does; shown to the user while it runs and echoed in its task records"`
}

type islandContinueArg struct {
	Path      string `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
	File      string `json:"file,omitempty" description:"workspace path of the script file; set exactly one of file and code"`
	Code      string `json:"code,omitempty" description:"script source; set exactly one of file and code"`
	Interrupt bool   `json:"interrupt,omitempty" description:"drop queued scripts and stop the running one at its next ui.draw; otherwise this script runs after the queued ones"`
}

type islandStopArg struct {
	Path string `json:"path" required:"true" description:"space path exactly as the latest sync reported it"`
}

type islandRunResult struct {
	Task   uint `json:"task" description:"id of the app's task: its task_notify records and its task_result carry it"`
	Script uint `json:"script" description:"id of this script: task_notify records from it carry it as script, and an error that ends the app names it"`
}
