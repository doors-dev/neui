package gpt

import "neui.dev/api"

// The current OpenAI models for Conf.Model, with their published limits
// (read 2026-09-24): a 1,050,000-token context, 30,720,000 pixels per image,
// 1,500 images and 50 MB per request; ImageBytes is unpublished and left zero.
// For a gateway-prefixed id such as "openai/gpt-6-sol" or a newer model,
// declare your own api.Model.
var (
	GPT56Sol  = api.Model{Name: "gpt-5.6-sol", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT56Luna = api.Model{Name: "gpt-5.6-luna", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT6Astra = api.Model{Name: "gpt-6-astra", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT6Sol   = api.Model{Name: "gpt-6-sol", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
	GPT6Luna  = api.Model{Name: "gpt-6-luna", Context: 1_050_000, ImagePixels: 30_000 * 32 * 32, ImagesPerRequest: 1500, RequestBytes: 50_000_000}
)
