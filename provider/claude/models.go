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

// The current Claude models, Fable 5.2, Opus 5.5, Sonnet 5.5 and Haiku 5.5,
// and the legacy Fable 5.1 and Sonnet 5, ready to pass as Conf.Model with the
// Anthropic API's limits. Every id is a pinned snapshot, and every model here
// takes adaptive thinking.
//
// ImageSide and ImagePixels are the high-resolution tier, a 2576 px long edge.
// ImageBytes is the raw size that encodes to the 10 MB base64 limit per image.
// RequestBytes is 32 MB, below the API's 128 MB. For another platform or a
// model not listed, build an api.Model with its id and limits; New then checks
// none of its settings.
var (
	Fable52  = api.Model{Name: "claude-fable-5-2", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	Fable51  = api.Model{Name: "claude-fable-5-1", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	Opus55   = api.Model{Name: "claude-opus-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	Sonnet55 = api.Model{Name: "claude-sonnet-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	Haiku55  = api.Model{Name: "claude-haiku-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	Sonnet5  = api.Model{Name: "claude-sonnet-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 32_000_000, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
)

type rules struct {
	maxTokens         int
	refusesForcedTool bool
}

var declared = map[string]rules{
	Fable52.Name:  {maxTokens: 128_000, refusesForcedTool: true},
	Fable51.Name:  {maxTokens: 128_000, refusesForcedTool: true},
	Opus55.Name:   {maxTokens: 128_000, refusesForcedTool: true},
	Sonnet55.Name: {maxTokens: 128_000, refusesForcedTool: true},
	Haiku55.Name:  {maxTokens: 128_000},
	Sonnet5.Name:  {maxTokens: 128_000},
}
