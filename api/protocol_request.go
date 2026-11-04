package api

// Request is one call to a model. Field order follows the cache hierarchy:
// tools, then system, then messages. A change at one level invalidates that
// level and every level after it.
//
// Model, the response cap, and sampling and reasoning settings are not fields
// here. They belong to the provider's own configuration; a caller wanting one
// model at two settings configures the provider twice.
type Request struct {
	Tools      []Tool     `json:"tools"`
	System     string     `json:"system"`
	Messages   []Message  `json:"messages"`
	ToolChoice ToolChoice `json:"toolChoice"`
}
