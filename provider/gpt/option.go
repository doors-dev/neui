package gpt

import (
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const (
	defaultBaseURL = "https://api.openai.com/v1"
	apiKeyEnv      = "OPENAI_API_KEY"
)

// ReplayMode says which earlier reasoning items are sent back on later turns;
// set it with WithReasoningReplay. The default, ReplayToolTurns, suits every
// reasoning model the service documents.
type ReplayMode int

const (
	// ReplayToolTurns replays a reasoning item when a tool call follows it in
	// the same message, with any preamble or commentary messages in between;
	// the case the service requires. Default.
	ReplayToolTurns ReplayMode = iota
	// ReplayAlways replays every reasoning item that has an item after it in
	// the same message, including one before a text answer. If the service
	// rejects that pairing, every multi-turn conversation fails with a 400.
	ReplayAlways
	// ReplayNever sends no reasoning items back; tool calls and results still
	// travel. Use it for a model or gateway that rejects reasoning input
	// ("Reasoning input items can only be provided to a reasoning or computer
	// use model"). It costs quality on tool loops.
	ReplayNever
)

// settings are the knobs api.Request deliberately has no field for:
// sampling, reasoning effort, caching, and the rest of what belongs to the
// pairing of a provider with its model rather than to a turn.
type settings struct {
	maxTokens         int
	temperature       *float64
	topP              *float64
	effort            string
	summary           string
	reasoningMode     string
	reasoningContext  string
	replay            ReplayMode
	verbosity         string
	parallelToolCalls *bool
	strictTools       *bool
	store             bool
	include           []string
	promptCacheKey    string
	cacheMode         string
	cacheTTL          string
	serviceTier       string
	metadata          map[string]string
	obfuscation       bool
	transport         lib.Options
}

// defaultSettings is what a Provider starts from. The Responses API stores turns
// by default, and this adapter replays the whole thread from api.Message
// every time, so there is nothing to resume from, and stateless mode is also
// what makes reasoning items come back with encrypted_content populated.
func defaultSettings() settings {
	return settings{
		replay: ReplayToolTurns,
		store:  false,
	}
}

// Option configures a Provider in New. Every option has a working default.
type Option func(*Provider) error

// Conf holds what every Provider needs. Without Model.Name New returns an
// error. Use one of the models in this package, such as GPT56Luna,
// or an api.Model of your own for a gateway or a newer id.
type Conf struct {
	Model api.Model // the one model this provider streams with, spelled as it goes on the wire
}

// New builds a Provider. A nil option is skipped; an option with an invalid
// value, or one the declared model does not take, makes New return its error,
// as does an empty Conf.Model.Name. So do WithTemperature or WithTopP on a
// known reasoning model, one whose Name is exactly that of a model declared in
// this package, unless the reasoning effort is EffortNone. Any other id is
// unknown: New checks no combination for it and every option is sent as
// configured, so what the model refuses fails the turn.
// Without WithAPIKey, New reads the key from OPENAI_API_KEY once. If that is
// empty, it logs a warning and requests carry no Authorization header, which
// api.openai.com refuses with CodeAccess.
func New(conf Conf, opts ...Option) (*Provider, error) {
	p := &Provider{
		baseURL: defaultBaseURL,
		client:  &http.Client{},
		model:   conf.Model,
		set:     defaultSettings(),
	}
	for _, o := range opts {
		if o == nil {
			continue
		}
		if err := o(p); err != nil {
			return nil, err
		}
	}
	if p.model.Name == "" {
		return nil, errors.New("gpt: no model, set Conf.Model")
	}
	if classify(p.model.Name) == reasoningModel && (p.set.temperature != nil || p.set.topP != nil) && p.set.effort != string(EffortNone) {
		return nil, fmt.Errorf("gpt: %s is a reasoning model and takes temperature and top_p only with reasoning effort none", p.model.Name)
	}
	if !p.keyExplicit {
		p.apiKey = os.Getenv(apiKeyEnv)
		if p.apiKey == "" {
			slog.Warn("gpt: no API key: set " + apiKeyEnv + " or pass WithAPIKey")
		}
	}
	return p, nil
}

// WithAPIKey sets the key, and OPENAI_API_KEY is then not read. An empty key
// sends no Authorization header, for a gateway that authenticates some other
// way.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.apiKey = key
		p.keyExplicit = true
		return nil
	}
}

// WithBaseURL points the adapter at something other than api.openai.com: a
// gateway, a proxy, or a test server. The path "/responses" is appended, so
// pass the part that ends in "/v1".
func WithBaseURL(base string) Option {
	return func(p *Provider) error {
		if base == "" {
			return errors.New("gpt: WithBaseURL needs a URL")
		}
		u, err := url.Parse(base)
		if err != nil {
			return fmt.Errorf("gpt: WithBaseURL: %w", err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("gpt: WithBaseURL needs a scheme and host, got %q", base)
		}
		p.baseURL = strings.TrimRight(base, "/")
		return nil
	}
}

// WithHTTPClient supplies the client. Leave Timeout at zero: it bounds the
// whole exchange including the streamed body and will cut a healthy long turn.
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) error {
		if client == nil {
			return errors.New("gpt: WithHTTPClient needs a client")
		}
		p.client = client
		return nil
	}
}

// WithOrganization sends the OpenAI-Organization header, for a key that
// belongs to several organizations.
func WithOrganization(id string) Option {
	return func(p *Provider) error {
		p.org = id
		return nil
	}
}

// WithProject sends the OpenAI-Project header, for a key that spans several
// projects.
func WithProject(id string) Option {
	return func(p *Provider) error {
		p.project = id
		return nil
	}
}

// WithTransport sets the stream limits documented on lib.Options: how long the
// connection may stay silent and how large one frame may grow. A zero field
// keeps lib's default. The idle limit cannot be turned off, only raised; a turn
// that crosses it fails with CodeNetwork.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.set.transport = opt
		return nil
	}
}

// maxOutputTokensFloor is the schema minimum. A cap below it is a 400, and a cap
// below it is also meaningless.
const maxOutputTokensFloor = 16

// WithMaxTokens caps the response, reasoning included, so on a reasoning model
// a small cap can leave no room for the answer. Zero, the default, sends no cap.
// A negative value and values from 1 to 15, below the API minimum, are
// rejected, and so is a value over 128,000 when Conf.Model is one of the
// models declared in this package, which take no more.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 0 {
			return errors.New("gpt: WithMaxTokens must not be negative")
		}
		if n > 0 && n < maxOutputTokensFloor {
			return fmt.Errorf("gpt: WithMaxTokens is %d or more, the schema minimum, or zero for no cap", maxOutputTokensFloor)
		}
		switch p.model.Name {
		case GPT56Sol.Name, GPT56Luna.Name, GPT6Astra.Name, GPT6Sol.Name, GPT6Luna.Name, GPT61Sol.Name, GPT61Terra.Name:
			if n > maxOutputTokensCeiling {
				return fmt.Errorf("gpt: WithMaxTokens is %d, over the %d output tokens %s takes", n, maxOutputTokensCeiling, p.model.Name)
			}
		}
		p.set.maxTokens = n
		return nil
	}
}

// WithTemperature sets the sampling temperature, 0 to 2. New refuses it on a
// known reasoning model (see New) unless the reasoning effort is EffortNone.
func WithTemperature(v float64) Option {
	return func(p *Provider) error {
		if v < 0 || v > 2 {
			return errors.New("gpt: temperature is 0 to 2")
		}
		p.set.temperature = &v
		return nil
	}
}

// WithTopP sets nucleus sampling, 0 to 1. Like WithTemperature, New refuses
// it on a known reasoning model unless the reasoning effort is EffortNone.
func WithTopP(v float64) Option {
	return func(p *Provider) error {
		if v < 0 || v > 1 {
			return errors.New("gpt: top_p is 0 to 1")
		}
		p.set.topP = &v
		return nil
	}
}

// Effort is a reasoning.effort value for WithReasoningEffort. Any string
// passes through, so a value newer than these constants works too.
type Effort string

// Effort values, from no reasoning to the most.
const (
	EffortNone    Effort = "none"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortMax     Effort = "max"
)

// WithReasoningEffort sets reasoning.effort. Accepted values differ per model.
// When Conf.Model is one of the models declared in this package, the values
// its docs leave out are rejected: EffortMinimal on all of them, and EffortNone
// on gpt-6-astra and gpt-6.1-sol. For any other model nothing is checked, and
// an unsupported value fails the turn with a 400.
func WithReasoningEffort(effort Effort) Option {
	return func(p *Provider) error {
		switch m := p.model.Name; m {
		case GPT56Sol.Name, GPT56Luna.Name, GPT6Astra.Name, GPT6Sol.Name, GPT6Luna.Name, GPT61Sol.Name, GPT61Terra.Name:
			if effort == EffortMinimal || effort == EffortNone && (m == GPT6Astra.Name || m == GPT61Sol.Name) {
				return fmt.Errorf("gpt: %s does not take reasoning effort %q", p.model.Name, effort)
			}
		}
		p.set.effort = string(effort)
		return nil
	}
}

// Summary is a reasoning.summary value for WithReasoningSummary. Any string
// passes through.
type Summary string

// Summary values. SummaryAuto lets the service pick the detail.
const (
	SummaryAuto     Summary = "auto"
	SummaryConcise  Summary = "concise"
	SummaryDetailed Summary = "detailed"
)

// WithReasoningSummary asks for readable reasoning summaries, which arrive
// as EventSummary and fill ContentReasoning.Summary. Off by default, so
// Summary stays empty. Recent models require a verified organization and
// answer 400 otherwise.
func WithReasoningSummary(summary Summary) Option {
	return func(p *Provider) error {
		p.set.summary = string(summary)
		return nil
	}
}

// ReasoningMode is a reasoning.mode value for WithReasoningMode. Any string
// passes through.
type ReasoningMode string

// ReasoningMode values.
const (
	ReasoningStandard ReasoningMode = "standard"
	ReasoningPro      ReasoningMode = "pro"
)

// WithReasoningMode sets reasoning.mode, standard or pro, for the gpt-5.6 and
// gpt-6 families.
func WithReasoningMode(mode ReasoningMode) Option {
	return func(p *Provider) error {
		p.set.reasoningMode = string(mode)
		return nil
	}
}

// ReasoningContext is a reasoning.context value for WithReasoningContext. Any
// string passes through.
type ReasoningContext string

// ReasoningContext values.
const (
	// ContextAuto lets the service decide which earlier reasoning to use.
	ContextAuto ReasoningContext = "auto"
	// ContextCurrentTurn uses only the reasoning of the current turn.
	ContextCurrentTurn ReasoningContext = "current_turn"
	// ContextAllTurns uses the replayed reasoning of earlier turns too.
	ContextAllTurns ReasoningContext = "all_turns"
)

// WithReasoningContext sets reasoning.context: auto, current_turn or all_turns.
// all_turns only does anything if the earlier reasoning items are actually in
// the request, which here means WithReasoningReplay is not ReplayNever.
func WithReasoningContext(context ReasoningContext) Option {
	return func(p *Provider) error {
		p.set.reasoningContext = string(context)
		return nil
	}
}

// WithReasoningReplay picks which reasoning items go back on later turns.
func WithReasoningReplay(mode ReplayMode) Option {
	return func(p *Provider) error {
		switch mode {
		case ReplayToolTurns, ReplayAlways, ReplayNever:
		default:
			return fmt.Errorf("gpt: unknown replay mode %d", mode)
		}
		p.set.replay = mode
		return nil
	}
}

// Verbosity is a text.verbosity value for WithVerbosity, how long the answer
// text tends to be. Any string passes through.
type Verbosity string

// Verbosity values.
const (
	VerbosityLow    Verbosity = "low"
	VerbosityMedium Verbosity = "medium"
	VerbosityHigh   Verbosity = "high"
)

// WithVerbosity sets text.verbosity: low, medium or high.
func WithVerbosity(verbosity Verbosity) Option {
	return func(p *Provider) error {
		p.set.verbosity = string(verbosity)
		return nil
	}
}

// WithParallelToolCalls allows or forbids more than one tool call per turn.
// The service allows it unless told otherwise.
func WithParallelToolCalls(ok bool) Option {
	return func(p *Provider) error {
		p.set.parallelToolCalls = &ok
		return nil
	}
}

// WithStrictTools sets the strict flag on every tool. Unset, the service makes
// schemas strict where it can. True fails the turn with a 400 unless every
// object in each tool schema has additionalProperties:false and lists every
// property in required.
func WithStrictTools(strict bool) Option {
	return func(p *Provider) error {
		p.set.strictTools = &strict
		return nil
	}
}

// WithStore sets whether the service retains responses server-side. Off by
// default; this provider resends the whole thread each turn and needs nothing
// stored. With it on, reasoning items without encrypted content replay by id
// for as long as the service keeps the response.
func WithStore(store bool) Option {
	return func(p *Provider) error {
		p.set.store = store
		return nil
	}
}

// WithInclude replaces the include list. Without it, or with a nil list as
// WithInclude() gives, known reasoning models (see New) are sent
// "reasoning.encrypted_content", which older models need for reasoning
// replay; keep it in a list of your own, and pass it for any other model that
// reasons. Leaving it out while store is off drops reasoning from replay.
func WithInclude(values ...string) Option {
	return func(p *Provider) error {
		p.set.include = values
		return nil
	}
}

// WithPromptCacheKey steers cache routing for requests sharing a long prefix.
// OpenAI's prompt-caching guide asks for about fifteen requests a minute per
// key across all prefixes; above that, requests overflow to other machines
// and miss the cache.
func WithPromptCacheKey(key string) Option {
	return func(p *Provider) error {
		p.set.promptCacheKey = key
		return nil
	}
}

// CacheMode is a prompt_cache_options.mode value for WithPromptCacheOptions.
// Any string passes through.
type CacheMode string

// CacheMode values.
const (
	CacheImplicit CacheMode = "implicit"
	CacheExplicit CacheMode = "explicit"
)

// CacheTTL is a prompt_cache_options.ttl value for WithPromptCacheOptions.
// Any string passes through.
type CacheTTL string

// CacheTTL values, the least time a cached prefix is kept. CacheTTL30M is the
// service default.
const (
	CacheTTL30M CacheTTL = "30m"
	CacheTTL12H CacheTTL = "12h"
)

// WithPromptCacheOptions sets prompt_cache_options. Use it only with gpt-5.6
// and later; earlier models answer 400. Prefer CacheImplicit: CacheExplicit
// expects per-block cache breakpoints, which this provider does not place.
func WithPromptCacheOptions(mode CacheMode, ttl CacheTTL) Option {
	return func(p *Provider) error {
		p.set.cacheMode = string(mode)
		p.set.cacheTTL = string(ttl)
		return nil
	}
}

// ServiceTier is a service_tier value for WithServiceTier. Any string passes
// through.
type ServiceTier string

// ServiceTier values. TierFlex is cheaper and slower; TierPriority is faster
// and costs more, and TierFast asks for the same processing, which the
// response reports as priority. TierUltrafast is access-controlled and, as of
// 2026-11-05, offered only on gpt-5.6-sol.
const (
	TierAuto      ServiceTier = "auto"
	TierDefault   ServiceTier = "default"
	TierFlex      ServiceTier = "flex"
	TierScale     ServiceTier = "scale"
	TierPriority  ServiceTier = "priority"
	TierFast      ServiceTier = "fast"
	TierUltrafast ServiceTier = "ultrafast"
)

// WithServiceTier sets service_tier. Under TierFlex a turn fails with CodeBusy
// when there is no spare capacity, the same as a rate limit; back off and retry.
func WithServiceTier(tier ServiceTier) Option {
	return func(p *Provider) error {
		p.set.serviceTier = string(tier)
		return nil
	}
}

// WithMetadata sends m as metadata on every turn. The service takes up to
// sixteen pairs; the count is not checked here. m is kept, not copied, so do
// not change it afterwards.
func WithMetadata(m map[string]string) Option {
	return func(p *Provider) error {
		p.set.metadata = m
		return nil
	}
}

// WithObfuscation sets whether the service adds its random padding field to
// delta events to even out packet sizes. Off by default: the decoder ignores
// the field, so it only costs bandwidth.
func WithObfuscation(on bool) Option {
	return func(p *Provider) error {
		p.set.obfuscation = on
		return nil
	}
}
