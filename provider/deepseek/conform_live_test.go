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

package deepseek

import (
	"bytes"
	"context"
	stdimage "image"
	"image/png"
	"os"
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

func TestLiveV4ProReadsNoImages(t *testing.T) {
	if os.Getenv("DEEPSEEK_API_KEY") == "" {
		t.Skip("DEEPSEEK_API_KEY is not set")
	}
	if V4Pro.Inputs.Has(api.ModalityImage) {
		t.Fatalf("V4Pro declares Inputs %v, which reads images", V4Pro.Inputs)
	}
	var shot bytes.Buffer
	if err := png.Encode(&shot, stdimage.NewGray(stdimage.Rect(0, 0, 8, 8))); err != nil {
		t.Fatal(err)
	}
	p, err := New(Conf{Model: V4Pro}, WithEffort(EffortNone), WithMaxTokens(16))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	var last api.Event
	var usage api.EventUsage
	for e := range p.Stream(ctx, api.Request{Messages: []api.Message{{Role: api.RoleUser, Content: api.Contents{
		api.ContentText{Text: "What colour is this square? One word."},
		api.ContentImage{Data: shot.Bytes(), Mime: "image/png"},
	}}}}) {
		if u, ok := e.(api.EventUsage); ok {
			usage = u
		}
		last = e
	}
	if _, ok := last.(api.EventOk); !ok {
		t.Fatalf("an image sent to %s ended %v, want EventOk with the image dropped", V4Pro.Name, last)
	}
	if read := usage.Input + usage.CacheRead + usage.CacheWrite; read >= 50 {
		t.Fatalf("an image sent to %s billed %d input tokens, want under 50: if it reads images now, declare ModalityImage", V4Pro.Name, read)
	}
}
