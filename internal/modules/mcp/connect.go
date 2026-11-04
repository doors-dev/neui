package mcp

import (
	"context"
	"net/http"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"neui.dev/internal/utils"
)

type transport struct {
	base    http.RoundTripper
	headers map[string]string
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	req = req.Clone(req.Context())
	for k, v := range t.headers {
		req.Header.Set(k, v)
	}
	return t.base.RoundTrip(req)
}

func connect(
	ctx context.Context,
	url string,
	headers map[string]string,
	ping time.Duration,
	onToolChange func(*mcp.ClientSession),
) (*mcp.ClientSession, error) {
	httpClient := &http.Client{
		Transport: transport{base: http.DefaultTransport, headers: headers},
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(
		&mcp.Implementation{Name: utils.LibPath, Version: utils.LibVersion}, &mcp.ClientOptions{
			ToolListChangedHandler: func(ctx context.Context, req *mcp.ToolListChangedRequest) {
				onToolChange(req.Session)
			},
			MultiRoundTrip: &mcp.MultiRoundTripOptions{Disabled: true},
			KeepAlive:      ping,
		})
	session, err := client.Connect(ctx, transport, nil)
	if err != nil {
		return nil, err
	}
	return session, nil
}
