package claude

import "encoding/json"

// request is the body of POST /v1/messages, and the types under it are the rest
// of that API's wire shapes. Only the fields this adapter reads or writes are
// declared: the versioning policy promises new fields and new event names with
// every release, so decoding ignores what it does not know rather than failing
// on it.
type request struct {
	Model         string            `json:"model"`
	MaxTokens     int               `json:"max_tokens"`
	Stream        bool              `json:"stream"`
	Tools         []tool            `json:"tools,omitempty"`
	ToolChoice    *toolChoice       `json:"tool_choice,omitempty"`
	System        []json.RawMessage `json:"system,omitempty"`
	Messages      []message         `json:"messages"`
	Temperature   *float64          `json:"temperature,omitempty"`
	TopP          *float64          `json:"top_p,omitempty"`
	TopK          *int              `json:"top_k,omitempty"`
	StopSequences []string          `json:"stop_sequences,omitempty"`
	Thinking      *thinkingWire     `json:"thinking,omitempty"`
	OutputConfig  *outputConfig     `json:"output_config,omitempty"`
}

type tool struct {
	Name         string          `json:"name"`
	Description  string          `json:"description,omitempty"`
	InputSchema  json.RawMessage `json:"input_schema"`
	CacheControl *cacheControl   `json:"cache_control,omitempty"`
}

type toolChoice struct {
	Type string `json:"type"`
	Name string `json:"name,omitempty"`
}

type thinkingWire struct {
	Type    string `json:"type"`
	Budget  int    `json:"budget_tokens,omitempty"`
	Display string `json:"display,omitempty"`
}

type outputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

// message content is raw because a replayed thinking or redacted_thinking block
// has to go back out exactly as it came in. Everything else is marshalled from
// a typed block and lands here as bytes too, so one slice covers both.
type message struct {
	Role    string            `json:"role"`
	Content []json.RawMessage `json:"content"`
}

type textBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type mediaSource struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      string `json:"data"`
}

type imageBlock struct {
	Type   string      `json:"type"`
	Source mediaSource `json:"source"`
}

type documentBlock struct {
	Type   string      `json:"type"`
	Source mediaSource `json:"source"`
	Title  string      `json:"title,omitempty"`
}

type toolUseBlock struct {
	Type  string          `json:"type"`
	ID    string          `json:"id"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
	Type      string            `json:"type"`
	ToolUseID string            `json:"tool_use_id"`
	Content   []json.RawMessage `json:"content"`
	IsError   bool              `json:"is_error"`
}

type thinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

// Streaming events. Every frame repeats its SSE name in this type field, and
// message_delta and content_block_delta both call their payload "delta", which
// is why Delta stays raw until the frame type says how to read it.
type sseMessage struct {
	Type         string          `json:"type"`
	Index        int             `json:"index"`
	Message      *startMessage   `json:"message"`
	ContentBlock json.RawMessage `json:"content_block"`
	Delta        json.RawMessage `json:"delta"`
	Usage        *wireUsage      `json:"usage"`
	Error        *wireError      `json:"error"`
}

type startMessage struct {
	ID    string     `json:"id"`
	Model string     `json:"model"`
	Usage *wireUsage `json:"usage"`
}

// Every counter is a pointer because message_delta reports only what changed
// and message_start sometimes reports nothing at all, so absent and zero mean
// different things and the adapter has to carry the last known value forward.
type wireUsage struct {
	InputTokens              *int `json:"input_tokens"`
	OutputTokens             *int `json:"output_tokens"`
	CacheCreationInputTokens *int `json:"cache_creation_input_tokens"`
	CacheReadInputTokens     *int `json:"cache_read_input_tokens"`
	OutputTokensDetails      *struct {
		ThinkingTokens *int `json:"thinking_tokens"`
	} `json:"output_tokens_details"`
}

type wireError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Details *struct {
		ErrorCode string `json:"error_code"`
	} `json:"details"`
}

// errorEnvelope is the body of a non 200 response. The same inner object
// arrives in band on a stream that already returned 200, minus request_id.
type errorEnvelope struct {
	Type      string     `json:"type"`
	Error     *wireError `json:"error"`
	RequestID string     `json:"request_id"`
}

// blockStart is read only to tell the block kinds apart and to name a tool
// call. A block whose payload goes back to the API is kept as the bytes it
// arrived in rather than through these fields.
type blockStart struct {
	Type string `json:"type"`
	ID   string `json:"id"`
	Name string `json:"name"`
}

type blockDelta struct {
	Type        string `json:"type"`
	Text        string `json:"text"`
	PartialJSON string `json:"partial_json"`
	Thinking    string `json:"thinking"`
	Signature   string `json:"signature"`
}

type messageDelta struct {
	StopReason *string `json:"stop_reason"`
}
