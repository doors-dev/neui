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

package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	_ "modernc.org/sqlite"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/nekit"
	"neui.dev/provider/claude"
	"neui.dev/provider/deepseek"
	"neui.dev/provider/gemini"
	"neui.dev/provider/generic"
	"neui.dev/provider/gpt"
)

type headers map[string]string

func (h headers) RoundTrip(r *http.Request) (*http.Response, error) {
	r = r.Clone(r.Context())
	for name, value := range h {
		r.Header.Set(name, value)
	}
	return http.DefaultTransport.RoundTrip(r)
}

func main() {
	var provider api.Provider
	var err error
	client := &http.Client{}
	if v := os.Getenv("HEADERS"); v != "" {
		var h headers
		if err = json.Unmarshal([]byte(v), &h); err != nil {
			log.Fatalf("HEADERS: %v", err)
		}
		client.Transport = h
	}
	switch {
	case os.Getenv("GENERIC_URL") != "":
		window := 0
		if v := os.Getenv("GENERIC_CONTEXT"); v != "" {
			if window, err = strconv.Atoi(v); err != nil {
				log.Fatalf("GENERIC_CONTEXT: %v", err)
			}
		}
		name := os.Getenv("GENERIC_MODEL")
		if name == "" {
			name = "local"
		}
		opts := []generic.Option{generic.WithAPIKey(os.Getenv("GENERIC_KEY")), generic.WithHTTPClient(client)}
		if v := os.Getenv("GENERIC_BODY"); v != "" {
			var body map[string]any
			if err = json.Unmarshal([]byte(v), &body); err != nil {
				log.Fatalf("GENERIC_BODY: %v", err)
			}
			opts = append(opts, generic.WithExtraBody(body))
		}
		if v := os.Getenv("GENERIC_MAX_TOKENS"); v != "" {
			n, err := strconv.Atoi(v)
			if err != nil || n < 0 {
				log.Fatalf("GENERIC_MAX_TOKENS: %q is not a token count", v)
			}
			opts = append(opts, generic.WithMaxTokens(n))
		}
		simpleSchemas := false
		switch v := os.Getenv("SIMPLE_SCHEMAS"); v {
		case "", "0":
		case "1":
			simpleSchemas = true
		default:
			log.Fatalf("SIMPLE_SCHEMAS: %q is not 0 or 1", v)
		}
		provider, err = generic.New(generic.Conf{Model: api.Model{Name: name, Context: window, SimpleSchemas: simpleSchemas}, BaseURL: os.Getenv("GENERIC_URL")}, opts...)
	case os.Getenv("DEEPSEEK_API_KEY") != "":
		provider, err = deepseek.New(deepseek.Conf{Model: deepseek.Flash}, deepseek.WithHTTPClient(client))
	case os.Getenv("OPENAI_API_KEY") != "":
		provider, err = gpt.New(gpt.Conf{Model: gpt.GPT61Terra}, gpt.WithEffort(gpt.EffortLow), gpt.WithHTTPClient(client))
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		provider, err = claude.New(claude.Conf{Model: claude.Haiku55}, claude.WithEffort(claude.EffortMedium), claude.WithHTTPClient(client))
	case os.Getenv("GEMINI_API_KEY") != "":
		provider, err = gemini.New(gemini.Conf{Model: gemini.Hydrogen4}, gemini.WithEffort(gemini.EffortMedium), gemini.WithHTTPClient(client))
	default:
		log.Fatal("set GENERIC_URL, DEEPSEEK_API_KEY, OPENAI_API_KEY, ANTHROPIC_API_KEY or GEMINI_API_KEY")
	}
	if err != nil {
		log.Fatal(err)
	}
	options := []doors.With{neui.WithProvider(provider), neui.WithSystemPrompt(
		"Back office of Kettle & Burr, a small coffee gear shop. " +
			"The user works on the pages next to the chat: prefer acting there, so they see it happen. " +
			"Deletions happen only when the user confirms them on the page.",
	)}
	modules := []neui.ModuleFactory{neui.ModuleXLS(neui.XLSConfig{})}
	switch v := os.Getenv("TIER"); v {
	case "", "developer":
	case "agent":
		options = append(options, neui.WithTier(neui.TierAgent))
	case "assistant":
		options = append(options, neui.WithTier(neui.TierAssistant))
		modules = modules[1:]
	default:
		log.Fatalf("TIER: %q is not developer, agent or assistant", v)
	}
	db, err := sql.Open("sqlite", "neui.db")
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	threads, err := nekit.NewSQLiteMessages(db)
	if err != nil {
		log.Fatal(err)
	}
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Comp {
		return App{Threads: threads}
	}, append(options, neui.WithModules(modules...))...)
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}
	log.Printf("serving on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, app))
}
