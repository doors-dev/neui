// Package claude is the api.Provider for Anthropic's Messages API.
//
// Build one with New and pass it to neui.WithProvider as the app-wide
// provider, or to neui.OptionProvider to set one session's provider, which
// takes precedence over WithProvider if any:
//
//	p := claude.New(claude.Conf{Model: claude.Opus55},
//		claude.WithMaxTokens(32000),
//		claude.WithCacheBreakpoints(claude.CacheTools, claude.CacheMessages))
//
// The key comes from ANTHROPIC_API_KEY unless WithAPIKey sets it. A provider
// serves one model with one set of options; build two for two settings.
// Failures reach the thread as api.EventErr with an api.ErrCode, and an error
// response from the API carries an *APIError, read with errors.As. The
// adapter talks to the wire directly, without an SDK, so thinking blocks
// replay byte for byte.
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
	ThinkingAdaptive ThinkingType = "adaptive" // the model decides how much to think; 4.6 and later
	ThinkingEnabled  ThinkingType = "enabled"  // extended thinking with Thinking.Budget; before 4.7
	ThinkingDisabled ThinkingType = "disabled" // no thinking, where the model allows it
)

// ThinkingDisplay is how thinking streams back. As with ThinkingType, the
// const is the documented value and custom strings pass through.
type ThinkingDisplay string

// DisplaySummarized streams a summary of the thinking rather than the thinking
// itself.
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

// Option configures a Provider at New. An option that returns an error is
// logged and skipped.
type Option func(*Provider) error

// Conf holds the provider's model. Model has no default: a provider whose
// Model.Name is empty fails every turn with api.CodeInvalid
// before anything is sent.
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
// and any overall request deadline come from; it must not be nil. Do not set
// Timeout on a client used for streaming: it bounds the whole response, so it
// cuts long turns mid sentence. Use WithTransport instead.
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

// WithMaxTokens sets max_tokens, the response cap sent on every request; the
// default is 16384. It caps thinking too, which is on by default on the newest
// models, so raise it for long answers. A turn that hits it ends with
// api.CodeCapped. A value below 1 is logged at New and the previous setting
// stays; a value over the model's ceiling is not checked here and fails each
// turn with api.CodeInvalid.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 1 {
			return fmt.Errorf("max tokens %d is below the 1 this API takes", n)
		}
		p.maxToken = n
		return nil
	}
}

// WithTemperature sets temperature. A turn that carries temperature, top_p or
// top_k fails with api.CodeInvalid before anything is sent when the model id
// matches one known to refuse sampling (Fable 5, Mythos 5 and preview, Opus 5,
// Opus 4.8, Opus 4.7, Sonnet 5) or WithThinking enables thinking; for any
// other model the request is sent and the API decides. The same holds for
// WithTopP and WithTopK.
func WithTemperature(t float64) Option {
	return func(p *Provider) error {
		p.temperature = &t
		return nil
	}
}

// WithTopP sets top_p, under the same model restrictions as WithTemperature.
func WithTopP(v float64) Option {
	return func(p *Provider) error {
		p.topP = &v
		return nil
	}
}

// WithTopK sets top_k, under the same model restrictions as WithTemperature.
func WithTopK(v int) Option {
	return func(p *Provider) error {
		p.topK = &v
		return nil
	}
}

// WithStopSequences adds to stop_sequences. A turn that ends on one ends as an
// ordinary completion, api.CodeTurn, and nothing reports which sequence was
// hit.
func WithStopSequences(seqs ...string) Option {
	return func(p *Provider) error {
		p.stopSeqs = append(p.stopSeqs, seqs...)
		return nil
	}
}

// WithThinking sets the thinking configuration; the zero value sends none and
// leaves the model default. It is part of the cached prompt, so switching
// providers with different settings mid thread starts a new cache prefix, and
// mid tool use the API silently drops thinking for that request. Keep one
// setting per thread.
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

// The documented effort levels, least to most. Which ones a model accepts
// differs by model.
const (
	EffortLow    Effort = "low"
	EffortMedium Effort = "medium"
	EffortHigh   Effort = "high"
	EffortXHigh  Effort = "xhigh"
	EffortMax    Effort = "max"
)

// WithEffort sets output_config.effort; empty leaves the model default. A
// level the model does not accept fails each turn with api.CodeInvalid. Like
// the thinking configuration it is part of the cached prompt.
func WithEffort(effort Effort) Option {
	return func(p *Provider) error {
		p.effort = string(effort)
		return nil
	}
}

// WithCacheBreakpoints turns on prompt caching at the given places. Caching is
// off without it. CacheMessages caches everything up to the latest message,
// which is what a growing thread wants; any combination stays within the
// API's limit of four markers.
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
	CacheTTL5M CacheTTL = "5m" // the default
	CacheTTL1H CacheTTL = "1h" // for threads idle longer than five minutes between turns
)

// WithCacheTTL sets the ttl of the cache_control markers, for example
// CacheTTL1H. Empty leaves the five minute default. It has no effect without
// WithCacheBreakpoints.
func WithCacheTTL(ttl CacheTTL) Option {
	return func(p *Provider) error {
		p.cacheTTL = string(ttl)
		return nil
	}
}

// WithTransport sets the stream limits documented on lib.Options: how long the
// wire may stay silent and how large one event may grow. A zero field keeps
// lib's default. The idle deadline is always on, covers the wait for response
// headers, and resets on every complete event, ping events included, and on
// every comment or blank line between events, so it catches a dead connection
// rather than a slow turn; a turn that crosses it ends with api.CodeNetwork.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.transport = opt
		return nil
	}
}

// Name returns "claude".
func (p *Provider) Name() string {
	return providerName
}

// Model returns the model from Conf. It is fixed for the provider's lifetime.
func (p *Provider) Model() api.Model {
	return p.model
}

// Stream runs one turn on its own goroutine and returns its events. A turn that
// finishes ends in exactly one EventOk or EventErr, then the channel closes; a
// cancelled ctx or a connection that just stops closes it with neither. A
// request that cannot be encoded, or is over Model.RequestBytes, ends at once
// in EventErr with api.CodeInvalid or api.CodeTooLarge. The thread calls it;
// hosts normally do not.
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
