package claude

import "neui.dev/api"

// The current Claude lineup with the windows the models overview publishes
// (https://platform.claude.com/docs/en/about-claude/models/overview, read
// 2026-09-24). Every id is a pinned snapshot; Haiku 4.5 is the one that still
// carries a date and the one with the smaller window.
var (
	Fable51 = api.Model{Name: "claude-fable-5-1", Context: 1_000_000, ImagePixels: 1568 * 1568, ImageBytes: 10_000_000, ImagesPerRequest: 600, RequestBytes: 32_000_000}
	Opus55  = api.Model{Name: "claude-opus-5-5", Context: 1_000_000, ImagePixels: 1568 * 1568, ImageBytes: 10_000_000, ImagesPerRequest: 600, RequestBytes: 32_000_000}
	Sonnet5 = api.Model{Name: "claude-sonnet-5", Context: 1_000_000, ImagePixels: 1568 * 1568, ImageBytes: 10_000_000, ImagesPerRequest: 600, RequestBytes: 32_000_000}
	Haiku45 = api.Model{Name: "claude-haiku-4-5-20251001", Context: 200_000, ImagePixels: 1568 * 1568, ImageBytes: 10_000_000, ImagesPerRequest: 100, RequestBytes: 32_000_000}
)
