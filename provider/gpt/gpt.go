// Package gpt implements api.Provider against OpenAI's Responses API.
//
// Responses rather than Chat Completions. api.ContentReasoning carries Meta
// for the provider's own encoding of the thinking, which is shaped by this API's
// reasoning items and their encrypted_content. Chat Completions can express none
// of it and loses reasoning between turns entirely, which would leave
// ContentReasoning permanently empty here and make a tool loop on a reasoning
// model measurably worse.
package gpt

import (
	"context"
	"errors"
	"net/http"

	"neui.dev/api"
	"neui.dev/provider/lib"
)

// Provider is the adapter. It is safe for concurrent use: everything on it is
// read only after New returns, and a turn keeps its state on the goroutine
// Stream starts.
//
// keyExplicit separates "no key wanted" from "no key configured", so
// WithAPIKey("") reaches a gateway that authenticates some other way instead of
// falling back to the environment.
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

// Name says which API this is. An encrypted reasoning payload is bound to
// whoever issued it and means nothing anywhere else, but a stored thread
// outlives the choice of provider, so remembering which one produced it belongs
// to whoever keeps the thread rather than to the blocks or to this adapter.
func (p *Provider) Name() string {
	return "gpt"
}

// Model reports the model this adapter is configured with, as Conf wrote it.
func (p *Provider) Model() api.Model {
	return p.model
}

// Stream sends the turn and decodes the reply. The channel is unbuffered: every
// event is handed to a caller that is there to take it, and a send that blocks
// until the context is done is what ends a turn nobody is reading.
//
// The model and its settings come from the Provider rather than the turn, so
// the same model at two efforts is two Providers. The request is built out of
// what the caller wrote plus that configuration, and a refusal building it is a
// mistake in one or the other rather than an outcome, so no turn ever starts.
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
