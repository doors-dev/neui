// Copyright 2026 doors dev LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package claude is the api.Provider for Anthropic's Messages API.
//
// Build one with New and pass it to neui.WithProvider as the app-wide
// provider, or to neui.OptionProvider to set one session's provider, which
// takes precedence over WithProvider if any:
//
//	p, err := claude.New(claude.Conf{Model: claude.Opus55},
//		claude.WithMaxTokens(32000))
//
// Prompt caching is on by default: each request marks the end of the system
// prompt, which covers the tools before it, and the end of the last message.
// WithCacheTTL sets how long entries live; WithoutCache turns caching off.
//
// Thinking is always adaptive: WithEffort sets how much the model thinks and
// WithThinkingDisplay how its thinking streams back. Every request asks the
// API, under the thinking-binding-controls-2026-08-01 beta, to drop a replayed
// thinking block whose earlier conversation no longer matches instead of
// refusing the request. A model without adaptive thinking cannot be served.
//
// The key comes from ANTHROPIC_API_KEY unless WithAPIKey sets it. A provider
// serves one model with one set of options; build two for two settings. New
// refuses an option it cannot apply and a setting the declared model does not
// take, so a provider for a declared model that builds sends nothing the API
// refuses for its configuration alone. For a model not declared here New
// checks no combination and every option is sent as configured.
//
// Only the Anthropic API is supported: api.anthropic.com, or a gateway or proxy
// that speaks it unchanged, set with WithBaseURL. Amazon Bedrock and Google
// Cloud Vertex AI authenticate and address models differently, and no model
// here is declared for them.
//
// Failures reach the thread as api.EventErr with an api.ErrCode, and an error
// response from the API carries an *APIError, read with errors.As. The
// adapter talks to the wire directly, with no SDK dependency.
package claude

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"slices"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const (
	providerName   = "claude"
	defaultBaseURL = "https://api.anthropic.com"
	defaultVersion = "2026-10-21"
	// max_tokens is required by this API, so every request carries one whether
	// the caller configured it or not. This is well under the smallest output
	// ceiling any current model has, because a value over a model's ceiling is a
	// 400 while a value under it only truncates, and truncation comes back as
	// CodeCapped naming its own fix where a 400 is just a failed turn. It also
	// caps thinking, which is always on, so callers who want long answers raise
	// it.
	defaultMaxTokens = 16384
)

var errTooLarge = errors.New("claude: request too large")

// ThinkingDisplay is how thinking streams back. The consts are the documented
// values; the type does not validate, so a value a later API release adds
// passes through unchanged.
type ThinkingDisplay string

// The documented display values.
const (
	// DisplaySummarized streams a summary of the thinking rather than the
	// thinking itself.
	DisplaySummarized ThinkingDisplay = "summarized"
	// DisplayOmitted streams thinking blocks with no text, only their
	// signatures; the default on Claude 5 and later.
	DisplayOmitted ThinkingDisplay = "omitted"
)

// Provider is one configured Anthropic endpoint. It is safe for concurrent use
// and holds no per turn state.
type Provider struct {
	key         string
	keyExplicit bool
	model       api.Model
	baseURL     string
	version     string
	betas       []string
	client      *http.Client
	maxToken    int
	stopSeqs    []string
	display     ThinkingDisplay
	effort      string
	noCache     bool
	cacheTTL    string
	transport   lib.Options
}

// Option configures a Provider at New. An option that returns an error makes
// New return it.
type Option func(*Provider) error

// Conf holds the provider's model. Model has no default: New refuses a Conf
// whose Model.Name is empty.
type Conf struct {
	Model api.Model // the one model this provider streams with
}

// New builds a provider for one model. The key defaults to ANTHROPIC_API_KEY
// and is not checked; a missing or wrong key ends the turn with
// api.CodeAccess. A nil option is skipped. New returns the error of the first
// option that fails, and an error for an empty Model.Name. On a model this package
// declares it also returns one for a WithMaxTokens over its output ceiling. On
// any other model nothing is checked.
func New(conf Conf, opts ...Option) (*Provider, error) {
	p := &Provider{
		model:    conf.Model,
		baseURL:  defaultBaseURL,
		version:  defaultVersion,
		client:   &http.Client{},
		maxToken: defaultMaxTokens,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	if p.model.Name == "" {
		return nil, errors.New("claude: Conf.Model has no name")
	}
	if m, ok := declared[p.model.Name]; ok && p.maxToken > m.maxTokens {
		return nil, fmt.Errorf("claude: max tokens %d is over the %d model %s returns", p.maxToken, m.maxTokens, p.model.Name)
	}
	if !p.keyExplicit {
		p.key = os.Getenv("ANTHROPIC_API_KEY")
	}
	return p, nil
}

// WithAPIKey overrides the key read from the environment. An empty key sends
// no x-api-key header, for a gateway that authenticates through its own
// transport.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		p.keyExplicit = true
		return nil
	}
}

// WithBaseURL points the adapter at another host: a gateway, a proxy, or a test
// server. The path /v1/messages is appended to it. A value that is empty or
// does not parse as a URL with a scheme and host makes New fail; a trailing "/"
// is trimmed.
func WithBaseURL(base string) Option {
	return func(p *Provider) error {
		if base == "" {
			return errors.New("claude: empty base URL")
		}
		u, err := url.Parse(base)
		if err != nil {
			return fmt.Errorf("claude: base URL %q: %w", base, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("claude: base URL %q needs a scheme and host", base)
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
		if c == nil {
			return errors.New("claude: WithHTTPClient needs a client")
		}
		p.client = c
		return nil
	}
}

// WithVersion overrides the anthropic-version header, 2026-10-21 by default.
// The adapter is written against that version. An empty version is an error.
func WithVersion(v string) Option {
	return func(p *Provider) error {
		if v == "" {
			return errors.New("claude: WithVersion needs a version")
		}
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

// WithMaxTokens sets max_tokens, the response cap sent on every request; 0
// keeps the default of 16384 and a negative value makes New fail. It caps
// thinking too, so raise it for long answers. A turn that hits it ends with api.CodeCapped. New refuses a
// value over the output ceiling of a model this package declares; on any other
// model one over its ceiling fails each turn with api.CodeInvalid.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 0 {
			return fmt.Errorf("claude: max tokens %d is negative", n)
		}
		if n == 0 {
			n = defaultMaxTokens
		}
		p.maxToken = n
		return nil
	}
}

// WithStopSequences sets stop_sequences, replacing any set before. A turn
// that ends on one ends as an ordinary completion, api.CodeTurn, and nothing
// reports which sequence was hit.
func WithStopSequences(seqs ...string) Option {
	return func(p *Provider) error {
		p.stopSeqs = seqs
		return nil
	}
}

const bindingBeta = "thinking-binding-controls-2026-08-01"

// WithThinkingDisplay sets how thinking streams back: DisplaySummarized as a
// readable summary, DisplayOmitted as blocks with no text. Empty leaves the
// model default.
func WithThinkingDisplay(d ThinkingDisplay) Option {
	return func(p *Provider) error {
		p.display = d
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

// WithEffort sets output_config.effort, which sets how much the model thinks;
// empty leaves the model default. On a model not declared here a level the
// model does not accept fails each turn with api.CodeInvalid. It is part of the
// cached prompt.
func WithEffort(effort Effort) Option {
	return func(p *Provider) error {
		p.effort = string(effort)
		return nil
	}
}

// WithoutCache sends no cache_control markers, for a gateway that refuses
// them.
func WithoutCache() Option {
	return func(p *Provider) error {
		p.noCache = true
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
// CacheTTL1H. Empty leaves the five minute default. A 1h entry costs twice the
// input price to write instead of 1.25 times.
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

var _ api.Provider = (*Provider)(nil)

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
// request whose content cannot be encoded, or is over Model.RequestBytes, ends
// at once in EventErr with api.CodeInvalid or api.CodeTooLarge. The thread calls it;
// hosts normally do not.
func (p *Provider) Stream(ctx context.Context, req api.Request) <-chan api.Event {
	httpReq, sent, err := p.request(ctx, req)
	if errors.Is(err, errTooLarge) {
		return lib.Drain(api.EventErr{Code: api.CodeTooLarge, Err: err})
	}
	if err != nil {
		return lib.Drain(api.EventErr{Code: api.CodeInvalid, Err: err})
	}
	return lib.Stream(ctx, p.client, httpReq, newConverter(p.model.Name, p.display == DisplaySummarized, sent, p.model.RequestBytes), p.transport)
}

// request encodes the turn. http.NewRequestWithContext cannot fail on what is
// passed to it here — the method is a constant and the URL is the configured
// base with a fixed path — so a failure is a base URL that was never valid,
// which is a mistake at construction rather than an outcome of this turn.
func (p *Provider) request(ctx context.Context, req api.Request) (*http.Request, int, error) {
	w, err := p.encode(req)
	if err != nil {
		return nil, 0, err
	}
	body, err := lib.MarshalBody(w)
	if err != nil {
		return nil, 0, fmt.Errorf("claude: encoding request: %w", err)
	}
	sent := len(body.Bytes())
	if p.model.RequestBytes > 0 && sent > p.model.RequestBytes {
		return nil, sent, fmt.Errorf("%w: the request is %d bytes, over the %d the model %s takes", errTooLarge, sent, p.model.RequestBytes, p.model.Name)
	}
	httpReq, err := body.Request(ctx, p.baseURL+"/v1/messages")
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("content-type", "application/json")
	httpReq.Header.Set("accept", "text/event-stream")
	httpReq.Header.Set("anthropic-version", p.version)
	if p.key != "" {
		httpReq.Header.Set("x-api-key", p.key)
	}
	betas := p.betas
	if !slices.Contains(betas, bindingBeta) {
		betas = append(slices.Clone(betas), bindingBeta)
	}
	httpReq.Header.Set("anthropic-beta", strings.Join(betas, ","))
	return httpReq, sent, nil
}
