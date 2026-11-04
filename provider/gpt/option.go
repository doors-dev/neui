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

// ReplayMode says when a reasoning item is sent back on a later turn.
//
// The service validates a reasoning item against the item that followed it in
// the response it came from: sending one with nothing behind it earns "Item
// 'rs_…' of type 'reasoning' was provided without its required following item".
// The refusal in the other direction — "'function_call' was provided without its
// required 'reasoning' item" — names the call by the id field of the
// function_call item, which this adapter's function_call does not carry, so a
// call is sent whatever happens to the thinking in front of it and the only
// question left is which reasoning items to replay at all.
type ReplayMode int

const (
	// ReplayToolTurns replays a reasoning item only when a tool call followed
	// it. That is the case the service documents as required. For a turn that
	// ended in text, replay is documented as optional and irrelevant items are
	// discarded server side, and an assistant message is replayed here without
	// the msg_ id it was issued with, which is the one thing about the pairing
	// check that is not documented. Default, because it takes the documented
	// benefit without betting on the undocumented half.
	ReplayToolTurns ReplayMode = iota
	// ReplayAlways replays every reasoning item that has something to carry.
	// Correct if an assistant message satisfies the pairing check without its
	// original id; a 400 on every multi turn conversation if it does not.
	ReplayAlways
	// ReplayNever sends no reasoning items back. The calls they produced still
	// travel, with the results answering them, because nothing in the thread
	// depends on a reasoning item having been sent.
	//
	// It is what a model that cannot take reasoning input at all is served with:
	// "Reasoning input items can only be provided to a reasoning or computer use
	// model." There is no table of which models those are, because the id field
	// is an open string, tiers land without notice, and a table that is wrong
	// fails the request it was written to save. It is also the escape hatch for
	// a gateway that rejects the items. Costs quality on tool loops.
	ReplayNever
)

// settings are the knobs api.Request deliberately has no field for:
// sampling, reasoning effort, caching, and the rest of what belongs to the
// pairing of a provider with its model rather than to a turn.
type settings struct {
	maxTokens         int
	temperature       *float64
	topP              *float64
	forceSampling     bool
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

// Option configures a Provider. Everything with a working default is an option:
// the key falls back to the environment, the base URL and the rest have values
// that stream as they are.
type Option func(*Provider) error

// Conf is what a Provider cannot be built without, so the compiler refuses a
// half configured one. It is a struct rather than a bare argument so that a
// second mandatory field later does not break every call site.
type Conf struct {
	Model api.Model // the one model this provider streams with, spelled as it goes on the wire
}

// New builds a Provider. The key comes from OPENAI_API_KEY unless WithAPIKey
// says otherwise; a missing key is only warned about here, and the auth
// failure on the first turn remains the real signal.
//
// An option that reports an error is logged and skipped, so the knob it would
// have set keeps its prior or default value.
//
// An empty Conf.Model is not refused here: it is the same omission as an empty
// model on the wire and is reported the same way, as a refusal to build the
// turn.
//
// The client is built with no Timeout. It is a deadline on the whole exchange
// including the body, so any value caps how long a turn may run rather than how
// long it may go quiet, and the idle budget WithTransport carries is the knob
// that means what a timeout is usually wanted for.
func New(conf Conf, opts ...Option) *Provider {
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
			slog.Error("gpt: option rejected", "err", err)
		}
	}
	if !p.keyExplicit {
		p.apiKey = os.Getenv(apiKeyEnv)
		if p.apiKey == "" {
			slog.Warn("gpt: no API key: set " + apiKeyEnv + " or pass WithAPIKey")
		}
	}
	return p
}

// WithAPIKey sets the key. An empty string means send no Authorization header
// at all, which is how a gateway that authenticates some other way is reached
// without inventing a placeholder credential.
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

// WithOrganization and WithProject set the headers a key spanning several of
// either needs to disambiguate itself.
func WithOrganization(id string) Option {
	return func(p *Provider) error {
		p.org = id
		return nil
	}
}

func WithProject(id string) Option {
	return func(p *Provider) error {
		p.project = id
		return nil
	}
}

// WithTransport sets the limits lib keeps on a turn: how long the wire may stay
// silent, and how many bytes one frame may accumulate. lib owns the mechanism
// and the defaults, so the fields, their values and the reasoning for them are
// documented on lib.Options and a field left at zero takes lib's default here
// rather than one of this adapter's.
//
// The idle half is always on and there is no value that turns it off; a caller
// who wants a turn to run effectively unbounded passes an hour. Any bytes on the
// connection reset it, including the SSE comment lines the service sends to hold
// a queued request open, so it measures a dead socket rather than a slow turn,
// and a turn that crosses it fails with CodeNetwork.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.set.transport = opt
		return nil
	}
}

// maxOutputTokensFloor is the schema minimum. A cap below it is a 400, and a cap
// below it is also meaningless.
const maxOutputTokensFloor = 16

// WithMaxTokens caps the response. It counts reasoning as well as visible
// output, so on a reasoning model a small value produces a turn that is entirely
// thinking and no answer. Zero, the default, sends no cap and leaves the service
// its own.
//
// A value under the schema minimum is refused here rather than quietly raised to
// it: whoever configures a provider is there to be told, where a turn is not.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 0 {
			return errors.New("gpt: WithMaxTokens must not be negative")
		}
		if n > 0 && n < maxOutputTokensFloor {
			return fmt.Errorf("gpt: WithMaxTokens is %d or more, the schema minimum, or zero for no cap", maxOutputTokensFloor)
		}
		p.set.maxTokens = n
		return nil
	}
}

// WithTemperature and WithTopP set sampling. Both are dropped on reasoning
// models, which answer "Unsupported parameter: 'temperature' is not supported
// with this model." to either, so setting one on a Provider configured with
// gpt-5 or an o-series model is not an error, it just does not reach the wire.
// WithForcedSampling overrides that.
func WithTemperature(v float64) Option {
	return func(p *Provider) error {
		if v < 0 || v > 2 {
			return errors.New("gpt: temperature is 0 to 2")
		}
		p.set.temperature = &v
		return nil
	}
}

func WithTopP(v float64) Option {
	return func(p *Provider) error {
		if v < 0 || v > 1 {
			return errors.New("gpt: top_p is 0 to 1")
		}
		p.set.topP = &v
		return nil
	}
}

// WithForcedSampling sends sampling parameters whatever the model is. For a
// gateway serving a model this adapter classifies wrongly, or for the reported
// case of gpt-5.1 accepting them at effort "none", which is not in the current
// documentation.
func WithForcedSampling() Option {
	return func(p *Provider) error {
		p.set.forceSampling = true
		return nil
	}
}

// Effort names a reasoning.effort value. It is an open string rather than a
// closed enum so a value a future release adds passes through without a new
// constant here.
type Effort string

const (
	EffortNone    Effort = "none"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortMax     Effort = "max"
)

// WithReasoningEffort sets reasoning.effort: none, minimal, low, medium, high,
// xhigh or max. Which values a model accepts differs per model and per release
// and an unsupported one is a 400, so this is not validated against a table
// that would be wrong within a month. It is dropped on non reasoning models,
// which reject a reasoning block outright.
func WithReasoningEffort(effort Effort) Option {
	return func(p *Provider) error {
		p.set.effort = string(effort)
		return nil
	}
}

// Summary names a reasoning.summary value. Open like Effort: custom strings
// pass through.
type Summary string

const (
	SummaryAuto     Summary = "auto"
	SummaryConcise  Summary = "concise"
	SummaryDetailed Summary = "detailed"
)

// WithReasoningSummary opts into readable reasoning: auto, concise or detailed.
// Without it a reasoning item comes back with an empty summary and
// ContentReasoning.Summary stays empty for every turn, which is the case
// api.ContentReasoning documents as legitimate rather than a bug. It is
// off by default because the summarizers on recent models are gated on
// organization verification and a default that 400s an unverified caller on
// every request is worse than an empty field.
func WithReasoningSummary(summary Summary) Option {
	return func(p *Provider) error {
		p.set.summary = string(summary)
		return nil
	}
}

// ReasoningMode names a reasoning.mode value. Open like Effort: custom strings
// pass through.
type ReasoningMode string

const (
	ReasoningStandard ReasoningMode = "standard"
	ReasoningPro      ReasoningMode = "pro"
)

// WithReasoningMode sets reasoning.mode, standard or pro. gpt-5.6 family only.
func WithReasoningMode(mode ReasoningMode) Option {
	return func(p *Provider) error {
		p.set.reasoningMode = string(mode)
		return nil
	}
}

// ReasoningContext names a reasoning.context value. Open like Effort: custom
// strings pass through.
type ReasoningContext string

const (
	ContextAuto        ReasoningContext = "auto"
	ContextCurrentTurn ReasoningContext = "current_turn"
	ContextAllTurns    ReasoningContext = "all_turns"
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

// Verbosity names a text.verbosity value. Open like Effort: custom strings
// pass through.
type Verbosity string

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

// WithStrictTools decides the strict flag on every tool. Unset, which is the
// default, means the service normalizes the schema into strict mode where it
// can and quietly falls back where it cannot. True requires every object in
// api.Tool.Schema to carry additionalProperties:false and list every
// property in required, and is a 400 otherwise; since the schema comes from the
// caller, that is theirs to promise.
func WithStrictTools(strict bool) Option {
	return func(p *Provider) error {
		p.set.strictTools = &strict
		return nil
	}
}

// WithStore turns server side retention on. Off by default. Turning it on stops
// encrypted_content being populated by default, so reasoning replay then
// depends on the include parameter this adapter already sends.
func WithStore(store bool) Option {
	return func(p *Provider) error {
		p.set.store = store
		return nil
	}
}

// WithInclude replaces the include list. Unset, the adapter asks for
// reasoning.encrypted_content on reasoning models, which is redundant on
// current models where stateless mode populates it anyway and still required on
// older ones.
func WithInclude(values ...string) Option {
	return func(p *Provider) error {
		p.set.include = values
		return nil
	}
}

// WithPromptCacheKey steers cache routing for requests sharing a long prefix,
// and is required on gpt-5.6 and later to get reliable matching at all. Keep
// traffic under roughly fifteen requests a minute per key.
func WithPromptCacheKey(key string) Option {
	return func(p *Provider) error {
		p.set.promptCacheKey = key
		return nil
	}
}

// CacheMode names a prompt_cache_options.mode value and CacheTTL its ttl. Open
// like Effort: custom strings pass through.
type CacheMode string

const (
	CacheImplicit CacheMode = "implicit"
	CacheExplicit CacheMode = "explicit"
)

type CacheTTL string

const CacheTTL30M CacheTTL = "30m"

// WithPromptCacheOptions sets prompt_cache_options, mode being implicit or
// explicit and ttl currently only 30m. gpt-5.6 and later only: earlier models
// answer 400 to the field, so it is set on a Provider configured with one of
// those and left alone on any other.
//
// Explicit mode wants per block breakpoints, which this adapter cannot place:
// api.Content has no way to mark a block as the end of a cached prefix,
// and guessing one would move it every time the thread grew.
func WithPromptCacheOptions(mode CacheMode, ttl CacheTTL) Option {
	return func(p *Provider) error {
		p.set.cacheMode = string(mode)
		p.set.cacheTTL = string(ttl)
		return nil
	}
}

// ServiceTier names a service_tier value. Open like Effort: custom strings
// pass through.
type ServiceTier string

const (
	TierAuto     ServiceTier = "auto"
	TierDefault  ServiceTier = "default"
	TierFlex     ServiceTier = "flex"
	TierScale    ServiceTier = "scale"
	TierPriority ServiceTier = "priority"
)

// WithServiceTier sets service_tier: auto, default, flex, scale or priority.
// flex is slower and answers 429 when there is no spare capacity, which arrives
// as CodeBusy, the same as a rate limit: the answer to either is to back off and
// send the turn again unchanged, so nothing is lost by not telling them apart.
func WithServiceTier(tier ServiceTier) Option {
	return func(p *Provider) error {
		p.set.serviceTier = string(tier)
		return nil
	}
}

// WithMetadata attaches up to sixteen string pairs to the turn.
func WithMetadata(m map[string]string) Option {
	return func(p *Provider) error {
		p.set.metadata = m
		return nil
	}
}

// WithObfuscation keeps the random padding field the service adds to delta
// events to even out packet sizes. Dropped by default because the decoder
// ignores it and it is bandwidth.
func WithObfuscation(on bool) Option {
	return func(p *Provider) error {
		p.set.obfuscation = on
		return nil
	}
}
