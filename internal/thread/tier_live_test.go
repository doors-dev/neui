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

package thread_test

import (
	"context"
	"os"
	"testing"

	"neui.dev/api"
	"neui.dev/provider/claude"
	"neui.dev/provider/deepseek"
	"neui.dev/provider/gemini"
	"neui.dev/provider/generic"
	"neui.dev/provider/gpt"
)

func TestLiveAssistantReplay(t *testing.T) {
	req := assistantReplay(t)
	for _, a := range []struct {
		name string
		env  string
		open func(key string) (api.Provider, error)
	}{
		{"claude", "ANTHROPIC_API_KEY", func(key string) (api.Provider, error) {
			return claude.New(claude.Conf{Model: claude.Haiku55}, claude.WithAPIKey(key))
		}},
		{"gpt", "OPENAI_API_KEY", func(key string) (api.Provider, error) {
			return gpt.New(gpt.Conf{Model: gpt.GPT56Luna}, gpt.WithAPIKey(key))
		}},
		{"deepseek", "DEEPSEEK_API_KEY", func(key string) (api.Provider, error) {
			return deepseek.New(deepseek.Conf{Model: deepseek.Flash}, deepseek.WithAPIKey(key))
		}},
		{"gemini", "GEMINI_API_KEY", func(key string) (api.Provider, error) {
			return gemini.New(gemini.Conf{Model: gemini.FlashLite35}, gemini.WithAPIKey(key))
		}},
		{"generic", "OPENROUTER_API_KEY", func(key string) (api.Provider, error) {
			return generic.New(generic.Conf{Model: api.Model{Name: "qwen/qwen3.8-27b"}, BaseURL: "https://openrouter.ai/api/v1"}, generic.WithAPIKey(key))
		}},
	} {
		t.Run(a.name, func(t *testing.T) {
			key := os.Getenv(a.env)
			if key == "" {
				t.Skip(a.env + " is not set")
			}
			p, err := a.open(key)
			if err != nil {
				t.Fatal(err)
			}
			var last api.Event
			for e := range p.Stream(context.Background(), req) {
				last = e
			}
			if _, ok := last.(api.EventOk); !ok {
				t.Fatalf("the TierAssistant replay ended with %#v", last)
			}
		})
	}
}
