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

// Package gemini is the api.Provider for Google's Gemini Interactions API.
//
// Build one with New and pass it to neui.WithProvider as the app-wide
// provider, or to neui.OptionProvider to set one session's provider, which
// takes precedence over WithProvider if any:
//
//	p, err := gemini.New(gemini.Conf{Model: gemini.Hydrogen4},
//		gemini.WithEffort(gemini.EffortMedium))
//
// The key comes from GEMINI_API_KEY unless WithAPIKey sets it. A provider
// serves one model with one set of options; build two for two settings.
//
// It speaks the Interactions API of generativelanguage.googleapis.com, or of a
// gateway that speaks it unchanged, set with WithBaseURL; Vertex AI is not
// supported. Nothing is stored on the service: every turn sends the whole
// thread with store off. Thinking summaries stream by default. Caching is
// implicit and needs no option.
//
// Failures reach the thread as api.EventErr with an api.ErrCode, and a failure
// the API reported carries an *APIError, read with errors.As.
package gemini

import (
	"context"
	"crypto/rand"
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
	providerName    = "gemini"
	defaultBaseURL  = "https://generativelanguage.googleapis.com"
	interactions    = "/v1/interactions"
	defaultRevision = "2026-05-20"
	skipSignature   = "skip_thought_signature_validator"
	opaquePrefix    = "gemini."
)

var errTooLarge = errors.New("gemini: request too large")

// Effort is a generation_config.thinking_level value for WithEffort.
type Effort string

// The documented thinking levels, least to most.
const (
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
)

// Provider is one configured Gemini endpoint. It is safe for concurrent use
// and holds no per turn state.
type Provider struct {
	key         string
	keyExplicit bool
	model       api.Model
	baseURL     string
	revision    string
	client      *http.Client
	maxTokens   int
	effort      string
	summaries   bool
	stopSeqs    []string
	transport   lib.Options
}

// Option configures a Provider at New. An option that returns an error makes
// New return it.
type Option func(*Provider) error

// Conf holds the provider's model.
type Conf struct {
	Model api.Model // the one model this provider streams with
}

// New builds a provider for one model. A nil option is skipped. New returns
// the error of the first option that fails, and an error for an empty
// Model.Name. On a model this package declares, matched by exact name, it
// also refuses a WithMaxTokens over the model's per-request output limit and
// a thinking level the model does not take. On any other model nothing is
// checked. The key defaults to GEMINI_API_KEY and is not checked; a missing
// or wrong key ends the turn with api.CodeAccess.
func New(conf Conf, opts ...Option) (*Provider, error) {
	p := &Provider{
		model:     conf.Model,
		baseURL:   defaultBaseURL,
		revision:  defaultRevision,
		client:    &http.Client{},
		summaries: true,
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
		return nil, errors.New("gemini: no model, set Conf.Model")
	}
	if m, ok := declared[p.model.Name]; ok {
		if p.maxTokens > m.maxTokens {
			return nil, fmt.Errorf("gemini: max tokens %d is over the %d model %s returns in one request", p.maxTokens, m.maxTokens, p.model.Name)
		}
		if p.effort != "" && !slices.Contains(m.levels, Effort(p.effort)) {
			return nil, fmt.Errorf("gemini: model %s does not take thinking level %q; it takes %q", p.model.Name, p.effort, m.levels)
		}
	}
	if !p.keyExplicit {
		p.key = os.Getenv("GEMINI_API_KEY")
	}
	return p, nil
}

// WithAPIKey sets the key, and GEMINI_API_KEY is then not read. An empty key
// sends no x-goog-api-key header, for a gateway that authenticates some other
// way.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		p.keyExplicit = true
		return nil
	}
}

// WithBaseURL points the provider at another host: a gateway, a proxy or a
// test server. The path /v1/interactions is appended to it. A value that is
// empty or does not parse as a URL with a scheme and host makes New fail; a
// trailing "/" is trimmed.
func WithBaseURL(base string) Option {
	return func(p *Provider) error {
		if base == "" {
			return errors.New("gemini: empty base URL")
		}
		u, err := url.Parse(base)
		if err != nil {
			return fmt.Errorf("gemini: base URL %q: %w", base, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("gemini: base URL %q needs a scheme and host", base)
		}
		p.baseURL = strings.TrimRight(base, "/")
		return nil
	}
}

// WithHTTPClient replaces the default client, for a proxy, middleware or a
// test server; it must not be nil. Leave its Timeout at zero: it caps the
// whole streamed turn. Use WithTransport to bound a dead connection.
func WithHTTPClient(c *http.Client) Option {
	return func(p *Provider) error {
		if c == nil {
			return errors.New("gemini: WithHTTPClient needs a client")
		}
		p.client = c
		return nil
	}
}

// WithTransport sets the stream limits documented on lib.Options: how long the
// wire may stay silent and how large one event may grow. A zero field keeps
// lib's default. The idle deadline covers the wait for response headers and
// resets on every complete event and on every comment or blank line between
// events; a turn that crosses it ends with api.CodeNetwork. The API sends a
// keep-alive comment every 15 seconds while a request waits in its queue or
// thinks, so the default deadline holds for turns that take minutes.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.transport = opt
		return nil
	}
}

// WithRevision overrides the Api-Revision header, 2026-05-20 by default. The
// adapter is written against that revision. An empty revision makes New fail.
func WithRevision(v string) Option {
	return func(p *Provider) error {
		if v == "" {
			return errors.New("gemini: WithRevision needs a revision")
		}
		p.revision = v
		return nil
	}
}

// WithMaxTokens sets generation_config.max_output_tokens, which caps thinking
// and answer together. Zero, the default, sends nothing and the model's
// per-request limit applies; a negative value makes New fail. New refuses a
// value over 65,536 on a declared model. A turn that hits the cap ends with
// api.CodeCapped.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 0 {
			return fmt.Errorf("gemini: max tokens %d is negative", n)
		}
		p.maxTokens = n
		return nil
	}
}

// WithEffort sets generation_config.thinking_level; empty leaves the model
// default: high on Hydrogen and Neon, medium on Carbon, Argon and 3.8 Flash,
// minimal on 3.5 Flash-Lite. No level turns thinking off. On a declared model
// New refuses a level the model does not take: minimal on the Gemini 4 models
// and 3.8 Flash, xhigh on Carbon, Argon, 3.8 Flash and 3.5 Flash-Lite, and any
// value that is not a documented level. On any other model the level is sent
// as is, and one the model refuses fails each turn with api.CodeInvalid.
func WithEffort(e Effort) Option {
	return func(p *Provider) error {
		p.effort = string(e)
		return nil
	}
}

// WithThinkingSummaries sets generation_config.thinking_summaries. On, the
// default, sends auto: summaries of the model's thinking stream as
// api.EventSummary while it thinks, and short reasoning often has none. Off
// sends none, and the stream carries only keep-alives until thinking ends.
func WithThinkingSummaries(on bool) Option {
	return func(p *Provider) error {
		p.summaries = on
		return nil
	}
}

// WithStopSequences sets generation_config.stop_sequences, replacing any set
// before. A turn that ends on one ends as an ordinary completion, api.CodeTurn,
// and nothing reports which sequence was hit.
func WithStopSequences(seqs ...string) Option {
	return func(p *Provider) error {
		p.stopSeqs = seqs
		return nil
	}
}

var _ api.Provider = (*Provider)(nil)

// Name returns "gemini".
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
// request this wire cannot carry, or one over Model.RequestBytes, ends at once
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
	base := providerName + "_" + strings.ToLower(rand.Text()[:10])
	return lib.Stream(ctx, p.client, httpReq, newConverter(p.model.Name, base, sent, p.model.RequestBytes), p.transport)
}

func (p *Provider) request(ctx context.Context, req api.Request) (*http.Request, int, error) {
	w, err := p.encode(req)
	if err != nil {
		return nil, 0, err
	}
	body, err := lib.MarshalBody(w)
	if err != nil {
		return nil, 0, fmt.Errorf("gemini: encoding the request: %w", err)
	}
	sent := len(body.Bytes())
	if p.model.RequestBytes > 0 && sent > p.model.RequestBytes {
		return nil, sent, fmt.Errorf("%w: the request is %d bytes, over the %d the model %s takes", errTooLarge, sent, p.model.RequestBytes, p.model.Name)
	}
	httpReq, err := body.Request(ctx, p.baseURL+interactions)
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Api-Revision", p.revision)
	if p.key != "" {
		httpReq.Header.Set("x-goog-api-key", p.key)
	}
	return httpReq, sent, nil
}
