// Package deepseek implements api.Provider against DeepSeek's OpenAI
// compatible Chat Completions API.
//
// The endpoint is POST https://api.deepseek.com/chat/completions, with no
// version segment in the path and no version header. Everything is sent as a
// stream: stream_options.include_usage is always on, because it is the only way
// usage is reported at all and a turn whose cost is unknown is worse than an
// extra frame.
//
// Two things about DeepSeek shape most of this package. Thinking is on by
// default and its reasoning comes back as plain readable text in a
// reasoning_content field, sealed behind nothing, so it is a
// api.ContentReasoning carrying only Thoughts, with nothing in Meta on any
// block, and this adapter produces no api.ContentOpaque at all. And
// schemas are unconstrained here, because
// strict mode lives on a separate host, so tool arguments that never parse are
// an ordinary outcome rather than an edge case.
package deepseek

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

// Name is what this adapter answers to, for whoever is choosing between APIs and
// for whoever stores what comes back. Nothing in a content block records it.
const Name = "deepseek"

const (
	defaultBaseURL = "https://api.deepseek.com"
	completionPath = "/chat/completions"
	// maxOutputTokens is the documented ceiling for both v4 models, against a
	// 1M context. A request above it is rejected, so WithMaxTokens refuses the
	// value rather than letting the turn fail at the server.
	maxOutputTokens = 384_000
	// maxTools and maxToolNameLen are documented request limits. Checking them
	// here turns an opaque 400 into a sentence naming the tool.
	maxTools       = 128
	maxToolNameLen = 64
)

// Effort is DeepSeek's reasoning_effort. Only these two values exist: the
// server folds low and medium into high and xhigh into max, so offering the
// OpenAI vocabulary here would only let a caller believe in a distinction the
// model does not make.
type Effort string

const (
	EffortHigh Effort = "high"
	EffortMax  Effort = "max"
)

// Provider is one DeepSeek configuration: the model to talk to and every knob
// api.Request leaves out, which is all of them: sampling, thinking,
// reasoning effort, stop sequences and the cache partition key.
//
// Nothing but the model name itself is keyed by model. Flash and pro take the
// identical request shape and differ only in price and concurrency, so what
// changes the body is the thinking setting. A caller wanting two settings for
// one model builds two Providers, which is what api.Request documents.
//
// Caching needs no configuration at all. DeepSeek's context cache is automatic
// and has no markers, breakpoints or opt in, so a prefix either matches a
// persisted unit byte for byte or it does not, and the only lever is keeping
// the tools, system and message prefix stable between turns.
type Provider struct {
	key            string
	base           string
	model          api.Model
	client         *http.Client
	thinking       bool
	effort         Effort
	effortTopLevel bool
	temperature    *float64
	topP           *float64
	stop           []string
	userID         string
	maxTokens      int
	transport      lib.Options
}

type Option func(*Provider) error

// Conf is what a Provider cannot be built without. It is a struct rather than a
// bare argument so a second mandatory field later adds a field here instead of
// breaking every call site.
type Conf struct {
	// Model is the model every turn of this Provider is sent to and stamped
	// with. There is no default: the two v4 models differ in price and
	// concurrency, so picking one would spend the caller's money on a guess.
	Model api.Model `json:"model"`
}

// New builds a Provider. The key defaults to DEEPSEEK_API_KEY; a missing key is
// not an error here, it draws a slog.Warn now and fails the first turn with
// api.CodeAccess, because a constructor that cannot fail is easier to
// place than one that can. The model is not an option for the same reason
// inverted: no request can be formed without it, so the compiler asks for it
// rather than the first turn. Options follow the constructor's rule: a nil
// option is skipped, and one that reports an error is logged through
// slog.Error and leaves its knob at the prior value.
//
// The default HTTP client has no timeout on purpose. DeepSeek holds a queued
// request for up to ten minutes before inference starts, so a client wide
// deadline would cut live turns; see WithTransport for the knob that is
// actually shaped for this.
func New(conf Conf, opts ...Option) *Provider {
	p := &Provider{
		key:      os.Getenv("DEEPSEEK_API_KEY"),
		base:     defaultBaseURL,
		model:    conf.Model,
		client:   &http.Client{},
		thinking: true,
	}
	for _, opt := range opts {
		if opt == nil {
			continue
		}
		if err := opt(p); err != nil {
			slog.Error("deepseek: option rejected", "err", err)
		}
	}
	if p.key == "" {
		slog.Warn("deepseek: no API key: set DEEPSEEK_API_KEY or pass WithAPIKey")
	}
	return p
}

// WithAPIKey sets the key sent as the Bearer token, overriding whatever
// DEEPSEEK_API_KEY holds. It is not validated here: a wrong key is the
// server's to reject, and an empty one fails the first turn exactly as a
// missing environment variable does.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		return nil
	}
}

// WithBaseURL overrides the host, without the /chat/completions path; a
// trailing slash is trimmed. A value that is empty or does not parse to a
// scheme and a host is rejected, which New logs while keeping the default, so
// a bad base is a line at construction rather than a panic on the first turn.
// api.deepseek.com/beta enables strict tool schemas and
// assistant prefixes, neither of which this adapter emits; api.deepseek.com/anthropic
// is a different wire format and will not work.
func WithBaseURL(base string) Option {
	return func(p *Provider) error {
		if base == "" {
			return errors.New("deepseek: empty base URL")
		}
		u, err := url.Parse(base)
		if err != nil {
			return fmt.Errorf("deepseek: base URL %q: %w", base, err)
		}
		if u.Scheme == "" || u.Host == "" {
			return fmt.Errorf("deepseek: base URL %q: no scheme or host", base)
		}
		p.base = strings.TrimSuffix(base, "/")
		return nil
	}
}

// WithHTTPClient replaces the default client, which is how a proxy, extra
// middleware or a test server gets in. Leave its Timeout at zero for the
// reason New gives: it caps the whole exchange, and lib.Options.Idle is the
// deadline shaped for streams.
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) error {
		p.client = client
		return nil
	}
}

// WithThinking turns reasoning on or off. It is on by default both here and on
// the server, and this adapter always sends the field explicitly so a change to
// the server default cannot quietly change what a turn does.
//
// Turning it off is not free of consequences elsewhere: the sampling knobs
// below only reach the model when thinking is off, and reasoning replay stops
// mattering.
func WithThinking(on bool) Option {
	return func(p *Provider) error {
		p.thinking = on
		return nil
	}
}

// WithReasoningEffort sets reasoning_effort inside the thinking object, which
// is where the API reference's parameter table puts it.
//
// DeepSeek's own pages disagree about this. The thinking mode guide and the
// landing page curl both show it as a top level sibling of thinking; the
// reference table lists it only as a property of thinking. Sending it in the
// wrong place almost certainly does not fail, it is silently ignored, which is
// the worse outcome because the knob then appears to work. WithTopLevelReasoningEffort
// is the other spelling; use it if a live account shows this one having no
// effect. They are never sent together.
func WithReasoningEffort(e Effort) Option {
	return func(p *Provider) error {
		p.effort, p.effortTopLevel = e, false
		return nil
	}
}

// WithTopLevelReasoningEffort sets reasoning_effort as a top level field. See
// WithReasoningEffort for why both spellings exist.
func WithTopLevelReasoningEffort(e Effort) Option {
	return func(p *Provider) error {
		p.effort, p.effortTopLevel = e, true
		return nil
	}
}

// WithTemperature sets temperature, which reaches the model only when thinking
// is disabled. DeepSeek documents thinking mode as not supporting it and as
// accepting it without effect, so this adapter omits it from the body entirely
// while thinking is on rather than sending a field that does nothing.
func WithTemperature(t float64) Option {
	return func(p *Provider) error {
		p.temperature = &t
		return nil
	}
}

// WithTopP sets top_p, under the same thinking mode restriction as
// WithTemperature.
func WithTopP(t float64) Option {
	return func(p *Provider) error {
		p.topP = &t
		return nil
	}
}

// WithStop sets stop sequences, up to the documented sixteen.
func WithStop(stop ...string) Option {
	return func(p *Provider) error {
		p.stop = stop
		return nil
	}
}

// WithMaxTokens caps the response.
//
// The default is to send no cap at all. DeepSeek does not require one and
// publishes no default for the v4 models, so inventing a number here would
// truncate long turns for no stated reason. A value above the documented 384K
// ceiling is refused rather than lowered to it: whoever configures a provider
// can be told the number was not the one in effect, which is not true of a
// model mid turn.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n > maxOutputTokens {
			return fmt.Errorf("deepseek: max tokens %d is over the documented ceiling of %d", n, maxOutputTokens)
		}
		p.maxTokens = n
		return nil
	}
}

// WithUserID sets user_id. It is not only telemetry: it partitions the KV cache
// and the concurrency allowance, so a value that varies per request destroys
// cache hits across requests and gives every request its own quota. Leave it
// unset unless the isolation is what is wanted.
func WithUserID(id string) Option {
	return func(p *Provider) error {
		p.userID = id
		return nil
	}
}

// WithTransport sets the limits a turn is driven under: how long the wire may
// stay silent, and how many bytes one frame may accumulate. Both live in
// lib.Options, which documents them and holds their defaults, and this adapter
// hands the value to lib.Stream without reading it.
//
// A turn that crosses the idle deadline ends with api.CodeNetwork, which is
// what api.Provider says an adapter's own deadline reports. Every line read
// resets the clock, the `: keep-alive` comments DeepSeek holds a queued request
// open with included, so it measures a dead socket rather than a slow turn.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.transport = opt
		return nil
	}
}

var _ api.Provider = (*Provider)(nil)

func (p *Provider) Name() string {
	return Name
}

// Model answers from this Provider's configuration, so it is callable before
// the first turn. Nothing here comes off the wire: DeepSeek publishes the
// context window in its docs and reports it in no response.
func (p *Provider) Model() api.Model {
	return p.model
}

func (p *Provider) Stream(ctx context.Context, req api.Request) <-chan api.Event {
	httpReq, sent, err := p.request(ctx, req)
	if err != nil {
		return lib.Drain(api.EventErr{Code: errorCodeFor(err), Err: err})
	}
	return lib.Stream(ctx, p.client, httpReq, newConverter(p.model.Name, sent, p.model.RequestBytes), p.transport)
}

// request encodes the turn. http.NewRequestWithContext cannot fail on what is
// passed to it here — the method is a constant and the URL is the configured
// base with a fixed path, and WithBaseURL refuses a base that does not parse —
// so the panic below guards a case construction already made unreachable.
func (p *Provider) request(ctx context.Context, req api.Request) (*http.Request, int, error) {
	if p.key == "" {
		return nil, 0, errNoKey
	}
	body, err := p.encode(req)
	if err != nil {
		return nil, 0, err
	}
	raw, err := lib.MarshalRaw(body)
	if err != nil {
		return nil, 0, fmt.Errorf("deepseek: encoding request: %w", err)
	}
	if p.model.RequestBytes > 0 && len(raw) > p.model.RequestBytes {
		return nil, len(raw), fmt.Errorf("%w: the request is %d bytes, over the %d the model %s takes", errTooLarge, len(raw), p.model.RequestBytes, p.model.Name)
	}
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, p.base+completionPath, bytes.NewReader(raw))
	if err != nil {
		panic(err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "text/event-stream")
	httpReq.Header.Set("Authorization", "Bearer "+p.key)
	return httpReq, len(raw), nil
}

// errorCodeFor classifies a turn that never left this process. A missing key is
// the one failure here that is not about the request's contents.
func errorCodeFor(err error) api.ErrCode {
	if errors.Is(err, errNoKey) {
		return api.CodeAccess
	}
	if errors.Is(err, errTooLarge) {
		return api.CodeTooLarge
	}
	return api.CodeInvalid
}
