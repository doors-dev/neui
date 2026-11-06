// Package deepseek is the api.Provider for DeepSeek's Chat Completions API.
//
// Build one with New and pass it to neui.WithProvider as the app-wide
// provider, or to neui.OptionProvider to set one session's provider, which
// takes precedence over WithProvider if any:
//
//	p, err := deepseek.New(deepseek.Conf{Model: deepseek.Flash},
//		deepseek.WithEffort(deepseek.EffortMax))
//
// The key comes from DEEPSEEK_API_KEY unless WithAPIKey sets it. A provider
// serves one model with one set of options; build two for two settings. New
// refuses a setting DeepSeek would reject or ignore, so a Provider that was
// built sends what it was given. Thinking is on by default and streams back as
// readable reasoning. Images go to deepseek-flash in user messages and tool
// results; every other model gets a short note in place of each image, and
// every model gets one in place of a document. Caching is automatic and needs no options:
// keep the tools, system prompt and history stable between turns to hit it.
// Failures reach the thread as api.EventErr with an api.ErrCode. Its Err is an
// *APIError for a non 200 response, a *RequestError for a request this adapter
// refused to encode, a *FinishError for a finish_reason that ended the turn
// without a reply, and a *StreamError for an error frame in the stream; read
// them with errors.As.
package deepseek

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// Name is what Provider.Name returns.
const Name = "deepseek"

const (
	defaultBaseURL = "https://api.deepseek.com"
	completionPath = "/chat/completions"
	// maxOutputTokens is the documented ceiling for both v4 models, against a
	// 1M context. A request above it is rejected, so WithMaxTokens refuses the
	// value rather than letting the turn fail at the server.
	maxOutputTokens = 393_216
	// maxToolNameLen is a documented request limit. Checking it here turns an
	// opaque 400 into a sentence naming the tool.
	maxToolNameLen = 128
	maxUserIDBytes = 512
)

// Effort is DeepSeek's reasoning_effort: EffortNone turns thinking off, and
// the other levels turn it on at that depth. The zero value leaves the choice
// to the server, which thinks at high.
type Effort string

// The reasoning_effort levels DeepSeek documents.
const (
	EffortNone Effort = "none"
	EffortLow  Effort = "low"
	EffortHigh Effort = "high"
	EffortMax  Effort = "max"
)

type family int

const (
	familyPro family = iota
	familyFlash
)

// Provider is one DeepSeek configuration: a model plus the settings
// api.Request leaves out (effort, sampling, stop sequences, response cap). It
// is safe for concurrent use and holds no per turn state.
type Provider struct {
	key         string
	base        string
	model       api.Model
	family      family
	client      *http.Client
	effort      Effort
	temperature *float64
	topP        *float64
	stop        []string
	userID      string
	maxTokens   int
	transport   lib.Options
}

// Option configures a Provider at New. An option that returns an error makes
// New return it.
type Option func(*Provider) error

// Conf holds the provider's model.
type Conf struct {
	// Model is the model every turn is sent to, such as Flash or V4Pro. There
	// is no default: New refuses an empty Model.Name.
	Model api.Model `json:"model"`
}

// New builds a Provider for conf.Model. The key defaults to DEEPSEEK_API_KEY.
// New returns the first option error, and an error for an empty Model.Name, a
// missing key, an Effort outside the declared levels, WithTemperature without
// WithEffort(EffortNone), and WithTopP where the model ignores it. The model
// rules follow the exact id: deepseek-flash and its legacy ids
// deepseek-v4-flash and deepseek-v4-flash-vision-exp take images and honor
// top_p with thinking on; deepseek-v4-pro and any id not listed here take no
// images and honor top_p with thinking off. The default HTTP client has no
// timeout, because DeepSeek may queue a request for minutes before answering;
// WithTransport bounds a dead connection instead.
func New(conf Conf, opts ...Option) (*Provider, error) {
	p := &Provider{
		key:    os.Getenv("DEEPSEEK_API_KEY"),
		base:   defaultBaseURL,
		model:  conf.Model,
		client: &http.Client{},
	}
	for _, opt := range opts {
		if err := opt(p); err != nil {
			return nil, err
		}
	}
	if p.model.Name == "" {
		return nil, errors.New("deepseek: no model, set Conf.Model")
	}
	if p.key == "" {
		return nil, errNoKey
	}
	switch p.effort {
	case "", EffortNone, EffortLow, EffortHigh, EffortMax:
	default:
		return nil, fmt.Errorf("deepseek: effort %q is not one of none, low, high or max", p.effort)
	}
	switch p.model.Name {
	case "deepseek-flash", "deepseek-v4-flash", "deepseek-v4-flash-vision-exp":
		p.family = familyFlash
	}
	if p.temperature != nil && p.effort != EffortNone {
		return nil, errors.New("deepseek: temperature has no effect in thinking mode, pass WithEffort(EffortNone) with it")
	}
	if p.topP != nil && (p.family == familyFlash) == (p.effort == EffortNone) {
		if p.family == familyFlash {
			return nil, fmt.Errorf("deepseek: %s fixes top_p at 1.0 with thinking off, so WithTopP needs thinking on", p.model.Name)
		}
		return nil, fmt.Errorf("deepseek: %s ignores top_p in thinking mode, pass WithEffort(EffortNone) with it", p.model.Name)
	}
	return p, nil
}

// WithAPIKey sets the key, overriding DEEPSEEK_API_KEY. New refuses an empty
// key; the server rejects a wrong one with api.CodeAccess.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		return nil
	}
}

// WithBaseURL points the provider at another host, such as a proxy or a test
// server, without the /chat/completions path; a trailing slash is trimmed. An
// empty value or one without a scheme and host makes New fail.
// https://api.deepseek.com/anthropic speaks a different wire format and does
// not work here.
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

// WithHTTPClient replaces the default client, for a proxy, middleware or a
// test server; it must not be nil. Leave its Timeout at zero: it caps the
// whole streamed turn. Use WithTransport to bound a dead connection.
func WithHTTPClient(client *http.Client) Option {
	return func(p *Provider) error {
		if client == nil {
			return errors.New("deepseek: WithHTTPClient needs a client")
		}
		p.client = client
		return nil
	}
}

// WithEffort sets the reasoning level. EffortNone sends thinking disabled and
// no reasoning_effort; the other levels send thinking enabled and that
// reasoning_effort; unset sends thinking enabled and leaves the level to the
// server. With thinking on, DeepSeek refuses a forced tool choice, so a
// request whose ToolChoice is api.ToolChoiceAny or names a tool fails with a
// *RequestError unless the effort is EffortNone.
func WithEffort(e Effort) Option {
	return func(p *Provider) error {
		p.effort = e
		return nil
	}
}

// WithTemperature sets temperature, from 0 to 2. DeepSeek ignores it in
// thinking mode, so New refuses it without WithEffort(EffortNone).
func WithTemperature(t float64) Option {
	return func(p *Provider) error {
		if !(t >= 0 && t <= 2) {
			return fmt.Errorf("deepseek: temperature %v is outside the documented range of 0 to 2", t)
		}
		p.temperature = &t
		return nil
	}
}

// WithTopP sets top_p, above 0 and at most 1. Only two settings honor it:
// deepseek-v4-pro with WithEffort(EffortNone), and deepseek-flash (or
// deepseek-v4-flash or deepseek-v4-flash-vision-exp) with thinking on, where
// the server raises values below 0.95 to 0.95. DeepSeek fixes top_p
// everywhere else, so New refuses it there.
func WithTopP(t float64) Option {
	return func(p *Provider) error {
		if !(t > 0 && t <= 1) {
			return fmt.Errorf("deepseek: top_p %v is outside the documented range of above 0 to 1", t)
		}
		p.topP = &t
		return nil
	}
}

// WithStop sets stop sequences, replacing any set before. DeepSeek takes up
// to sixteen; more are not checked here and are left to the server.
func WithStop(stop ...string) Option {
	return func(p *Provider) error {
		p.stop = stop
		return nil
	}
}

// WithMaxTokens caps the response; a turn that hits it ends with
// api.CodeCapped. At 0, the default, no cap is sent and the server's default
// applies. A negative value or one over the documented 393,216 ceiling makes
// New fail.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n < 0 {
			return fmt.Errorf("deepseek: max tokens %d is negative", n)
		}
		if n > maxOutputTokens {
			return fmt.Errorf("deepseek: max tokens %d is over the documented ceiling of %d", n, maxOutputTokens)
		}
		p.maxTokens = n
		return nil
	}
}

// WithUserID sets user_id, which partitions DeepSeek's cache and concurrency
// allowance by user. Requests with different ids share no cache, so leave it
// unset unless that isolation is wanted, and keep it fixed per provider. An id
// over 512 bytes, the documented limit, makes New fail.
func WithUserID(id string) Option {
	return func(p *Provider) error {
		if len(id) > maxUserIDBytes {
			return fmt.Errorf("deepseek: user id is %d bytes, over the documented limit of %d", len(id), maxUserIDBytes)
		}
		p.userID = id
		return nil
	}
}

// WithTransport sets the stream limits documented on lib.Options: how long the
// wire may stay silent and how large one event may grow. A zero field keeps
// lib's default. The idle deadline covers the wait for response headers and
// resets on every complete event and on every comment or blank line between
// events, including the keep-alive comments DeepSeek sends while a request is
// queued, so it catches a dead connection rather than a slow turn; a turn that
// crosses it ends with api.CodeNetwork.
func WithTransport(opt lib.Options) Option {
	return func(p *Provider) error {
		p.transport = opt
		return nil
	}
}

var _ api.Provider = (*Provider)(nil)

// Name returns "deepseek".
func (p *Provider) Name() string {
	return Name
}

// Model returns the model from Conf. It is fixed for the provider's lifetime.
func (p *Provider) Model() api.Model {
	return p.model
}

// Stream runs one turn on its own goroutine and returns its events. A turn that
// finishes ends in exactly one EventOk or EventErr, then the channel closes. A
// cancelled ctx closes it with neither, and so does a connection that stops
// before any finish_reason arrived; one that stops after it ends as that
// finish_reason does. A request this wire cannot carry, or one over
// Model.RequestBytes, ends at once in EventErr with api.CodeInvalid or
// api.CodeTooLarge. The thread calls it; hosts normally do
// not.
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

// errorCodeFor classifies a turn that never left this process.
func errorCodeFor(err error) api.ErrCode {
	if errors.Is(err, errTooLarge) {
		return api.CodeTooLarge
	}
	return api.CodeInvalid
}
