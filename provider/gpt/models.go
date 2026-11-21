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

package gpt

import "neui.dev/api"

// The OpenAI models for Conf.Model: the current gpt-6.2-astra, gpt-6.2-sol,
// gpt-6.1-terra and gpt-6-luna, and the earlier models still served. Each has
// a 1,050,000-token context window, of which at most 922,000 tokens are input,
// and takes 1,500 images and 512 MB per request; the input_file parts of one
// request share a separate 50 MB, which api.Model has no field for. Images are
// sent at detail "high", which fits an image within 2,500 patches of 32x32
// pixels and, on gpt-5.6, 2048 pixels a side; gpt-6-astra allows a side of
// 65,535, and the other models are declared at the gpt-5.6 sizing. ImageBytes
// is the raw size that encodes to the 20,971,520-character data URL a tool
// result takes for an image, and covers messages too. Each model takes at most
// 128,000 output tokens, which WithMaxTokens is checked against. For a
// gateway-prefixed id such as "openai/gpt-6-sol" or a newer model, declare
// your own api.Model.
var (
	GPT56Sol   = api.Model{Name: "gpt-5.6-sol", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT56Luna  = api.Model{Name: "gpt-5.6-luna", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT6Astra  = api.Model{Name: "gpt-6-astra", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: 65_535, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT6Sol    = api.Model{Name: "gpt-6-sol", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT6Luna   = api.Model{Name: "gpt-6-luna", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT61Sol   = api.Model{Name: "gpt-6.1-sol", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT61Terra = api.Model{Name: "gpt-6.1-terra", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT62Astra = api.Model{Name: "gpt-6.2-astra", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
	GPT62Sol   = api.Model{Name: "gpt-6.2-sol", Context: 1_050_000, ImagePixels: imagePixels, ImageSide: imageSide, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes, Inputs: api.Modalities{api.ModalityImage, api.ModalityDocument}}
)

const (
	imagePixels            = 2_500 * 32 * 32
	imageSide              = 2048
	imageBytes             = (20_971_520 - len("data:image/jpeg;base64,")) / 4 * 3
	requestBytes           = 512_000_000
	maxOutputTokensCeiling = 128_000
)

type rules struct {
	maxTokens     int
	refusedEffort []Effort
}

var declared = map[string]rules{
	GPT56Sol.Name:   {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal}},
	GPT56Luna.Name:  {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal}},
	GPT6Astra.Name:  {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal, EffortNone}},
	GPT6Sol.Name:    {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal}},
	GPT6Luna.Name:   {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal}},
	GPT61Sol.Name:   {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal, EffortNone}},
	GPT61Terra.Name: {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal}},
	GPT62Astra.Name: {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal, EffortNone}},
	GPT62Sol.Name:   {maxTokens: maxOutputTokensCeiling, refusedEffort: []Effort{EffortMinimal, EffortNone}},
}
