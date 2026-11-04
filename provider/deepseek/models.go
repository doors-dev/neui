package deepseek

import "neui.dev/api"

// The models DeepSeek serves, ready to pass as Conf.Model, with the windows its
// pricing page publishes (https://api-docs.deepseek.com/quick_start/pricing,
// read 2026-09-24). Both differ only in price and concurrency, and both read
// 1,048,576 tokens, the exact figure the server names when a conversation does
// not fit. The legacy ids deepseek-v4-flash and deepseek-v4-flash-vision-exp
// are still served by Flash; to have turns stamped with one, pass your own
// api.Model. Neither declares image capacity, because this adapter sends text
// only and leaves a note in place of every image.
var (
	Flash = api.Model{Name: "deepseek-flash", Context: 1_048_576, RequestBytes: 48 * 1024 * 1024}
	V4Pro = api.Model{Name: "deepseek-v4-pro", Context: 1_048_576, RequestBytes: 48 * 1024 * 1024}
)
