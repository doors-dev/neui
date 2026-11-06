package claude

import "neui.dev/api"

// The current Claude lineup and the two legacy models it replaced, ready to
// pass as Conf.Model, with the limits the models overview, the vision guide and
// the API overview publish for the Anthropic API
// (https://platform.claude.com/docs/en/about-claude/models/overview, read
// 2026-11-06). Fable 5.1, Opus 5.5, Sonnet 5.5 and Haiku 5.5 are current;
// Sonnet 5 and Haiku 4.5 are legacy and still served. Every id is a pinned
// snapshot; Haiku 4.5 is the one that still carries a date and the one with the
// smaller window and the standard resolution tier.
//
// ImageSide and ImagePixels are the vision guide's tiers: a 2576 px long edge
// on Claude 4.7 and later, 1568 px on Haiku 4.5. ImageBytes is the raw size of
// the API's 10 MB base64 limit per image, because the field is compared
// against the image's raw bytes. RequestBytes is the 128 MB the Anthropic API
// takes. These are the Anthropic API's limits only, the one API this package
// supports. For a model not listed, build an api.Model with its id and limits;
// New then checks none of its settings.
var (
	Fable51  = api.Model{Name: "claude-fable-5-1", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Opus55   = api.Model{Name: "claude-opus-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Sonnet55 = api.Model{Name: "claude-sonnet-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Haiku55  = api.Model{Name: "claude-haiku-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Sonnet5  = api.Model{Name: "claude-sonnet-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageSide: 2576, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Haiku45  = api.Model{Name: "claude-haiku-4-5-20251001", Context: 200_000, ImagePixels: 1568 * 28 * 28, ImageSide: 1568, ImageBytes: 7_500_000, ImagesPerRequest: 100, RequestBytes: 128_000_000}
)

type rules struct {
	maxTokens         int
	refusedThinking   []ThinkingType
	effort            bool
	disabledUpToHigh  bool
	refusesForcedTool bool
}

var declared = map[string]rules{
	Fable51.Name:  {maxTokens: 128_000, refusedThinking: []ThinkingType{ThinkingEnabled, ThinkingDisabled}, effort: true, refusesForcedTool: true},
	Opus55.Name:   {maxTokens: 128_000, refusedThinking: []ThinkingType{ThinkingEnabled, ThinkingDisabled}, effort: true, refusesForcedTool: true},
	Sonnet55.Name: {maxTokens: 128_000, refusedThinking: []ThinkingType{ThinkingEnabled, ThinkingDisabled}, effort: true, refusesForcedTool: true},
	Haiku55.Name:  {maxTokens: 128_000, refusedThinking: []ThinkingType{ThinkingEnabled}, effort: true, disabledUpToHigh: true},
	Sonnet5.Name:  {maxTokens: 128_000, refusedThinking: []ThinkingType{ThinkingEnabled}, effort: true},
	Haiku45.Name:  {maxTokens: 64_000, refusedThinking: []ThinkingType{ThinkingAdaptive}},
}
