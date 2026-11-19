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

package claude

import (
	"encoding/json"
	"encoding/json/jsontext"
	"unicode/utf8"
)

// request is the body of POST /v1/messages, and the types under it are the rest
// of that API's wire shapes. Only the fields this adapter reads or writes are
// declared: the versioning policy promises new fields and new event names with
// every release, so decoding ignores what it does not know rather than failing
// on it.
type request struct {
	Model         string        `json:"model"`
	MaxTokens     int           `json:"max_tokens"`
	Stream        bool          `json:"stream"`
	Tools         []tool        `json:"tools,omitempty"`
	ToolChoice    *toolChoice   `json:"tool_choice,omitempty"`
	System        []textBlock   `json:"system,omitempty"`
	Messages      []message     `json:"messages"`
	StopSequences []string      `json:"stop_sequences,omitempty"`
	Thinking      *thinkingWire `json:"thinking,omitempty"`
	OutputConfig  *outputConfig `json:"output_config,omitempty"`
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
	Type         string        `json:"type"`
	Budget       int           `json:"budget_tokens,omitempty"`
	Display      string        `json:"display,omitempty"`
	BlockBinding *blockBinding `json:"block_binding,omitempty"`
}

type blockBinding struct {
	PrefixMismatchBehavior string `json:"prefix_mismatch_behavior"`
}

type outputConfig struct {
	Effort string `json:"effort,omitempty"`
}

type cacheControl struct {
	Type string `json:"type"`
	TTL  string `json:"ttl,omitempty"`
}

// message content holds typed blocks, encoded once with the request, and the
// raw bytes of a replayed redacted_thinking or opaque block, which has to go
// back out exactly as it came in.
type message struct {
	Role    string `json:"role"`
	Content []any  `json:"content"`
}

type cached struct {
	CacheControl *cacheControl `json:"cache_control,omitempty"`
}

type textBlock struct {
	cached
	Type string   `json:"type"`
	Text htmlText `json:"text"`
}

type base64Source struct {
	Type      string `json:"type"`
	MediaType string `json:"media_type"`
	Data      []byte `json:"data"`
}

type textSource struct {
	Type      string   `json:"type"`
	MediaType string   `json:"media_type"`
	Data      htmlText `json:"data"`
}

type imageBlock struct {
	cached
	Type   string       `json:"type"`
	Source base64Source `json:"source"`
}

type documentBlock struct {
	cached
	Type   string   `json:"type"`
	Source any      `json:"source"`
	Title  htmlText `json:"title,omitempty"`
}

type toolUseBlock struct {
	cached
	Type  string          `json:"type"`
	ID    htmlText        `json:"id"`
	Name  htmlText        `json:"name"`
	Input json.RawMessage `json:"input"`
}

type toolResultBlock struct {
	cached
	Type      string   `json:"type"`
	ToolUseID htmlText `json:"tool_use_id"`
	Content   []any    `json:"content"`
	IsError   bool     `json:"is_error"`
}

type thinkingBlock struct {
	Type      string `json:"type"`
	Thinking  string `json:"thinking"`
	Signature string `json:"signature"`
}

type htmlText string

func (s htmlText) MarshalJSONTo(enc *jsontext.Encoder) error {
	return enc.WriteValue(appendHTMLText(enc.AvailableBuffer(), string(s)))
}

func appendHTMLText(dst []byte, s string) []byte {
	const hex = "0123456789abcdef"
	dst = append(dst, '"')
	start := 0
	for i := 0; i < len(s); {
		if b := s[i]; b < utf8.RuneSelf {
			if b >= ' ' && b != '"' && b != '\\' && b != '<' && b != '>' && b != '&' {
				i++
				continue
			}
			dst = append(dst, s[start:i]...)
			switch b {
			case '"', '\\':
				dst = append(dst, '\\', b)
			case '\b':
				dst = append(dst, '\\', 'b')
			case '\f':
				dst = append(dst, '\\', 'f')
			case '\n':
				dst = append(dst, '\\', 'n')
			case '\r':
				dst = append(dst, '\\', 'r')
			case '\t':
				dst = append(dst, '\\', 't')
			default:
				dst = append(dst, '\\', 'u', '0', '0', hex[b>>4], hex[b&0xf])
			}
			i++
			start = i
			continue
		}
		r, size := utf8.DecodeRuneInString(s[i:])
		switch {
		case r == utf8.RuneError && size == 1:
			dst = append(dst, s[start:i]...)
			dst = utf8.AppendRune(dst, utf8.RuneError)
		case r == 0x2028 || r == 0x2029:
			dst = append(dst, s[start:i]...)
			dst = append(dst, '\\', 'u', '2', '0', '2', hex[r&0xf])
		default:
			i += size
			continue
		}
		i += size
		start = i
	}
	dst = append(dst, s[start:]...)
	return append(dst, '"')
}

// frameHead is read before anything else in a frame, so a frame type this
// adapter does not know is never decoded further.
type frameHead struct {
	Type string `json:"type"`
}

// Streaming events. Every frame repeats its SSE name in this type field, and
// message_delta and content_block_delta both call their payload "delta", which
// is why Delta holds the fields of both and the frame type says which to read.
// Index is a pointer because the block frames require it and absent must not
// read as 0.
type sseMessage struct {
	Type         string          `json:"type"`
	Index        *int            `json:"index"`
	Message      *startMessage   `json:"message"`
	ContentBlock json.RawMessage `json:"content_block"`
	Delta        delta           `json:"delta"`
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

type delta struct {
	Type        string  `json:"type"`
	Text        string  `json:"text"`
	PartialJSON string  `json:"partial_json"`
	Thinking    string  `json:"thinking"`
	Signature   string  `json:"signature"`
	StopReason  *string `json:"stop_reason"`
}
