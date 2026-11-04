package deepseek

import "neui.dev/api"

// The models DeepSeek serves, with the windows its pricing page publishes
// (https://api-docs.deepseek.com/quick_start/pricing, read 2026-09-24). Both
// v4 models read 1,048,576 tokens, which is the figure the server itself names
// when a conversation does not fit: "This model's maximum context length is
// 1048576 tokens." The pricing page rounds it to 1M, and a window declared 48k
// tokens wider than the real one is a compaction that runs too late. The legacy
// ids deepseek-v4-flash and deepseek-v4-flash-vision-exp are still accepted on
// the wire and served by Flash; a caller that needs them stamped as such passes
// its own api.Model.
//
// Neither declares any image capacity, because encode.go has no image path on
// either: content is a bare string on this wire and every ContentImage leaves a
// note in its place. The caps DeepSeek publishes for the vision surface —
// 32 MiB inline, 8192 px a side, 600 images a request — would have the caller
// downscale, re-encode and budget attachments that this adapter then drops, and
// none of that work is visible anywhere downstream. Declaring the caps is for
// whoever gives this adapter the typed content array a vision request needs.
var (
	Flash = api.Model{Name: "deepseek-flash", Context: 1_048_576, RequestBytes: 48 * 1024 * 1024}
	V4Pro = api.Model{Name: "deepseek-v4-pro", Context: 1_048_576, RequestBytes: 48 * 1024 * 1024}
)
