package gpt

import "strings"

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

type modelKind int

const (
	unknownModel modelKind = iota
	reasoningModel
	plainModel
)

// classify splits the known families into the two kinds that refuse each
// other's options: a reasoning model refuses temperature and top_p with
// "Unsupported parameter: 'temperature' is not supported with this model.",
// and a model that is not one refuses a reasoning block with a 400 of its own.
// Any other id is unknown, and nothing is checked for it.
//
// The test is on the family prefix rather than a list of ids, because dated
// snapshots and new tiers of a family land without notice. The *-chat-latest
// aliases are the exception that has to be named: they carry a gpt-5 prefix
// and route to a non reasoning model that does accept sampling. The id is read
// as it is spelled, so a gateway prefix or another case is an unknown model.
func classify(m string) modelKind {
	if strings.Contains(m, "chat-latest") {
		return plainModel
	}
	switch {
	case inFamily(m, "gpt-5"), inFamily(m, "gpt-6"),
		inFamily(m, "o1"), inFamily(m, "o3"), inFamily(m, "o4"),
		inFamily(m, "codex"), strings.Contains(m, "-codex"):
		return reasoningModel
	case inFamily(m, "gpt-4"), inFamily(m, "gpt-4o"), inFamily(m, "gpt-3.5"), inFamily(m, "chatgpt"):
		return plainModel
	}
	return unknownModel
}
