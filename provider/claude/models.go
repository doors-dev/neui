package claude

import "neui.dev/api"

// The current Claude lineup and the two legacy models it replaced, ready to
// pass as Conf.Model, with the limits the models overview, the vision guide and
// the API overview publish for the first party API
// (https://platform.claude.com/docs/en/about-claude/models/overview, read
// 2026-11-05). Fable 5.1, Opus 5.5, Sonnet 5.5 and Haiku 5.5 are current;
// Sonnet 5 and Haiku 4.5 are legacy and still served. Every id is a pinned
// snapshot; Haiku 4.5 is the one that still carries a date and the one with the
// smaller window and the standard resolution tier.
//
// ImageBytes is the raw size of the API's 10 MB base64 limit per image, because
// the field is compared against the image's raw bytes. RequestBytes is the
// 128 MB the first party API takes. Partner platforms take less: Claude Platform
// on AWS, Claude Models on Google Cloud (Anthropic managed) and Microsoft
// Foundry take 32 MB, Claude Models on Google Cloud (Google managed) 30 MB and
// Amazon Bedrock 20 MB, and the last two cap each image at 5 MB base64, so a
// provider pointed at one of them needs an api.Model of its own. For a model not
// listed, build an api.Model with its id and limits.
var (
	Fable51  = api.Model{Name: "claude-fable-5-1", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Opus55   = api.Model{Name: "claude-opus-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Sonnet55 = api.Model{Name: "claude-sonnet-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Haiku55  = api.Model{Name: "claude-haiku-5-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Sonnet5  = api.Model{Name: "claude-sonnet-5", Context: 1_000_000, ImagePixels: 4784 * 28 * 28, ImageBytes: 7_500_000, ImagesPerRequest: 600, RequestBytes: 128_000_000}
	Haiku45  = api.Model{Name: "claude-haiku-4-5-20251001", Context: 200_000, ImagePixels: 1568 * 28 * 28, ImageBytes: 7_500_000, ImagesPerRequest: 100, RequestBytes: 128_000_000}
)
