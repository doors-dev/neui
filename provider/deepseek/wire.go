package deepseek

import "encoding/json"

// chatRequest is the body of POST /chat/completions.
//
// Field order is the one api.Request documents: tools, then system, then
// messages. The system prompt is not a field here, it is the first element of
// messages, so tools and tool_choice lead and everything a turn does not cache
// on follows the message list.
//
// frequency_penalty and presence_penalty are absent on purpose. DeepSeek marks
// both "no longer supported", so a field for them would encode a knob the
// server ignores.
type chatRequest struct {
	Model           string         `json:"model"`
	Tools           []tool         `json:"tools,omitempty"`
	ToolChoice      any            `json:"tool_choice,omitempty"`
	Messages        []message      `json:"messages"`
	Stream          bool           `json:"stream"`
	StreamOptions   *streamOptions `json:"stream_options"`
	MaxTokens       int            `json:"max_tokens,omitempty"`
	Thinking        *thinking      `json:"thinking,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	Temperature     *float64       `json:"temperature,omitempty"`
	TopP            *float64       `json:"top_p,omitempty"`
	Stop            []string       `json:"stop,omitempty"`
	UserID          string         `json:"user_id,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type thinking struct {
	Type string `json:"type"`
}

// message is all four documented message shapes in one struct, because the
// endpoint takes a union and the fields that do not apply to a role are simply
// absent from it.
//
// Content has no omitempty: it is a string, nil, or a []any of textPart and
// imagePart. The assistant form is documented as string or null and a turn
// that only called tools has to send an explicit null, while system, user and
// tool all require the key present. User and tool content is a part list only
// when it carries an image.
type message struct {
	Role             string     `json:"role"`
	Content          any        `json:"content"`
	ReasoningContent string     `json:"reasoning_content,omitempty"`
	ToolCalls        []toolCall `json:"tool_calls,omitempty"`
	ToolCallID       string     `json:"tool_call_id,omitempty"`
}

type textPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imagePart struct {
	Type     string   `json:"type"`
	ImageURL imageURL `json:"image_url"`
}

type imageURL struct {
	URL string `json:"url"`
}

type toolCall struct {
	ID       string           `json:"id"`
	Type     string           `json:"type"`
	Function toolCallFunction `json:"function"`
}

// toolCallFunction carries Arguments as a JSON document inside a JSON string,
// which is what the endpoint documents. api.ContentToolUse.Input is raw
// JSON, so replay marshals it into a string rather than nesting it.
type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

// toolFunction omits strict. Strict schemas are gated behind the separate
// api.deepseek.com/beta host, so on this endpoint every schema is advisory and
// the arguments that come back are unvalidated.
type toolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type namedToolChoice struct {
	Type     string            `json:"type"`
	Function namedToolFunction `json:"function"`
}

type namedToolFunction struct {
	Name string `json:"name"`
}

// chunk is one streamed frame. Every field DeepSeek sends that this protocol
// has no home for is left out of the struct rather than decoded and dropped:
// object, created, system_fingerprint, logprobs and choices[].index.
//
// Error, ErrorType and ErrorMsg are defensive. DeepSeek documents no in band
// error frame on this endpoint at all, but the two shapes that circulate for
// OpenAI compatible hosts cost three fields to recognize and mistaking one for
// a content frame would end the turn as a completion.
type chunk struct {
	ID        string          `json:"id"`
	Model     string          `json:"model"`
	Choices   []chunkChoice   `json:"choices"`
	Usage     *usage          `json:"usage"`
	Error     json.RawMessage `json:"error"`
	ErrorType string          `json:"error_type"`
	ErrorMsg  string          `json:"error_message"`
}

type chunkChoice struct {
	Delta        delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

// delta is the incremental assistant message. Content and ReasoningContent are
// siblings and both arrive as null on frames that do not carry them, which
// decodes to the empty string and is why emptiness rather than presence is what
// opens a block.
type delta struct {
	Content          string      `json:"content"`
	ReasoningContent string      `json:"reasoning_content"`
	ToolCalls        []toolDelta `json:"tool_calls"`
}

// toolDelta is one fragment of one tool call. Index is the position among the
// calls of this turn and is what fragments are correlated by. Function is a
// pointer because a fragment that carries only an id has no function object at
// all, which is how a call's name can start arriving after its id.
type toolDelta struct {
	Index    int                `json:"index"`
	ID       string             `json:"id"`
	Function *toolDeltaFunction `json:"function"`
}

type toolDeltaFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

// usage is null on every frame but the last one before [DONE], which carries the
// whole turn's counters.
//
// The counters are inclusive, which is the opposite of what several other
// providers do: prompt_tokens is documented as equal to
// prompt_cache_hit_tokens + prompt_cache_miss_tokens, and reasoning_tokens is a
// breakdown inside completion_tokens rather than a sixth number to add.
type usage struct {
	Prompt     int `json:"prompt_tokens"`
	Completion int `json:"completion_tokens"`
	Total      int `json:"total_tokens"`
	CacheHit   int `json:"prompt_cache_hit_tokens"`
	CacheMiss  int `json:"prompt_cache_miss_tokens"`
	Details    *struct {
		Reasoning int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}
