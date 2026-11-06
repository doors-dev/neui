package gpt

type modelKind int

const (
	unknownModel modelKind = iota
	reasoningModel
)

func classify(m string) modelKind {
	switch m {
	case GPT56Sol.Name, GPT56Luna.Name, GPT6Astra.Name, GPT6Sol.Name, GPT6Luna.Name, GPT61Sol.Name, GPT61Terra.Name:
		return reasoningModel
	}
	return unknownModel
}
