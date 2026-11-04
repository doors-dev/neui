package api

// Tool is a tool the caller runs itself and answers with a ContentToolResult.
//
// Tools the provider runs are never declared here, even where the wire puts
// both in one array: the caller neither executes nor answers them, and their
// output arrives as ContentOpaque. Configure those on the adapter.
type Tool struct {
	Name   string `json:"name"`
	Desc   string `json:"description"`
	Schema any    `json:"schema"`
}

// ToolChoice constrains which tool the model may call. The constants below are
// modes; any other value names the one tool the model must call. Empty leaves
// the choice to the provider, which is not ToolChoiceAuto everywhere, so an
// adapter sends no field for it.
//
// Modes and tool names share one string: a tool named auto, none or any cannot
// be forced and becomes a mode instead.
type ToolChoice string

const (
	ToolChoiceAuto ToolChoice = "auto" // call a tool or answer, model decides
	ToolChoiceNone ToolChoice = "none" // answer without calling a tool
	ToolChoiceAny  ToolChoice = "any"  // call some tool, model picks which
)

// Tool reports the forced tool name, or false for empty and for any mode.
func (c ToolChoice) Tool() (string, bool) {
	switch c {
	case "", ToolChoiceAuto, ToolChoiceNone, ToolChoiceAny:
		return "", false
	}
	return string(c), true
}
