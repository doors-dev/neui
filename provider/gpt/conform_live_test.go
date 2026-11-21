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

//go:build live

package gpt

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"neui.dev/api"
	"neui.dev/internal/testkit/conform"
)

func TestConformLive(t *testing.T) {
	conform.RunLive(t, conformAdapter())
}

func TestLiveCross(t *testing.T) {
	conform.RunLiveCross(t, conformAdapter())
}

func TestLiveXMLDocumentIsRead(t *testing.T) {
	if os.Getenv("OPENAI_API_KEY") == "" {
		t.Skip("OPENAI_API_KEY is not set")
	}
	p, err := New(Conf{Model: GPT56Luna}, WithEffort(EffortNone), WithMaxTokens(64))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var answer strings.Builder
	var last api.Event
	for e := range p.Stream(ctx, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
		api.ContentText{Text: "What is the code in the attached order? Answer with the code only."},
		api.ContentDocument{Data: []byte("<order><item>lamp</item><code>K7Q2-XW</code></order>"), Mime: "application/xml", Name: "order.xml"},
	}}}}) {
		if text, ok := e.(api.EventText); ok {
			answer.WriteString(text.Text)
		}
		last = e
	}
	if _, ok := last.(api.EventOk); !ok {
		t.Fatalf("an application/xml document ended the turn with %v, want EventOk", last)
	}
	if !strings.Contains(answer.String(), "K7Q2-XW") {
		t.Fatalf("the model answered %q, want the code from the document", answer.String())
	}
	t.Logf("answered %q", answer.String())
}
