// Package deepseek is the api.Provider for DeepSeek's Chat Completions API.
//
// Build one with New and pass it to neui.WithProvider as the app-wide
// provider, or to neui.OptionProvider to set one session's provider, which
// takes precedence over WithProvider if any:
//
//	p := deepseek.New(deepseek.Conf{Model: deepseek.Flash},
//		deepseek.WithReasoningEffort(deepseek.EffortHigh))
//
// The key comes from DEEPSEEK_API_KEY unless WithAPIKey sets it. A provider
// serves one model with one set of options; build two for two settings.
// Thinking is on by default and streams back as readable reasoning. The wire
// carries text only, so images and documents in a request reach the model as
// a short note in their place. Caching is automatic and needs no options:
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
	"log/slog"
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
	maxOutputTokens = 384_000
	// maxTools and maxToolNameLen are documented request limits. Checking them
	// here turns an opaque 400 into a sentence naming the tool.
	maxTools       = 128
	maxToolNameLen = 64
)

// Effort is DeepSeek's reasoning_effort. Only two levels exist; the server
// folds low and medium into high and xhigh into max.
type Effort string

// The two reasoning_effort levels DeepSeek accepts.
const (
	EffortHigh Effort = "high"
	EffortMax  Effort = "max"
)

// Provider is one DeepSeek configuration: a model plus the settings
// api.Request leaves out (thinking, effort, sampling, stop sequences, response
// cap). It is safe for concurrent use and holds no per turn state.
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

// Option configures a Provider at New. An option that returns an error is
// logged and skipped.
type Option func(*Provider) error

// Conf holds the provider's model.
type Conf struct {
	// Model is the model every turn is sent to, such as Flash or V4Pro. There
	// is no default: an empty Model.Name fails every turn with a *RequestError
	// under api.CodeInvalid.
	Model api.Model `json:"model"`
}

// New builds a Provider for conf.Model. The key defaults to DEEPSEEK_API_KEY;
// without one New logs a warning and every turn fails with api.CodeAccess. A
// nil option is skipped, and an option that fails is logged and leaves its
// setting as it was. The default HTTP client has no timeout, because DeepSeek
// may queue a request for minutes before answering; WithTransport bounds a
// dead connection instead.
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

// WithAPIKey sets the key, overriding DEEPSEEK_API_KEY. An empty key fails
// every turn with api.CodeAccess; a wrong one is rejected by the server with
// the same code.
func WithAPIKey(key string) Option {
	return func(p *Provider) error {
		p.key = key
		return nil
	}
}

// WithBaseURL points the provider at another host, such as a proxy or a test
// server, without the /chat/completions path; a trailing slash is trimmed. An
// empty value or one without a scheme and host is rejected at New and the
// default stays. https://api.deepseek.com/anthropic speaks a different wire
// format and does not work here.
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
		p.client = client
		return nil
	}
}

// WithThinking turns reasoning on or off; it is on by default and always sent
// explicitly. WithTemperature and WithTopP take effect only with it off.
func WithThinking(on bool) Option {
	return func(p *Provider) error {
		p.thinking = on
		return nil
	}
}

// WithReasoningEffort sets reasoning_effort inside the thinking object, where
// the API reference puts it. DeepSeek's guides show it as a top level field
// instead, and a misplaced field is silently ignored, so if this one has no
// visible effect switch to WithTopLevelReasoningEffort. The last of the two
// options given wins.
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

// WithTemperature sets temperature. It is sent only while thinking is off
// (WithThinking(false)); with thinking on DeepSeek ignores it, so it is left
// out.
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

// WithStop sets stop sequences, replacing any set before. DeepSeek takes up
// to sixteen; more are not checked here and are left to the server.
func WithStop(stop ...string) Option {
	return func(p *Provider) error {
		p.stop = stop
		return nil
	}
}

// WithMaxTokens caps the response; a turn that hits it ends with
// api.CodeCapped. By default, and for a value of 0 or below, no cap is sent.
// A value over the documented 384,000 ceiling is logged at New and the
// previous setting stays.
func WithMaxTokens(n int) Option {
	return func(p *Provider) error {
		if n > maxOutputTokens {
			return fmt.Errorf("deepseek: max tokens %d is over the documented ceiling of %d", n, maxOutputTokens)
		}
		p.maxTokens = n
		return nil
	}
}

// WithUserID sets user_id, which partitions DeepSeek's cache and concurrency
// allowance by user. Requests with different ids share no cache, so leave it
// unset unless that isolation is wanted, and keep it fixed per provider.
func WithUserID(id string) Option {
	return func(p *Provider) error {
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
// finish_reason does. A missing key, a request this wire cannot carry, or one over
// Model.RequestBytes ends at once in EventErr with api.CodeAccess,
// api.CodeInvalid or api.CodeTooLarge. The thread calls it; hosts normally do
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
