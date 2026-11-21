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

import "neui.dev/api"

// The current Gemini models for Conf.Model, with the limits Google publishes,
// read 2026-11-19 from the model cards
// (https://aistudio.google.com/docs/models/gemini-4-hydrogen and its siblings),
// the token guide (https://aistudio.google.com/docs/tokens#context-window), the
// thinking guide (https://aistudio.google.com/docs/thinking#token-limits), the
// image guide (https://aistudio.google.com/docs/image-understanding#file-limit),
// the file input guide
// (https://aistudio.google.com/docs/file-input-methods#method-comparison) and
// the changelog (https://aistudio.google.com/docs/changelog#01-08-2026).
//
// The Gemini 4 models share one 1,048,576-token window between input,
// thinking and output. 3.8 Flash and 3.5 Flash-Lite take 1,048,576 input
// tokens plus 65,536 output tokens, so Context is their sum. A request takes
// up to 3,600 images and 100 MB inline. No pixel or per-image byte limit is
// published; each image costs a fixed token budget whatever its size. Each
// model returns at most 65,536 tokens in one streamed request, which
// WithMaxTokens is checked against.
//
// For a model not listed, build an api.Model with its id and limits; New then
// checks none of its settings.
var (
	Hydrogen4   = api.Model{Name: "gemini-4-hydrogen", Context: contextGemini4, ImagesPerRequest: imagesPerRequest, RequestBytes: requestBytes, Inputs: inputs}
	Neon4       = api.Model{Name: "gemini-4-neon", Context: contextGemini4, ImagesPerRequest: imagesPerRequest, RequestBytes: requestBytes, Inputs: inputs}
	Carbon4     = api.Model{Name: "gemini-4-carbon", Context: contextGemini4, ImagesPerRequest: imagesPerRequest, RequestBytes: requestBytes, Inputs: inputs}
	Argon4      = api.Model{Name: "gemini-4-argon", Context: contextGemini4, ImagesPerRequest: imagesPerRequest, RequestBytes: requestBytes, Inputs: inputs}
	Flash38     = api.Model{Name: "gemini-3.8-flash", Context: contextGemini3, ImagesPerRequest: imagesPerRequest, RequestBytes: requestBytes, Inputs: inputs}
	FlashLite35 = api.Model{Name: "gemini-3.5-flash-lite", Context: contextGemini3, ImagesPerRequest: imagesPerRequest, RequestBytes: requestBytes, Inputs: inputs}
)

const (
	contextGemini4   = 1_048_576
	contextGemini3   = 1_048_576 + 65_536
	maxOutput        = 65_536
	imagesPerRequest = 3_600
	requestBytes     = 100_000_000
	inputs           = api.ModalityText | api.ModalityImage | api.ModalityDocument
)

type rules struct {
	maxTokens int
	levels    []Effort
}

var declared = map[string]rules{
	Hydrogen4.Name:   {maxTokens: maxOutput, levels: []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh}},
	Neon4.Name:       {maxTokens: maxOutput, levels: []Effort{EffortLow, EffortMedium, EffortHigh, EffortXHigh}},
	Carbon4.Name:     {maxTokens: maxOutput, levels: []Effort{EffortLow, EffortMedium, EffortHigh}},
	Argon4.Name:      {maxTokens: maxOutput, levels: []Effort{EffortLow, EffortMedium, EffortHigh}},
	Flash38.Name:     {maxTokens: maxOutput, levels: []Effort{EffortLow, EffortMedium, EffortHigh}},
	FlashLite35.Name: {maxTokens: maxOutput, levels: []Effort{EffortMinimal, EffortLow, EffortMedium, EffortHigh}},
}
