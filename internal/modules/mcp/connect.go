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

package mcp

import (
	"context"
	"fmt"
	"net/http"
	neturl "net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"neui.dev/internal/utils"
)

type transport struct {
	base    http.RoundTripper
	scheme  string
	host    string
	headers map[string]string
}

func (t transport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.URL.Scheme != t.scheme || req.URL.Host != t.host {
		if req.Body != nil {
			req.Body.Close()
		}
		return nil, fmt.Errorf("%s://%s is not the MCP server at %s://%s, so the request was refused", req.URL.Scheme, req.URL.Host, t.scheme, t.host)
	}
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
	onToolChange func(),
) (*mcp.ClientSession, error) {
	endpoint, err := neturl.Parse(url)
	if err != nil {
		return nil, err
	}
	httpClient := &http.Client{
		Transport: transport{base: http.DefaultTransport, scheme: endpoint.Scheme, host: endpoint.Host, headers: headers},
	}
	transport := &mcp.StreamableClientTransport{
		Endpoint:   url,
		HTTPClient: httpClient,
	}
	client := mcp.NewClient(
		&mcp.Implementation{Name: utils.LibPath, Version: utils.LibVersion}, &mcp.ClientOptions{
			ToolListChangedHandler: func(ctx context.Context, req *mcp.ToolListChangedRequest) {
				onToolChange()
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
