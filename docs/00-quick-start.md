# Quick start

With Neui, a page instance of your Doors app gets an agent thread alongside it. The agent works inside that page: it sees what you let it see, uses the controls you mark, calls the tools you render, and acts with the user's permissions. It can also build its own UI right on the page.

## Set up

> New to Doors? Start with its [get started guide](https://doors.dev/docs/get-started).

Add neui to your module:

```sh
go get neui.dev
```

Then create the Doors app with a provider and serve it:

```go
// main.go
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/doors-dev/doors"
	"github.com/doors-dev/gox"
	"neui.dev"
	"neui.dev/provider/deepseek"
)

func main() {
	provider, err := deepseek.New(deepseek.Conf{Model: deepseek.Flash}) // reads DEEPSEEK_API_KEY; a missing key fails the first turn
	if err != nil {
		log.Fatal(err)
	}
	app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Comp {
		return App{}
	}, neui.WithProvider(provider))
	log.Fatal(http.ListenAndServe(":8080", app))
}
```

Four providers come included: Claude (`neui.dev/provider/claude`), GPT (`neui.dev/provider/gpt`), Gemini (`neui.dev/provider/gemini`) and DeepSeek (`neui.dev/provider/deepseek`). For any server with an OpenAI-compatible Chat Completions API, such as Ollama, vLLM or OpenRouter, use `neui.dev/provider/generic`. To use another model, implement `api.Provider`.

Then enable neui for the user, typically right after sign-in:

```go
err := neui.Enable(ctx)
```

It applies to the user's whole session: every page they have open picks it up, and so does every page they open later.

While developing, you can just enable it at the top of your page:

```gox
// app.gox
package main

import "neui.dev"

type App struct{}

elem (a App) Main() {
	~~
	neui.Enable(ctx)
	~~
	<!doctype html>
	<html lang="en">
		<head>
			<meta charset="utf-8">
			<title>My app</title>
		</head>
		<body>
			<main>...</main>
		</body>
	</html>
}
```

> Enable is idempotent: once the session is enabled, later calls do nothing. Pass `neui.OptionForce()` to rebuild an enabled session with new options.

## Storage

By default threads live in memory and uploads in a temporary directory, so both are gone when the session ends.

### Threads

Implement `api.MessageStorage`, or use the included SQLite storage. Open the database with any SQLite driver in `main.go` and pass the storage to your page:

```go
// main.go
import (
	"database/sql"

	_ "modernc.org/sqlite"
	"neui.dev/nekit"
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
	}, neui.WithProvider(provider))
	log.Fatal(http.ListenAndServe(":8080", app))
}
```

Give each user their part when you enable neui:

```gox
// app.gox
type App struct {
	Threads nekit.SQLiteMessages
}

elem (a App) Main() {
	~~
	neui.Enable(ctx, neui.OptionMessageStorage(a.Threads.User(userID)))
	~~
	...
}
```

### Uploads

Implement `api.UploadStorage`, or use an included one. The simplest keeps files in a directory on disk. Set it once for the whole app:

```go
// main.go
files, err := nekit.NewDiskUploads("uploads")
if err != nil {
	log.Fatal(err)
}
app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Comp {
	return App{Threads: threads}
}, neui.WithProvider(provider), neui.WithUploadStorage(files))
```

> For production systems, use S3 instead: `nekit.NewS3Uploads(client, bucket, prefix)`, with a minio client for any S3-compatible service.

## Chat

Drop the ready-made chat into your page:

```gox
// app.gox
import (
	"neui.dev"
	"neui.dev/nekit"
)

elem (a App) Main() {
	...
		<body>
			<main>...</main>
			~nekit.Chat{}
		</body>
	...
}
```

It floats over the page as a button in the corner, so your markup stays as it is. The user opens it as a window or docks it to the side.

It shows only while neui is enabled for the session, so render it unconditionally.

### Questions in the chat

During a conversation the agent can also ask the user something right in the chat: a small form shown in place of the message box, whose answer goes back to the agent. Any module can provide such questions with `Module.Ask`. Its handler gets the agent's input and returns the form and its submit button, which hands the answer back:

```gox
// app.gox
m.Ask("confirm", "Ask the user to confirm an action", func(in ConfirmInput, answer neui.Answer[bool]) api.ChatAsk {
	return api.ChatAsk{
		Form:   <p>~(in.Question)</p>,
		Button: <button (doors.AClick{On: func(ctx context.Context, _ doors.RequestPointer) bool {
			answer.Submit(ctx, true)
			return false
		}})>Confirm</button>,
	}
})
```

If the user dismisses the question, the agent's call fails. nekit ships the most common one, picking from a list of options with an optional answer of their own. Register it like any module:

```go
// main.go
neui.WithModules(nekit.ModuleAsk())
```

> `nekit.Chat` has layout, feature and theming options; see its godoc. For full control, build your own chat on `neui.Instance`, or start from a copy of `nekit.Chat`.

## Modules

Tools in neui are organized into modules. In a module you register Go functions, and the agent can call them as tools. The input and output schemas are generated from your Go types with [swaggest/jsonschema-go](https://github.com/swaggest/jsonschema-go), so add tags with descriptions and validation:

```go
// main.go
type ForecastInput struct {
	City string `json:"city" required:"true" description:"city name, like Berlin"`
	Days int    `json:"days" minimum:"1" maximum:"7" description:"days ahead; default 1"`
}

type ForecastOutput struct {
	Days []string `json:"days" description:"one line per day"`
}

func weather(ctx context.Context) neui.AnyModule {
	m := neui.NewModule("weather", "Weather forecasts for any city.")
	_, err := m.Tool("forecast", "Forecast for a city.", func(ctx context.Context, in ForecastInput) (ForecastOutput, error) {
		...
	})
	if err != nil {
		panic(err) // invalid schema
	}
	return m
}
```

> In this setup a module is created for each chat thread. If it creates resources, release them when ctx is done.

Enable it for all users when you create the app:

```go
// main.go
app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Comp {
	return App{}
}, neui.WithProvider(provider), neui.WithModules(weather))
```

Or for one user only when you enable neui:

```go
err := neui.Enable(ctx, neui.OptionModules(weather))
```

### Included modules

By default only the system modules are on. Except at `TierAssistant` they include js, which lets the agent run sandboxed scripts to automate work, calling your tools from the script. These come included:

- `ModuleXLS` lets the agent read and build Excel workbooks.
- `ModuleWeb` lets the agent read web pages, and search the web when you give it a search backend.
- `ModuleMCP` plugs in the tools of any remote MCP server.

Enable them all, with Exa search over MCP:

```go
// main.go
app := doors.NewApp(func(ctx context.Context, r doors.Request) gox.Comp {
	return App{}
}, neui.WithProvider(provider), neui.WithModules(
	neui.ModuleXLS(neui.XLSConfig{}),
	neui.ModuleWeb(neui.WebConfig{}),
	neui.ModuleMCP(neui.MCPConfig{Name: "exa", URL: "https://mcp.exa.ai/mcp"}),
))
```

> Module tools are static: they don't depend on the page and are always available.

## Integration

The agent can help the user find their way around your app, offer features your UI doesn't have, and automate routine work. You enable this with markup on your pages.

### Spaces

First, organize page regions into spaces. Spaces separate the functional areas of your app and help the agent understand the page:

```gox
~>neui.Space{Name: "navigation", Desc: "main navigation menu"} <nav>...</nav>
~>neui.Space{Name: "product", Desc: "product card"} <section>...</section>
```

### Inputs

Next, mark what the agent can use: put `neui.Use` in front of the element. With `neui.R` the agent can't press it, so the user gives the final confirmation:

```gox
~>neui.Use{Desc: "go to the orders list"} <a (doors.ALink{Model: Path{Section: SectionOrders}})>Orders</a>
~>neui.Use{Desc: "place the order", Rights: neui.R} <button type="submit">Place order</button>
```

Set `Highlight: true` on the main menu's links: the agent's page tree then lists their descriptions, so it sees where it can go from anywhere. Describe each link by what its destination offers, not only where it leads: `"go to the orders list: search, filter by status and dates, sort by any column"`.

Keep one task per space: a form, its fields and its submit button together. It makes the agent's work easier:

```gox
~>neui.Space{Name: "shipping", Desc: "shipping address form"} <form (doors.ASubmit[Address]{On: save})>
	~>neui.Use{Desc: "street and house number"} <input name="street">
	~>neui.Use{Desc: "city"} <input name="city">
	~>neui.Use{Desc: "save the address"} <button type="submit">Save</button>
</form>
```

Supported elements: `a`, `button`, `input`, `textarea` and `select`. A radio input stands for its whole group.

### Data

Give the agent extra context with `neui.Data`:

```gox
~neui.Data{Text: "the product shown in the card", Object: product}
```

> The agent can't see your page's HTML or use elements you didn't mark. You decide what it sees and what it may use.

### Tools

Just as you render buttons and forms for the user depending on context and permissions, you render tools for the agent inside your markup. A tool rendered in a space belongs to it and lives as long as that part of the page.

```gox
// app.gox
~>neui.Space{Name: "orders", Desc: "the orders list the user sees"} <section>
	~neui.Tool[Filter, struct{}]{
		Name: "filter",
		Desc: "Filter the list the user sees by status and dates.",
		Handler: func(ctx context.Context, f Filter) (struct{}, error) {
			filter.Update(ctx, f)
			return struct{}{}, nil
		},
	}
	~(filter.Bind(table))
</section>
```

The handler runs in the user's page, so it updates the same state the user's controls do: the user sees the filtered list, and the agent gets back what changed on the page. Input and output schemas come from your Go types, the same way as for module tools.

Render a tool only where the user may use it, with the same checks as the rest of your UI:

```gox
// app.gox
~(if user.Admin {
	~neui.Tool[StockInput, Product]{
		Name: "set_stock",
		Desc: "Set the stock of this product.",
		Handler: setStock,
	}
})
```

Put operations the agent may need from anywhere, such as changing any order by id, in a space in your app shell: they're on every page.

Page tools are the main way to let the agent operate your app. Prefer them over modules because:

- **Permissions come for free.** A tool exists only where your render put it, for the user it was rendered for.
- **They reuse your business logic.** Context, data dependencies and changes come naturally with your UI declaration.
- **They scale.** You can have hundreds of tools across your app; the agent discovers them on demand, only the ones the task at hand needs.

> Keep modules for tools that don't depend on the page or your business logic, like the included JS, XLS, web and MCP modules.

## Generative apps

The agent can build mini apps and interactive screens that let users solve their specific tasks.

To enable it, set aside a place for it on your page with an island:

```gox
// app.gox
~neui.Island{
	Name:        "report",
	Desc:        "Reports on the shop's sales: its apps read orders, customers and products, rank customers by what they paid, chart sales and set product stock.",
	Placeholder: <p>Ask the assistant for a report.</p>,
	Template: func(slot gox.Comp) gox.Elem {
		return <div class="card">~(slot)</div>
	},
	Library: library,
}
```

The agent decides whether an island fits a task from its description, so say what its apps can work with and do, not where the island sits.

You can have many islands across your app, each serving a specific need with its own set of building blocks, or one Swiss Army knife.

The agent writes a script that wires the components of the island's library into the flow the user needs. You register them in the library:

```go
// app.gox
func library(l neui.Library) error {
	l.Layout("card", "a titled card around its items", card)
	l.View("receipt", "a printable receipt of an order", receipt)
	l.Control("rating", "asks the user for a rating from 1 to 5", rating)
	return nil
}
```

- **Layouts** arrange other components, like `card`.
- **Views** present information, like `receipt`.
- **Controls** collect input from the user and pass it to the script, like `rating`.

Each one is a Go function that renders your component from the params the script passes.

Island scripts can call your agent's modules except page and JS. Give an island the tools its apps need in its library; `l.Add` takes the same `neui.Tool` struct you render on the page. The island also exposes them to the agent as tools of its app space.

```go
// app.gox
l.Add(neui.Tool[StockInput, Product]{
	Name: "set_stock",
	Desc: "Set the stock of a product.",
	Handler: setStock,
})
l.Tool("top_customers", "customers ranked by what they paid", topCustomers)
```

Out of the box neui ships a library of powerful widgets (layouts, markdown, forms, tables, buttons, charts and stat tiles) that covers almost any workflow. Add it to your library with:

```go
// app.gox
func library(l neui.Library) error {
	nekit.IslandRegisterAll(l)
	return nil
}
```

> You can customize the default styles, or register only the widgets you want.

## Configuration

Set neui up for the whole app when you create the Doors app:

```go
// main.go
app := doors.NewApp(page,
	neui.WithProvider(provider),
	neui.WithSystemPrompt("You help the staff of a coffee roaster run their store."),
	neui.WithThreadConfig(api.ThreadConfig{CompactionTrigger: 0.7}),
	neui.WithFilesConfig(api.FilesConfig{MaxBytes: 256 << 20}),
	neui.WithIslandConfig(api.IslandConfig{TurnTime: time.Minute}),
)
```

- **System prompt:** your own instructions on top of neui's, like who the users are and the house rules.
- **Thread config:** when long conversations are compacted into a summary, and how much of the context one input or tool result may take.
- **Files config:** the agent's workspace limits, how much it reads at once, and size limits for images and documents.
- **Island config:** limits for island scripts: run time, memory and number of widgets.
- **JS config** (`WithJSConfig`): limits for the JS module's scripts.

Then adjust it per user when you enable neui. The provider, upload storage, files config, system prompt and modules can all differ per user; a user's system prompt and modules add to the app's:

```go
// app.gox
err := neui.Enable(ctx,
	neui.OptionProvider(largerModel),
	neui.OptionSystemPrompt("The user is a store manager."),
	neui.OptionModules(adminTools),
	neui.OptionFilesConfig(api.FilesConfig{MaxBytes: 1 << 30}),
)
```

> Every field has a default, so set only what you need. Each field's godoc says what it limits.
