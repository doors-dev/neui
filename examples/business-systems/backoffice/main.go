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
	"neui.dev/nekit"
	"neui.dev/provider/deepseek"
)

func main() {
	provider, err := deepseek.New(deepseek.Conf{Model: deepseek.Flash})
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
			"Refunds, cancellations and deletions happen only when the user confirms them on the page.",
	), neui.WithModules(
		neui.ModuleJS,
		neui.ModuleXLS(neui.XLSConfig{}),
	))
	port := os.Getenv("PORT")
	if port == "" {
		port = "8095"
	}
	log.Fatal(http.ListenAndServe(":"+port, app))
}
