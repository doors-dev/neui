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

import "neui.dev/api"

// The current Claude lineup and the legacy Sonnet 5, ready to pass as
// Conf.Model, with the limits the models overview, the vision guide and the API
// overview publish for the Anthropic API
// (https://platform.claude.com/docs/en/about-claude/models/overview, read
// 2026-11-06). Fable 5.1, Opus 5.5, Sonnet 5.5 and Haiku 5.5 are current;
// Sonnet 5 is legacy and still served. Every id is a pinned snapshot, and every
// model here takes adaptive thinking.
//
// ImageSide and ImagePixels are the vision guide's tier for Claude 4.7 and
// later, a 2576 px long edge. ImageBytes is the raw size of
// the API's 10 MB base64 limit per image, because the field is compared
// against the image's raw bytes. RequestBytes is 32 MB, below the 128 MB the
// API overview gives. These are the Anthropic API's limits only, the one API
// this package supports. For a model not listed, build an api.Model with its id
// and limits; New then checks none of its settings.
var (
	Fable51  = api.Model{Name: "claude-fable-5-1", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: inputs}
	Opus55   = api.Model{Name: "claude-opus-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: inputs}
	Sonnet55 = api.Model{Name: "claude-sonnet-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: inputs}
	Haiku55  = api.Model{Name: "claude-haiku-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: inputs}
	Sonnet5  = api.Model{Name: "claude-sonnet-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: inputs}
)

var inputs = api.Modalities{api.ModalityText, api.ModalityImage, api.ModalityDocument}

type rules struct {
	maxTokens         int
	refusesForcedTool bool
}

var declared = map[string]rules{
	Fable51.Name:  {maxTokens: 128_000, refusesForcedTool: true},
	Opus55.Name:   {maxTokens: 128_000, refusesForcedTool: true},
	Sonnet55.Name: {maxTokens: 128_000, refusesForcedTool: true},
	Haiku55.Name:  {maxTokens: 128_000},
	Sonnet5.Name:  {maxTokens: 128_000},
}
