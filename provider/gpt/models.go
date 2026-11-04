package gpt

import "neui.dev/api"

// The current OpenAI lineup with the windows the model pages publish
// (https://developers.openai.com/api/docs/models, read 2026-09-24): 1,050,000
// tokens on every GPT-5.6 and GPT-6 model. A gateway-prefixed id such as
// "openai/gpt-6-sol" is a caller's own api.Model; baseModel still
// classifies it by family.
//
// ImagePixels is this service's own limit and not a sibling's: the images guide
// gives 30,000 patches of 32x32 pixels per image, exceeded by rejection rather
// than by a downscale, which as an area is 30,720,000 pixels. A smaller number
// here is a caller throwing away resolution the service would have read, and
// nothing downstream reports that it happened.
//
// ImageBytes stays zero, which is unknown. The old 20 and 50 MB per-image
// figures are gone from the docs and only the 512 MB whole-payload cap remains,
// so any number here would be invented, and an invented one sheds attachments
// the service accepts. 1,500 images a request and the 50 MB per request that
// bounds files are both published.
var (
	GPT56Sol  = api.Model{Name: "gpt-5.6-sol", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT56Luna = api.Model{Name: "gpt-5.6-luna", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT6Astra = api.Model{Name: "gpt-6-astra", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT6Sol   = api.Model{Name: "gpt-6-sol", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT6Luna  = api.Model{Name: "gpt-6-luna", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
)
