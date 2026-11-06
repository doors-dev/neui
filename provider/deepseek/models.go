package deepseek

import "neui.dev/api"

// The models DeepSeek serves, ready to pass as Conf.Model, with the windows its
// pricing page publishes (https://api-docs.deepseek.com/quick_start/pricing,
// read 2026-11-06). Both read 1,048,576 tokens, the exact figure the server
// names when a conversation does not fit. The legacy ids deepseek-v4-flash and
// deepseek-v4-flash-vision-exp are still served by Flash; to have turns stamped
// with one, pass your own api.Model.
//
// Only Flash reads images, and its image fields follow the vision guide
// (https://api-docs.deepseek.com/guides/vision, read 2026-11-06): 600 images
// per request, each downscaled to about 1300x1300 pixels, at most 4096 pixels
// per side once a request carries 15 or more images, and 32 MiB per image,
// which the guide does not say is counted before or after base64, so
// ImageBytes is the raw size that encodes to it.
var (
	Flash = api.Model{Name: "deepseek-flash", Context: 1_048_576, ImagePixels: 1300 * 1300, ImageSide: 4096, ImageBytes: 24 * 1024 * 1024, ImagesPerRequest: 600, RequestBytes: 48 * 1024 * 1024}
	V4Pro = api.Model{Name: "deepseek-v4-pro", Context: 1_048_576, RequestBytes: 48 * 1024 * 1024}
)
