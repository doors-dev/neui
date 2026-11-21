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

package gemini

import "encoding/json"

type request struct {
	Model             string            `json:"model"`
	Stream            bool              `json:"stream"`
	Store             bool              `json:"store"`
	Tools             []tool            `json:"tools,omitempty"`
	SystemInstruction string            `json:"system_instruction,omitempty"`
	Input             []any             `json:"input"`
	GenerationConfig  *generationConfig `json:"generation_config,omitempty"`
}

type tool struct {
	Type        string          `json:"type"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Parameters  json.RawMessage `json:"parameters"`
}

type generationConfig struct {
	MaxOutputTokens   int      `json:"max_output_tokens,omitempty"`
	StopSequences     []string `json:"stop_sequences,omitempty"`
	ThinkingLevel     string   `json:"thinking_level,omitempty"`
	ThinkingSummaries string   `json:"thinking_summaries,omitempty"`
	ToolChoice        any      `json:"tool_choice,omitempty"`
}

type toolChoiceConfig struct {
	AllowedTools allowedTools `json:"allowed_tools"`
}

type allowedTools struct {
	Mode  string   `json:"mode"`
	Tools []string `json:"tools"`
}

type userInput struct {
	Type    string `json:"type"`
	Content []any  `json:"content"`
}

type modelOutput struct {
	Type    string `json:"type"`
	Content []any  `json:"content"`
}

type thought struct {
	Type      string `json:"type"`
	Signature string `json:"signature"`
}

type functionCall struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type functionResult struct {
	Type    string `json:"type"`
	Name    string `json:"name"`
	CallID  string `json:"call_id"`
	Result  []any  `json:"result"`
	IsError bool   `json:"is_error,omitempty"`
}

type textContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type imageContent struct {
	Type     string `json:"type"`
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
}

type documentContent struct {
	Type     string `json:"type"`
	Data     []byte `json:"data"`
	MimeType string `json:"mime_type"`
}

type frameHead struct {
	EventType string `json:"event_type"`
}

type typeHead struct {
	Type string `json:"type"`
}

type streamEvent struct {
	EventType   string          `json:"event_type"`
	Index       *int            `json:"index"`
	Step        json.RawMessage `json:"step"`
	Delta       json.RawMessage `json:"delta"`
	Interaction *interaction    `json:"interaction"`
	Status      string          `json:"status"`
	Usage       *usage          `json:"usage"`
	Error       *wireError      `json:"error"`
}

type stepBody struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
	Signature string          `json:"signature"`
	Summary   []contentPart   `json:"summary"`
	Content   []contentPart   `json:"content"`
}

type deltaBody struct {
	Type      string       `json:"type"`
	Text      string       `json:"text"`
	Content   *contentPart `json:"content"`
	Signature string       `json:"signature"`
	Arguments string       `json:"arguments"`
}

type contentPart struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type interaction struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Usage  *usage `json:"usage"`
}

type usage struct {
	TotalInputTokens   int `json:"total_input_tokens"`
	TotalCachedTokens  int `json:"total_cached_tokens"`
	TotalOutputTokens  int `json:"total_output_tokens"`
	TotalThoughtTokens int `json:"total_thought_tokens"`
	TotalToolUseTokens int `json:"total_tool_use_tokens"`
}

type wireError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type errorBody struct {
	Error *errorDetail `json:"error"`
}

type errorDetail struct {
	Code    json.RawMessage `json:"code"`
	Message string          `json:"message"`
	Status  string          `json:"status"`
	Details []struct {
		Type   string `json:"@type"`
		Reason string `json:"reason"`
	} `json:"details"`
}
