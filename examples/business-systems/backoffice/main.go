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
	"log"
	"net/http"
	"os"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	_ "modernc.org/sqlite"
	"neui.dev"
	"neui.dev/api"
	"neui.dev/nekit"
	"neui.dev/provider/claude"
	"neui.dev/provider/deepseek"
	"neui.dev/provider/gpt"
)

func main() {
	var provider api.Provider
	var err error
	switch {
	case os.Getenv("DEEPSEEK_API_KEY") != "":
		provider, err = deepseek.New(deepseek.Conf{Model: deepseek.Flash})
	case os.Getenv("OPENAI_API_KEY") != "":
		provider, err = gpt.New(gpt.Conf{Model: gpt.GPT61Terra}, gpt.WithEffort(gpt.EffortMedium))
	case os.Getenv("ANTHROPIC_API_KEY") != "":
		provider, err = claude.New(claude.Conf{Model: claude.Haiku55},
			claude.WithThinking(claude.Thinking{Type: claude.ThinkingAdaptive, Display: claude.DisplaySummarized}),
			claude.WithEffort(claude.EffortMedium))
	default:
		log.Fatal("set DEEPSEEK_API_KEY, OPENAI_API_KEY or ANTHROPIC_API_KEY")
	}
	if err != nil {
		log.Fatal(err)
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
	}, neui.WithProvider(provider), neui.WithSystemPrompt(
		"Back office of Kettle & Burr, a small coffee gear shop. "+
			"The user works on the pages next to the chat: prefer acting there, so they see it happen. "+
			"Deletions happen only when the user confirms them on the page.",
	), neui.WithModules(
		neui.ModuleJS,
		neui.ModuleXLS(neui.XLSConfig{}),
	))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}
	log.Printf("serving on http://localhost:%s", port)
	log.Fatal(http.ListenAndServe(":"+port, app))
}
