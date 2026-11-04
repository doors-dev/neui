// Package gpt is the OpenAI provider for neui. Build one with New and hand it
// to neui.WithProvider or neui.OptionProvider:
//
//	p := gpt.New(gpt.Conf{Model: gpt.GPT56Luna}, gpt.WithReasoningEffort(gpt.EffortHigh))
//	neui.Enable(ctx, neui.OptionProvider(p))
//
// The key is read from OPENAI_API_KEY unless WithAPIKey sets one. Model and
// settings are fixed per Provider, so the same model at two efforts is two
// Providers. It speaks the Responses API rather than Chat Completions, because
// only Responses carries reasoning across turns.
package gpt

import (
	"context"
	"errors"
	"net/http"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// Provider is an api.Provider for one OpenAI model. It is safe for concurrent
// use by any number of sessions.
type Provider struct {
	apiKey      string
	baseURL     string
	client      *http.Client
	org         string
	project     string
	model       api.Model // the one model this adapter streams with, spelled as Conf wrote it
	set         settings
	keyExplicit bool
}

// Name returns "gpt".
func (p *Provider) Name() string {
	return "gpt"
}

// Model reports the model this adapter is configured with, as Conf wrote it.
func (p *Provider) Model() api.Model {
	return p.model
}

// Stream sends one turn and streams its events as api.Provider describes;
// read until the channel closes or cancel ctx. A request that cannot be built,
// such as one with no model, ends at once with CodeInvalid, and one larger
// than a nonzero Model.RequestBytes with CodeTooLarge. A failure the service
// reports arrives as an EventErr whose Err is or wraps an *APIError.
func (p *Provider) Stream(ctx context.Context, req api.Request) <-chan api.Event {
	httpReq, sent, err := p.request(ctx, req)
	if errors.Is(err, errTooLarge) {
		return lib.Drain(api.EventErr{Code: api.CodeTooLarge, Err: err})
	}
	if err != nil {
		return lib.Drain(api.EventErr{Code: api.CodeInvalid, Err: err})
	}
	return lib.Stream(ctx, p.client, httpReq, newConverter(p.model.Name, sent, p.model.RequestBytes), p.set.transport)
}
