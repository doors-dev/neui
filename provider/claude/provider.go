// Package claude implements api.Provider against Anthropic's Messages
// API, streaming POST /v1/messages with the anthropic-version header.
//
// Nothing here uses an SDK. The whole point of a hand written adapter is exact
// control of the wire format: a thinking block has to be replayed byte for
// byte, a sealed payload must never be re-encoded, and the block index the
// protocol reports is counted here rather than taken from the provider.
//
// Everything api.Request deliberately omits lives on the provider value
// instead: the model comes from Conf, and the response cap, sampling settings,
// thinking configuration, reasoning effort and cache breakpoints from options,
// because they either vary by model or are refused outright by the models that
// matter most. A caller that wants two settings for one model builds two
// providers.
package claude

import (
	"bytes"
	"context"
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
	providerName   = "claude"
	defaultBaseURL = "https://api.anthropic.com"
	defaultVersion = "2023-06-01"
	// max_tokens is required by this API, so every request carries one whether
	// the caller configured it or not. This is well under the smallest output
	// ceiling any current model has, because a value over a model's ceiling is a
	// 400 while a value under it only truncates, and truncation comes back as
	// CodeCapped naming its own fix where a 400 is just a failed turn. It also
	// caps thinking, which is on by default on the newest models, so callers who
	// want long answers raise it.
	defaultMaxTokens = 16384
)

var errTooLarge = errors.New("claude: request too large")

// ThinkingType is the thinking mode sent on the wire. The consts are the
// documented values; the type does not validate, so a value a later API
// release adds passes through unchanged.
type ThinkingType string

const (
	ThinkingAdaptive ThinkingType = "adaptive"
	ThinkingEnabled  ThinkingType = "enabled"
	ThinkingDisabled ThinkingType = "disabled"
)

// ThinkingDisplay is how thinking streams back. As with ThinkingType, the
// const is the documented value and custom strings pass through.
type ThinkingDisplay string

const DisplaySummarized ThinkingDisplay = "summarized"

// Thinking is the thinking configuration sent verbatim. It is a passthrough
// rather than a normalization because which values a model accepts changes with
// every release: adaptive is rejected by models older than 4.6, enabled is
// rejected by 4.7 and later, disabled is rejected by the models that always
// think, and any table of that here would be wrong within a quarter. The zero
// value sends no thinking field at all, which is not the same as disabling it:
// on the newest models thinking is on by default.
type Thinking struct {
	Type    ThinkingType    // adaptive, enabled or disabled; empty sends nothing
	Budget  int             // budget_tokens, extended thinking only
	Display ThinkingDisplay // summarized or omitted; empty leaves the model default
}

func (t Thinking) active() bool {
	return t.Type != "" && t.Type != ThinkingDisabled
}

// summarized reports whether the thinking this configuration asks for is a
// summary. The wire streams a summary and the thinking itself through the same
// delta and says which nowhere, so the request is what decides how the adapter
// reports it.
func (t Thinking) summarized() bool {
	return t.Display == DisplaySummarized
}

// Breakpoint names a place a cache_control marker goes. The three mirror the
// tools then system then messages hierarchy the API invalidates in, which is
// the same order api.Request lists its fields in.
type Breakpoint string

const (
	CacheTools    Breakpoint = "tools"    // after the last tool definition
	CacheSystem   Breakpoint = "system"   // after the last system block
	CacheMessages Breakpoint = "messages" // after the last block of the last message
)

// Provider is one configured Anthropic endpoint. It is safe for concurrent use
// and holds no per turn state.
type Provider struct {
	key         string
	model       api.Model
	baseURL     string
	version     string
	betas       []string
	client      *http.Client
	maxToken    int
	temperature *float64
	topP        *float64
	topK        *int
	stopSeqs    []string
	thinking    Thinking
	effort      string
	breakpoints []Breakpoint
	cacheTTL    string
	transport   lib.Options
}

type Option func(*Provider) error

// Conf is what a provider cannot be built without. The model decides which
// thinking and sampling settings are accepted and there is nothing sane to guess,
// so it is an argument rather than an option and the compiler is what keeps a
// half configured provider from existing. It is a struct so that a second
// mandatory field later does not rewrite every call site.
type Conf struct {
	Model api.Model // the one model this provider streams with
}

// New builds a provider for one model. The key defaults to ANTHROPIC_API_KEY.
// A nil option is skipped, and an option that fails is logged and dropped, so
// the provider keeps the prior or default value the option would have replaced.
func New(conf Conf, opts ...Option) *Provider {
	p := &Provider{
		key:      os.Getenv("ANTHROPIC_API_KEY"),
		model:    conf.Model,
		baseURL:  defaultBaseURL,
		version:  defaultVersion,
		client:   http.DefaultClient,
		maxToken: defaultMaxTokens,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			slog.Error("claude: option rejected", "err", err)
		}
	}
	if p.key == "" {
		slog.Warn("claude: no API key: set ANTHROPIC_API_KEY or pass WithAPIKey")
	}
	return p
}

// WithAPIKey overrides the key read from the environment.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		return nil
	}
}

// WithBaseURL points the adapter at another host: a gateway, a proxy, or a test
// server. The path /v1/messages is appended to it. A value that is empty or
// does not parse as a URL with a scheme and host is rejected, so a bad base URL
// is logged at New and the default stays; a trailing "/" is trimmed.
func WithBaseURL(base string) Option {
	return func(p *Provider) error {
		if base == "" {
			return fmt.Errorf("empty base URL")
		}
		u, err := url.Parse(base)
		if err != nil {
			return fmt.Errorf("base URL %q: %w", base, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("base URL %q needs a scheme and host", base)
		}
		p.baseURL = strings.TrimRight(base, "/")
		return nil
	}
}

// WithHTTPClient supplies the client, which is where connection reuse, proxies
// and any overall request deadline come from. Do not set Timeout on a client
// used for streaming: it bounds the whole response, so it cuts long turns
// mid sentence. Use WithTransport instead.
func WithHTTPClient(c *http.Client) Option {
	return func(p *Provider) error {
		p.client = c
		return nil
	}
}

// WithVersion overrides the anthropic-version header.
func WithVersion(v string) Option {
	return func(p *Provider) error {
		p.version = v
		return nil
	}
}

// WithBeta adds anthropic-beta flags, joined with commas.
func WithBeta(flags ...string) Option {
	return func(p *Provider) error {
		p.betas = append(p.betas, flags...)
		return nil
	}
}

// WithMaxTokens sets max_tokens, the response cap this API requires on every
// request. It caps thinking too, which is on by default on the newest models,
// so a caller who wants long answers raises it.
//
// A value below this API's schema minimum of 1 is refused here, because an
// out-of-range cap is a misconfiguration a host can be told about at
// construction, where the same number arriving mid turn could only be silently
// adjusted. The ceiling is the other half and cannot be checked: it differs per
// model and the API publishes it nowhere api.Model carries, so a value over the
// configured model's ceiling goes to the API and comes back as CodeInvalid.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 1 {
			return fmt.Errorf("max tokens %d is below the 1 this API takes", n)
		}
		p.maxToken = n
		return nil
	}
}

// WithTemperature, WithTopP and WithTopK set sampling. Every current frontier
// model refuses a non default value for these on every request, thinking or
// not, and older models refuse them while thinking is on, so setting one and
// then streaming to such a model is refused here rather than sent and answered
// with a 400.
func WithTemperature(t float64) Option {
	return func(p *Provider) error {
		p.temperature = &t
		return nil
	}
}

func WithTopP(v float64) Option {
	return func(p *Provider) error {
		p.topP = &v
		return nil
	}
}

func WithTopK(v int) Option {
	return func(p *Provider) error {
		p.topK = &v
		return nil
	}
}

// WithStopSequences sets stop_sequences. A turn that ends on one reports
// stop_sequence, which is an ordinary completion: the turn is kept, and the
// caller learns the sequence was hit from the text rather than from the code.
func WithStopSequences(seqs ...string) Option {
	return func(p *Provider) error {
		p.stopSeqs = append(p.stopSeqs, seqs...)
		return nil
	}
}

// WithThinking sets the thinking configuration. It is rendered into the prompt,
// so changing it starts a new cache prefix, and changing it partway through a
// tool use turn is not an error: the API quietly turns thinking off for that
// request instead. Nothing in the response says so except the absence of
// thinking blocks, so configure one provider per setting and keep it.
func WithThinking(t Thinking) Option {
	return func(p *Provider) error {
		p.thinking = t
		return nil
	}
}

// Effort is an output_config.effort level. The consts are the documented
// values; the type does not validate, so a level a later API release adds
// passes through unchanged.
type Effort string

const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortXHigh  Effort = "xhigh"
	EffortMax    Effort = "max"
)

// WithEffort sets output_config.effort: low, medium, high, xhigh or max. Which
// levels a model accepts differs by model, and it is rendered into the prompt
// alongside the thinking configuration, so changing it also starts a new cache
// prefix.
func WithEffort(effort Effort) Option {
	return func(p *Provider) error {
		p.effort = string(effort)
		return nil
	}
}

// WithCacheBreakpoints places ephemeral cache_control markers. Caching is opt
// in and the API allows at most four markers; the three breakpoints here are
// the whole hierarchy, so any combination is within the limit.
//
// A marker is never attached to a replayed reasoning or opaque block: those go
// back byte for byte or the request is refused as modified.
func WithCacheBreakpoints(points ...Breakpoint) Option {
	return func(p *Provider) error {
		p.breakpoints = append(p.breakpoints, points...)
		return nil
	}
}

// CacheTTL is a cache_control ttl. The consts are the documented values; the
// type does not validate, so a value a later API release adds passes through
// unchanged.
type CacheTTL string

const (
	CacheTTL5M CacheTTL = "5m"
	CacheTTL1H CacheTTL = "1h"
)

// WithCacheTTL sets the cache_control ttl, for example CacheTTL1H. Empty leaves
// the five minute default.
func WithCacheTTL(ttl CacheTTL) Option {
	return func(p *Provider) error {
		p.cacheTTL = string(ttl)
		return nil
	}
}

// WithTransport sets the limits lib keeps on a turn: how long the wire may stay
// silent, and how many bytes one server sent event may accumulate. The fields,
// their defaults and the reasoning behind both live on lib.Options and the value
// is handed down unread, so a field left zero keeps lib's own default rather
// than one of this adapter's.
//
// The idle deadline is what bounds a turn here, and it is always on: anything
// arriving resets it, ping frames included, so it measures a dead socket rather
// than a slow turn, which is the whole reason a deadline here works where one
// built on content alone cannot. There is no value that turns it off; a caller
// who wants a turn to run effectively unbounded passes an hour.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.transport = opt
		return nil
	}
}

// Name says which API this is, for whoever is choosing between providers and
// for whoever stores what comes back.
func (p *Provider) Name() string {
	return providerName
}

// Model answers from configuration alone, so it is callable before the first
// turn and says the same thing on every one.
func (p *Provider) Model() api.Model {
	return p.model
}

// Stream runs the turn on its own goroutine. A turn that is allowed to finish
// ends in exactly one EventOk or EventErr and the channel closes after it. A
// cancelled turn ends in neither, and neither does one whose connection simply
// stopped: the producer stops where it is and closes the channel.
//
// The request is built out of what the caller asked for, so a refusal building
// it is theirs to fix and no turn ever starts.
func (p *Provider) Stream(ctx context.Context, req api.Request) <-chan api.Event {
	httpReq, sent, err := p.request(ctx, req)
	if errors.Is(err, errTooLarge) {
		return lib.Drain(api.EventErr{Code: api.CodeTooLarge, Err: err})
	}
	if err != nil {
		return lib.Drain(api.EventErr{Code: api.CodeInvalid, Err: err})
	}
	return lib.Stream(ctx, p.client, httpReq, newConverter(p.model.Name, p.thinking.summarized(), sent, p.model.RequestBytes), p.transport)
}

// request encodes the turn. http.NewRequestWithContext cannot fail on what is
// passed to it here — the method is a constant and the URL is the configured
// base with a fixed path — so a failure is a base URL that was never valid,
// which is a mistake at construction rather than an outcome of this turn.
func (p *Provider) request(ctx context.Context, req api.Request) (*http.Request, int, error) {
	body, err := p.encode(req)
	if err != nil {
		return nil, 0, err
	}
	if p.model.RequestBytes > 0 && len(body) > p.model.RequestBytes {
		return nil, len(body), fmt.Errorf("%w: the request is %d bytes, over the %d the model %s takes", errTooLarge, len(body), p.model.RequestBytes, p.model.Name)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.baseURL+"/v1/messages", bytes.NewReader(body))
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	httpReq.Header.Set("anthropic-version", p.version)
	if p.key != "" {
		httpReq.Header.Set("x-api-key", p.key)
	}
	if len(p.betas) > 0 {
		httpReq.Header.Set("anthropic-beta", strings.Join(p.betas, ","))
	}
	return httpReq, len(body), nil
}
