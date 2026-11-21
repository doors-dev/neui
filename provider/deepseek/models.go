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

package deepseek

import "neui.dev/api"

// The models DeepSeek serves, ready to pass as Conf.Model, each with a
// 1,048,576-token window. The legacy ids deepseek-v4-flash and
// deepseek-v4-flash-vision-exp are still served by Flash; to have turns
// stamped with one, pass your own api.Model.
//
// Only Flash reads images: 600 per request, each downscaled to about 1300x1300
// pixels, at most 8192 pixels per side (4096 once a request carries 15 or more
// images), and 32 MiB per image, which ImageBytes declares as the raw size
// that encodes to it.
var (
	Flash = api.Model{Name: "deepseek-flash", Context: 1_048_576, ImagePixels: 1300 * 1300, ImageSide: 8192, ImageBytes: 24 * 1024 * 1024, ImagesPerRequest: 600, RequestBytes: 48 * 1024 * 1024, Inputs: api.Modalities{api.ModalityImage}}
	V4Pro = api.Model{Name: "deepseek-v4-pro", Context: 1_048_576, RequestBytes: 48 * 1024 * 1024}
)
