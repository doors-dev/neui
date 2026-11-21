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

// Package compat is the api.Provider for servers that speak OpenAI's Chat
// Completions API: self-hosted ones such as llama.cpp, vLLM, SGLang, Ollama and
// LM Studio, and gateways such as OpenRouter.
//
// Build one with New and pass it to neui.WithProvider as the app-wide
// provider, or to neui.OptionProvider to set one session's provider, which
// takes precedence over WithProvider if any:
//
//	p, err := compat.New(compat.Conf{
//		Model:   api.Model{Name: "qwen3.8-27b", Context: 131_072},
//		BaseURL: "http://127.0.0.1:8080/v1",
//	}, compat.WithEffort(compat.EffortLow))
//
// No key is read from the environment: WithAPIKey sets one, and without it no
// Authorization header is sent. A provider serves one model with one set of
// options; build two for two settings. No model is declared here: New checks
// no combination, every option is sent as configured, and a turn the server
// refuses ends with api.CodeInvalid. WithExtraBody sends the settings a server
// or gateway defines beyond the standard fields.
//
// Reasoning streams back from reasoning or reasoning_content, whichever the
// server writes, and goes back on later turns in the same field; reasoning
// another provider wrote is left out, and reasoning a server leaves in the
// text as <think> tags stays text. Images go as data URLs in user messages and
// tool results, and a server whose model reads no images refuses such a turn.
// A document of a text type goes as its text when its bytes are UTF-8, and any
// other document gets a short note. Caching is the server's: keep the tools,
// system prompt and history stable between turns, and cache reads arrive in
// the usage where the server reports prompt_tokens_details.cached_tokens.
// Failures reach the thread as api.EventErr with an api.ErrCode, and a failure
// the server reported carries an *APIError, read with errors.As.
package compat

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

const (
	providerName   = "compat"
	completionPath = "/chat/completions"
)

// Effort is the reasoning_effort sent with every request. What a level does is
// the server's choice; llama.cpp and OpenRouter turn thinking off at
// EffortNone, and a server or chat template may refuse a level it lacks. Any
// string passes through unchanged.
type Effort string

// The reasoning_effort levels OpenAI documents.
const (
	EffortNone    Effort = "none"
	EffortMinimal Effort = "minimal"
	EffortLow     Effort = "low"
	EffortMedium  Effort = "medium"
	EffortHigh    Effort = "high"
	EffortXHigh   Effort = "xhigh"
	EffortMax     Effort = "max"
)

// Provider is one server and model plus the settings api.Request leaves out
// (effort, sampling, stop sequences, response cap, extra fields). It is safe
// for concurrent use and holds no per turn state.
type Provider struct {
	key         string
	base        string
	model       api.Model
	client      *http.Client
	effort      Effort
	temperature *float64
	topP        *float64
	stop        []string
	maxTokens   int
	extra       []byte
	transport   lib.Options
}

// Option configures a Provider at New. An option that returns an error makes
// New return it.
type Option func(*Provider) error

// Conf holds what a Provider has no default for.
type Conf struct {
	// Model is the model every turn is sent to, named as the server lists it.
	// Set Context and the other limits the server has; none is read from the
	// server. New refuses an empty Model.Name.
	Model api.Model `json:"model"`
	// BaseURL is the server's OpenAI-compatible root, such as
	// http://127.0.0.1:8080/v1 or https://openrouter.ai/api/v1, without the
	// /chat/completions path; a trailing slash is trimmed. New refuses an
	// empty value or one without a scheme and host.
	BaseURL string `json:"baseURL"`
}

// New builds a Provider for conf.Model at conf.BaseURL. A nil option is
// skipped. New returns the first option error, and an error for an empty
// Model.Name or a BaseURL that is empty, does not parse or has no scheme and
// host. Nothing else is checked: the server decides what it accepts, and a
// missing or wrong key ends the turn with api.CodeAccess. The default HTTP
// client has no timeout; WithTransport bounds a dead connection.
func New(conf Conf, opts ...Option) (*Provider, error) {
	p := &Provider{
		model:  conf.Model,
		client: &http.Client{},
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
		return nil, errors.New("compat: no model, set Conf.Model")
	}
	if conf.BaseURL == "" {
		return nil, errors.New("compat: no base URL, set Conf.BaseURL")
	}
	u, err := url.Parse(conf.BaseURL)
	if err != nil {
		return nil, fmt.Errorf("compat: base URL %q: %w", conf.BaseURL, err)
	}
	if u.Scheme == "" || u.Host == "" {
		return nil, fmt.Errorf("compat: base URL %q: no scheme or host", conf.BaseURL)
	}
	p.base = strings.TrimSuffix(conf.BaseURL, "/")
	return p, nil
}

// WithAPIKey sets the key, sent as a bearer token. Without it, or with an
// empty key, no Authorization header is sent; a server that wants one
// refuses the turn with api.CodeAccess.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		return nil
	}
}

// WithHTTPClient replaces the default client, for a proxy, middleware or a
// test server; it must not be nil. Leave its Timeout at zero: it caps the
// whole streamed turn. Use WithTransport to bound a dead connection.
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) error {
		if client == nil {
			return errors.New("compat: WithHTTPClient needs a client")
		}
		p.client = client
		return nil
	}
}

// WithTransport sets the stream limits documented on lib.Options: how long the
// wire may stay silent and how large one event may grow. A zero field keeps
// lib's default. The idle deadline covers the wait for response headers and
// resets on every event and on every comment or blank line between events,
// such as OpenRouter's keep-alive comments; a turn that crosses it ends with
// api.CodeNetwork. A server that loads the model on the first request, such
// as Ollama or LM Studio, sends nothing while it loads: set Idle above the
// load time.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.transport = opt
		return nil
	}
}

// WithEffort sets reasoning_effort. Unset omits reasoning_effort and the
// server's default applies, which is thinking on for some models and off for
// others.
func WithEffort(e Effort) Option {
	return func(p *Provider) error {
		p.effort = e
		return nil
	}
}

// WithMaxTokens sets max_tokens, which on most servers counts reasoning
// tokens too; a turn that hits it ends with api.CodeCapped. At 0, the
// default, no cap is sent and the server's default applies. A negative value
// makes New fail.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 0 {
			return fmt.Errorf("compat: max tokens %d is negative", n)
		}
		p.maxTokens = n
		return nil
	}
}

// WithTemperature sets temperature, from 0 to 2. Unset leaves it to the
// server, which may apply the model's recommended value.
func WithTemperature(t float64) Option {
	return func(p *Provider) error {
		if !(t >= 0 && t <= 2) {
			return fmt.Errorf("compat: temperature %v is outside the range of 0 to 2", t)
		}
		p.temperature = &t
		return nil
	}
}

// WithTopP sets top_p, above 0 and at most 1. Unset leaves it to the server.
func WithTopP(t float64) Option {
	return func(p *Provider) error {
		if !(t > 0 && t <= 1) {
			return fmt.Errorf("compat: top_p %v is outside the range of above 0 to 1", t)
		}
		p.topP = &t
		return nil
	}
}

// WithStopSequences sets stop, replacing any set before. A turn that ends on
// one ends as an ordinary completion, api.CodeTurn. Some servers match the
// sequences inside reasoning too, which ends the turn before any text.
func WithStopSequences(seqs ...string) Option {
	return func(p *Provider) error {
		p.stop = seqs
		return nil
	}
}

// WithExtraBody adds fields to the top level of every request body, for the
// settings a server or gateway defines beyond the standard ones, such as
// OpenRouter's provider routing or reasoning object, vLLM's
// chat_template_kwargs or llama.cpp's cache_prompt:
//
//	compat.WithExtraBody(map[string]any{
//		"provider": map[string]any{"order": []string{"wafer"}, "allow_fallbacks": false},
//	})
//
// Each value is encoded to JSON when the option runs, and changing the map
// later has no effect. The fields follow the provider's own, sorted by name.
// A later call replaces the fields of an earlier one. New fails on an empty
// name, a value that does not encode, and a field the provider sets itself:
// model, messages, tools, tool_choice, stream, stream_options, max_tokens,
// reasoning_effort, temperature, top_p and stop. The server decides what a
// field does, and one it refuses ends the turn with api.CodeInvalid.
func WithExtraBody(fields map[string]any) Option {
	return func(p *Provider) error {
		var extra []byte
		for _, name := range slices.Sorted(maps.Keys(fields)) {
			var hint string
			switch name {
			case "":
				return errors.New("compat: WithExtraBody has a field with no name")
			case "model":
				hint = "set Conf.Model"
			case "max_tokens":
				hint = "use WithMaxTokens"
			case "reasoning_effort":
				hint = "use WithEffort"
			case "temperature":
				hint = "use WithTemperature"
			case "top_p":
				hint = "use WithTopP"
			case "stop":
				hint = "use WithStopSequences"
			case "messages", "tools", "tool_choice", "stream", "stream_options":
				hint = "which the provider builds for every request"
			}
			if hint != "" {
				return fmt.Errorf("compat: WithExtraBody cannot set %q, %s", name, hint)
			}
			value, err := lib.MarshalRaw(fields[name])
			if err != nil {
				return fmt.Errorf("compat: extra field %q: %w", name, err)
			}
			key, _ := lib.MarshalRaw(name)
			if len(extra) > 0 {
				extra = append(extra, ',')
			}
			extra = append(append(append(extra, key...), ':'), value...)
		}
		p.extra = extra
		return nil
	}
}

var _ api.Provider = (*Provider)(nil)

// Name returns "compat".
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
	if err != nil {
		code := api.CodeInvalid
		if errors.Is(err, errTooLarge) {
			code = api.CodeTooLarge
		}
		return lib.Drain(api.EventErr{Code: code, Err: err})
	}
	return lib.Stream(ctx, p.client, httpReq, newConverter(p.model.Name, sent, p.model.RequestBytes), p.transport)
}

func (p *Provider) request(ctx context.Context, req api.Request) (*http.Request, int, error) {
	body, err := p.encode(req)
	if err != nil {
		return nil, 0, err
	}
	raw, err := lib.MarshalBody(body)
	if err != nil {
		return nil, 0, fmt.Errorf("compat: encoding request: %w", err)
	}
	raw.AppendFields(p.extra)
	sent := len(raw.Bytes())
	if p.model.RequestBytes > 0 && sent > p.model.RequestBytes {
		return nil, sent, fmt.Errorf("%w: the request is %d bytes, over the %d the model %s takes", errTooLarge, sent, p.model.RequestBytes, p.model.Name)
	}
	httpReq, err := raw.Request(ctx, p.base+completionPath)
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	if p.key != "" {
		httpReq.Header.Set("Authorization", "Bearer "+p.key)
	}
	return httpReq, sent, nil
}
