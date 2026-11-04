package gpt

import "strings"

// baseModel drops a gateway prefix and lowercases. Model ids reach this adapter
// spelled "gpt-5.6-sol", "openai/gpt-5.6-sol" and "GPT-5.6-sol" depending on
// what sits in front of it, and all three have to classify the same way.
func baseModel(model string) string {
	if i := strings.LastIndexByte(model, '/'); i >= 0 {
		model = model[i+1:]
	}
	return strings.ToLower(strings.TrimSpace(model))
}

// inFamily matches a family prefix followed by a separator or nothing, so
// "o1" catches o1-pro and o1-2024-12-17 without catching a future o10.
func inFamily(model, name string) bool {
	if !strings.HasPrefix(model, name) {
		return false
	}
	rest := model[len(name):]
	if rest == "" {
		return true
	}
	switch rest[0] {
	case '-', '.', '_':
		return true
	}
	return false
}

// isReasoningModel decides two things at once, because one split governs both:
// a reasoning model refuses temperature and top_p with "Unsupported parameter:
// 'temperature' is not supported with this model.", and a model that is not one
// refuses a reasoning block with a 400 of its own. Sending either blindly earns
// a 400 that no retry fixes.
//
// The test is on the family prefix rather than a list of ids. The id field is
// an open string on the wire, dated snapshots and new tiers land without
// notice, and a table of exact names goes stale faster than this package will
// be edited. WithForcedSampling and WithReasoningReplay are the escape hatches
// for a name this guesses wrong.
//
// The *-chat-latest aliases are the exception that has to be named: they carry
// a gpt-5 prefix and route to a non reasoning model that does accept sampling.
func isReasoningModel(model string) bool {
	m := baseModel(model)
	if strings.Contains(m, "chat-latest") {
		return false
	}
	switch {
	case inFamily(m, "gpt-5"), inFamily(m, "gpt-6"),
		inFamily(m, "o1"), inFamily(m, "o3"), inFamily(m, "o4"),
		inFamily(m, "codex"), strings.Contains(m, "-codex"):
		return true
	}
	return false
}
