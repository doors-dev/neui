package conform

import (
	"fmt"
	"net/http"
	"strings"

	"neui.dev/api"
)

const (
	EffortOff  = "off"
	EffortLow  = "low"
	EffortHigh = "high"
	EffortMax  = "max"
)

const ChoiceNamed api.ToolChoice = "lookup"

type Params struct {
	Effort      string
	Choice      api.ToolChoice
	Temperature float64
	TopP        float64
	MaxTokens   int
	Key         string
}

func (p Params) String() string {
	var parts []string
	if p.Effort != "" {
		parts = append(parts, "effort="+p.Effort)
	}
	if p.Choice != "" {
		parts = append(parts, "choice="+string(p.Choice))
	}
	if p.Temperature != 0 {
		parts = append(parts, fmt.Sprintf("temperature=%v", p.Temperature))
	}
	if p.TopP != 0 {
		parts = append(parts, fmt.Sprintf("top_p=%v", p.TopP))
	}
	if p.MaxTokens != 0 {
		parts = append(parts, fmt.Sprintf("max_tokens=%d", p.MaxTokens))
	}
	if len(parts) == 0 {
		return "default"
	}
	return strings.Join(parts, ",")
}

func (p Params) Thinking() bool {
	return p.Effort != EffortOff
}

func (p Params) Forced() bool {
	return p.Choice == api.ToolChoiceAny || p.Choice == ChoiceNamed
}

func (p Params) Sampling() bool {
	return p.Temperature != 0 || p.TopP != 0
}

type Outcome int

const (
	Send Outcome = iota
	RefuseLocal
	Remote
)

func (o Outcome) String() string {
	switch o {
	case RefuseLocal:
		return "refused before sending"
	case Remote:
		return "sent, and documented to be refused by the API"
	}
	return "sent"
}

type Rule struct {
	Model  string
	Entry  string
	When   func(Params) bool
	Want   Outcome
	Carry  map[string]any
	Omit   []string
	Bug    string
	Source string
}

type Spec struct {
	File         string
	Request      string
	Count        string
	Event        string
	Error        string
	Version      string
	Private      []string
	EventPrivate []string
	Defaults     map[string]any
	LooseOneOf   bool
}

type Block struct {
	Kind      string
	Text      string
	Signature string
	ID        string
	Name      string
	Args      string
}

const (
	KindText     = "text"
	KindThinking = "thinking"
	KindSummary  = "summary"
	KindTool     = "tool"
	KindOpaque   = "opaque"
)

type Script struct {
	Name   string
	Blocks []Block
	Stop   api.ErrCode
}

type ErrorCase struct {
	Name   string
	Status int
	Header http.Header
	Body   string
	Stream string
	Want   api.ErrCode
	Schema bool
	Source string
}

type Live struct {
	Env    string
	Model  api.Model
	Params Params
	Base   string
	Count  string
	Strip  []string
	Auth   func(r *http.Request, key string)
}

type Adapter struct {
	Name       string
	Models     []api.Model
	New        func(base string, m api.Model, p Params) api.Provider
	Wire       map[string]string
	Spec       Spec
	Render     func(m api.Model, s Script) (string, bool)
	Errors     []ErrorCase
	Rules      []Rule
	BindsModel bool
	Signed     bool
	Unframed   bool
	Live       Live
}
