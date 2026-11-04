package gpt

import "neui.dev/api"

// The current OpenAI models for Conf.Model, with their published limits
// (read 2026-11-05): a 1,050,000-token context window, of which at most 922,000
// tokens are input; 30,720,000 pixels per image; 1,500 images and 512 MB per
// request. The input_file parts of one request share a separate 50 MB, which
// api.Model has no field for. No byte limit is published for an image in a
// message; an image in a tool result is a data URL of at most 20,971,520
// characters, and ImageBytes is the raw size that encodes to it, so one value
// covers both places. Each takes at most 128,000 output tokens, which
// WithMaxTokens is checked against. For a gateway-prefixed id such as
// "openai/gpt-6-sol" or a newer model, declare your own api.Model.
var (
	GPT56Sol  = api.Model{Name: "gpt-5.6-sol", Context: 1_050_000, ImagePixels: imagePixels, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes}
	GPT56Luna = api.Model{Name: "gpt-5.6-luna", Context: 1_050_000, ImagePixels: imagePixels, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes}
	GPT6Astra = api.Model{Name: "gpt-6-astra", Context: 1_050_000, ImagePixels: imagePixels, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes}
	GPT6Sol   = api.Model{Name: "gpt-6-sol", Context: 1_050_000, ImagePixels: imagePixels, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes}
	GPT6Luna  = api.Model{Name: "gpt-6-luna", Context: 1_050_000, ImagePixels: imagePixels, ImageBytes: imageBytes, ImagesPerRequest: 1500, RequestBytes: requestBytes}
)

const (
	imagePixels            = 30_000 * 32 * 32
	imageBytes             = (20_971_520 - len("data:image/jpeg;base64,")) / 4 * 3
	requestBytes           = 512_000_000
	maxOutputTokensCeiling = 128_000
)
