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

package generic

import "encoding/json"

type chatRequest struct {
	Model           string         `json:"model"`
	Tools           []tool         `json:"tools,omitempty"`
	ToolChoice      any            `json:"tool_choice,omitempty"`
	Messages        []message      `json:"messages"`
	Stream          bool           `json:"stream"`
	StreamOptions   *streamOptions `json:"stream_options"`
	MaxTokens       int            `json:"max_tokens,omitempty"`
	ReasoningEffort string         `json:"reasoning_effort,omitempty"`
	Temperature     *float64       `json:"temperature,omitempty"`
	TopP            *float64       `json:"top_p,omitempty"`
	Stop            []string       `json:"stop,omitempty"`
}

type streamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

type message struct {
	Role             string     `json:"role"`
	Content          any        `json:"content"`
	Reasoning        string     `json:"reasoning,omitempty"`
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

type toolCallFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type tool struct {
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters"`
}

type namedToolChoice struct {
	Type     string            `json:"type"`
	Function namedToolFunction `json:"function"`
}

type namedToolFunction struct {
	Name string `json:"name"`
}

type chunk struct {
	ID      string          `json:"id"`
	Choices []chunkChoice   `json:"choices"`
	Usage   *usage          `json:"usage"`
	Error   json.RawMessage `json:"error"`
}

type chunkChoice struct {
	Index        int     `json:"index"`
	Delta        delta   `json:"delta"`
	FinishReason *string `json:"finish_reason"`
}

type delta struct {
	Content          string      `json:"content"`
	Reasoning        string      `json:"reasoning"`
	ReasoningContent string      `json:"reasoning_content"`
	ToolCalls        []toolDelta `json:"tool_calls"`
}

type toolDelta struct {
	Index    *int               `json:"index"`
	ID       string             `json:"id"`
	Function *toolDeltaFunction `json:"function"`
}

type toolDeltaFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type usage struct {
	Prompt        int `json:"prompt_tokens"`
	Completion    int `json:"completion_tokens"`
	PromptDetails *struct {
		Cached     int `json:"cached_tokens"`
		CacheWrite int `json:"cache_write_tokens"`
	} `json:"prompt_tokens_details"`
	CompletionDetails *struct {
		Reasoning int `json:"reasoning_tokens"`
	} `json:"completion_tokens_details"`
}

type errorBody struct {
	Message  string          `json:"message"`
	Type     string          `json:"type"`
	Code     json.RawMessage `json:"code"`
	Param    json.RawMessage `json:"param"`
	Metadata struct {
		ErrorType string `json:"error_type"`
	} `json:"metadata"`
}
