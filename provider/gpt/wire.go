package gpt

import "encoding/json"

// requestBody is the POST /v1/responses body. Field order is the order the
// service renders the prompt in — tools, then instructions, then input — which
// is the order a cached prefix is matched in and the order api.Request
// already documents. The body is JSON, so the order costs nothing and reads
// like the thing it describes.
type requestBody struct {
	Model             string             `json:"model"`
	Stream            bool               `json:"stream"`
	StreamOptions     *streamOptions     `json:"stream_options,omitempty"`
	Store             bool               `json:"store"`
	Include           []string           `json:"include,omitempty"`
	Tools             []toolDef          `json:"tools,omitempty"`
	ToolChoice        any                `json:"tool_choice,omitempty"`
	ParallelToolCalls *bool              `json:"parallel_tool_calls,omitempty"`
	Instructions      string             `json:"instructions,omitempty"`
	MaxOutputTokens   int                `json:"max_output_tokens,omitempty"`
	Reasoning         *reasoningConfig   `json:"reasoning,omitempty"`
	Text              *textConfig        `json:"text,omitempty"`
	Temperature       *float64           `json:"temperature,omitempty"`
	TopP              *float64           `json:"top_p,omitempty"`
	PromptCacheKey    string             `json:"prompt_cache_key,omitempty"`
	PromptCacheOpts   *promptCacheConfig `json:"prompt_cache_options,omitempty"`
	ServiceTier       string             `json:"service_tier,omitempty"`
	Metadata          map[string]string  `json:"metadata,omitempty"`
	Input             []any              `json:"input"`
}

type streamOptions struct {
	IncludeObfuscation bool `json:"include_obfuscation"`
}

type reasoningConfig struct {
	Effort  string `json:"effort,omitempty"`
	Summary string `json:"summary,omitempty"`
	Mode    string `json:"mode,omitempty"`
	Context string `json:"context,omitempty"`
}

type textConfig struct {
	Verbosity string `json:"verbosity,omitempty"`
}

type promptCacheConfig struct {
	Mode string `json:"mode,omitempty"`
	TTL  string `json:"ttl,omitempty"`
}

// toolDef is flat: the JSON Schema goes straight into parameters. The nested
// function:{} wrapper belongs to Chat Completions and is a 400 here.
type toolDef struct {
	Type        string `json:"type"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
	Strict      *bool  `json:"strict,omitempty"`
}

type toolChoiceFunction struct {
	Type string `json:"type"`
	Name string `json:"name"`
}

// inputMessage is the easy message form, which takes every role and whose
// content is either one string or an array of input_text, input_image and
// input_file parts. The array is what this adapter sends, for the assistant
// role as much as for the user role: one api.Content block becomes one
// part, so the block boundaries the caller wrote survive, where a string holds
// one block and turns several into one joined by a separator the model never
// generated.
//
// The other message form is the output message, and it is the only one that
// takes an output_text part. It wants an id and a status on the message and
// annotations and logprobs on every part, none of which api.Content keeps,
// so assistant text goes back on the easy form instead, as output_text parts
// with the message's phase; the easy form takes output_text from the assistant
// and refuses input_text there. Both serialize as {"type":"message"} and the
// service reads the shape.
type inputMessage struct {
	Role    string `json:"role"`
	Content any    `json:"content"` // string, or an array: output_text for the assistant, input_text, input_image and input_file for the user
	Phase   string `json:"phase,omitempty"`
}

// inputText, inputImage and inputFile are the Responses spellings. Chat
// Completions' text/image_url/file are not accepted here.
type inputText struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// inputImage always sends detail. The schema marks it required while the
// documented example omits it, and it feeds tokenization, so a request that
// leaves it to a default cannot match the cached prefix of one that did not.
type inputImage struct {
	Type     string `json:"type"`
	ImageURL string `json:"image_url"`
	Detail   string `json:"detail"`
}

type inputFile struct {
	Type     string `json:"type"`
	Filename string `json:"filename,omitempty"`
	FileData string `json:"file_data"`
}

// functionCall and functionCallOutput are top level items, siblings of
// messages rather than blocks inside them. ID is the call_id, not the fc_ item
// id: only call_id links a call to its result.
type functionCall struct {
	Type      string `json:"type"`
	CallID    string `json:"call_id"`
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type functionCallOutput struct {
	Type   string `json:"type"`
	CallID string `json:"call_id"`
	Output any    `json:"output"` // string, or an array of input_text, input_image and input_file
}

// streamEvent is one SSE frame. Every field any handled event needs lives here
// flat, because the frames share a namespace and separate types per event would
// be fifty declarations to read one string out of each.
//
// The event: line duplicates type, so type is what this dispatches on: it is
// authoritative and always present.
//
// An in band error arrives in two shapes. The documented one is flat, with code
// and message at the top level; the one seen on the wire nests them under error.
// Both official SDKs only detect the nested form. Both are read here.
type streamEvent struct {
	Type         string          `json:"type"`
	OutputIndex  int             `json:"output_index"`
	ContentIndex int             `json:"content_index"`
	SummaryIndex int             `json:"summary_index"`
	Delta        string          `json:"delta"`
	Arguments    string          `json:"arguments"`
	Item         json.RawMessage `json:"item"`
	Response     *responseObject `json:"response"`
	Code         string          `json:"code"`
	Message      string          `json:"message"`
	Param        string          `json:"param"`
	Error        *wireError      `json:"error"`
}

type responseObject struct {
	ID                string     `json:"id"`
	Model             string     `json:"model"`
	Status            string     `json:"status"`
	Usage             *wireUsage `json:"usage"`
	Error             *wireError `json:"error"`
	IncompleteDetails *struct {
		Reason string `json:"reason"`
	} `json:"incomplete_details"`
}

type wireError struct {
	Type    string `json:"type"`
	Code    string `json:"code"`
	Message string `json:"message"`
	Param   string `json:"param"`
}

// wireUsage reports its details relative to their parents: cached_tokens and
// cache_write_tokens are both part of input_tokens, reasoning_tokens is part of
// output_tokens. api.EventUsage wants the five disjoint, so the adapter
// subtracts rather than passing these through.
type wireUsage struct {
	InputTokens        int `json:"input_tokens"`
	OutputTokens       int `json:"output_tokens"`
	InputTokensDetails struct {
		CachedTokens     int `json:"cached_tokens"`
		CacheWriteTokens int `json:"cache_write_tokens"`
	} `json:"input_tokens_details"`
	OutputTokensDetails struct {
		ReasoningTokens int `json:"reasoning_tokens"`
	} `json:"output_tokens_details"`
}

// outputItem is the item carried by response.output_item.added and .done, read
// only for what the item says that its own delta events do not: a message's
// phase, a function call's identity, a reasoning item's seal, whether the item
// finished, and the type every other kind of item is carried and sent back
// under.
//
// Status is in_progress while the item is being produced, completed when it is,
// and incomplete on an item the turn cut short. It decides whether the block an
// item becomes whole is closed; the blocks that close on an event of their own
// read it nowhere.
type outputItem struct {
	Type             string `json:"type"`
	ID               string `json:"id"`
	Status           string `json:"status"`
	Phase            string `json:"phase"`
	CallID           string `json:"call_id"`
	Name             string `json:"name"`
	EncryptedContent string `json:"encrypted_content"`
}
