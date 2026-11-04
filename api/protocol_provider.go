package api

import "context"

// Provider is one model API. It converts a Request into whatever that API wants
// and the reply back into Events.
type Provider interface {
	// Name identifies the API. Signatures and sealed payloads are bound to the
	// API that issued them and cannot be replayed against another, so whoever
	// stores a conversation stores this name with it. No Content block carries it.
	Name() string

	// Stream returns at once and delivers the reply on the channel, in the order
	// it was produced.
	//
	// A turn that ends on its own ends in EventOk or EventErr, then the channel
	// closes. A connection that dies mid-reply ends in neither: an adapter must
	// not invent an ending, and a closed channel with no terminal event means an
	// incomplete reply.
	//
	// A caller that stops reading must cancel ctx, or the provider blocks. Cancel,
	// and a caller deadline on ctx, end the turn: the caller reads ctx.Err() rather
	// than a terminal event. Events already queued may still arrive before the
	// channel closes, and a caller that cancelled ignores whatever it is handed
	// from then on rather than counting it; what is promised is that the channel
	// closes and that no ending is invented. An adapter holds nothing open after
	// cancel. A deadline the provider keeps itself, such as an idle timeout, is
	// invisible in ctx and so ends the turn with EventErr and CodeNetwork.
	Stream(ctx context.Context, req Request) <-chan Event

	// Model is the model this provider was configured with: callable before the
	// first turn, and constant across turns.
	Model() Model
}

// Model names a model and carries what the API never reports about it. A
// provider package declares the models it knows as Model values; a caller may
// pass its own literal for a model the package does not list, or to override
// what it lists.
type Model struct {
	// Name is the id as it goes on the wire.
	Name string `json:"name"`
	// Context is how many tokens the model reads at once. Zero means unknown,
	// not a window of zero; a caller reading zero skips token-based compaction,
	// leaving the ThreadMemoryLimit byte budget and the API's own over-length
	// error to start it.
	Context int `json:"context"`
	// ImagePixels is the pixel area, width times height, worth sending in one
	// image: past it the API either downscales or refuses, so the extra pixels
	// buy nothing. Zero means unknown, and a caller reading zero sends what it
	// has.
	ImagePixels int `json:"imagePixels"`
	// ImageBytes is the largest one encoded image the API takes. Zero means
	// unknown.
	ImageBytes int `json:"imageBytes"`
	// ImagesPerRequest is how many images the API reads in one request. It
	// tracks the model's context rather than its byte limit, since every image
	// costs tokens. A PDF charges one per page, the API rendering every page to
	// an image. Zero means unknown.
	ImagesPerRequest int `json:"imagesPerRequest"`
	// RequestBytes is the largest request the API takes, every block of every
	// message included. Zero means unknown.
	RequestBytes int `json:"requestBytes"`
}
