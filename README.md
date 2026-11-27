# Neui: an AI agent built into your Go web app
[![codecov](https://codecov.io/gh/doors-dev/neui/branch/main/graph/badge.svg)](https://codecov.io/gh/doors-dev/neui)
[![Go Reference](https://pkg.go.dev/badge/neui.dev.svg)](https://pkg.go.dev/neui.dev)

<img width="800" height="615" alt="neui-demo-alt-10fps" src="https://github.com/user-attachments/assets/730c7f2d-62b8-416e-9b84-12359689adf3" />

Neui is an AI agent kit and a web framework built as one stack. The framework, [Doors](https://github.com/doors-dev/doors) (Apache-2.0), lets Go teams build modern interactive apps with no separate frontend and no API layer. Neui adds the agent: it runs inside the user's session, sees the parts of the page you let it see, operates the UI like a user, and brings its own work environment for the jobs a chat alone cannot do.

Every feature you build for users is instantly usable by the agent, with a work environment powerful enough to automate the work your users do by hand.

## Contents

- [What you get](#what-you-get)
- [Runtime](#runtime)
- [Agentic Apps](#agentic-apps)
- [Chat](#chat)
- [Work environment](#work-environment)
- [Compared with CopilotKit and Vercel AI SDK](#compared-with-copilotkit-and-vercel-ai-sdk)
- [Getting started](#getting-started)

## What you get

### The agent operates your UI

Developers mark which parts of the page the agent may see and use, right in the existing markup. The agent then clicks, types and reads like a user, and the app's own logic runs. There is no per-action code to write and no state to mirror.

### Capability-based tools with reactive permissions

A tool is one Go function that runs on the server, with no endpoint or permission code to write. Developers decide what the agent can see and do right where they build the UI, so its access is scoped to each user and updates with the screen.

### An agent work environment

The agent gets its own file workspace, scripts that run bulk UI work in one step, spreadsheets, web search, MCP connectors, background tasks, and automatic handling of long conversations. One script can call every tool and work across the whole UI in a single step.

### Agentic apps

When a user needs functionality your UI doesn't have, the agent builds it on the spot, anything from a single screen to a specialized app. It runs in the page with the user's permissions, and the agent stays in the loop to make the decisions.

### Fully-featured chat

Your users get the chat experience they already know from chat apps, with nothing to build or purchase.

### Cheaper to run

The conversation is prompt-cache friendly, so input costs an estimated 6 to 14 times less than in copilots that rebuild the prompt for every screen, depending on the provider's cache pricing. Built-in scripts do bulk work without extra model turns or context bloat.

### The browser can't steer the agent

The conversation, instructions and tools live on the server, and the browser sends only the user's new message. A tampered browser can't add tools, rewrite history or change what the agent is told.

## Runtime

Every open page runs on the server, integrated with the agent harness. You mark in the page template what the agent may see and use.

**Works the real UI.** The agent reads, fills and clicks the elements you mark, as the user would, and your handlers run as usual. There's no state to mirror and no action to re-implement.

```gox
~>neui.Use{
    Desc: "Customer's email",
    // RW: the agent reads the value in the user's browser or writes its own.
    Rights: neui.RW,
} <input type="email" name="email">
```

**Tools in the template.** Register agent tools right in the page template, based on the UI context and the user's permissions.

```gox
~(if user.IsAdmin() {
    ~neui.Tool[Refund, Receipt]{
        Name: "refund",
        Desc: "Refund the order",
        Handler: orders.Refund,
    }
})
```

**Persistent tools.** Go functions registered for the whole app or for one session are available in every chat. The built-in work environment works the same way.

```gox
crm := neui.NewModule("crm", "Customer records of the signed-in user")
crm.Tool("find", "Find a customer by name or email", customers.Find)
```

**Page context.** Attach data to a part of the page to tell the agent what it shows, such as the records in a view or hints on how to use it.

```gox
~neui.Data{
    Text: "Open orders of this customer",
    Object: orders,
}
```

**App events.** Your app can tell the agent when something happens, such as a payment arriving. An idle agent reacts right away, and a busy one picks it up on its next turn.

```gox
neui.Instance(ctx).Emit(ctx, "payment_received", map[string]string{"invoice": id}, nil)
```

**Stays in sync without token waste.** The framework tracks tools, data and elements as the page changes, and the agent reads them only when it needs to.

## Agentic Apps

An agentic app is a program the agent writes and runs for the user in an island, a region of the page. You declare the island in the page template and give it the components and Go functions its apps may use.

```gox
~neui.Island{
	Name:     "workbench",
	Desc:     "Apps for the user's invoice work",
	Template: func(slot gox.Comp) gox.Elem { return <section>~(slot)</section> },
	Library: func(l neui.Library) error {
		l.Tool("overdue", "Overdue invoices, one page at a time", invoices.Overdue)
		l.Tool("remind", "Send a payment reminder", invoices.Remind)
		return nekit.IslandRegisterAll(l)
	},
}
```

- **The agent writes a program.** It builds an app for the user's request from your component library and the tools you declare, and runs it right away.
- **The model is called only for decisions.** The app handles user actions itself and calls the agent only when a decision is needed. The UI stays responsive, no tokens are spent on routine steps, and the agent is still in the loop.
- **Data stays out of the context.** The agent works with data through the app's script, without loading it into its context. Data costs no tokens, and text inside it can't steer the agent.
- **App changes on the fly.** The user can ask for an extra column, another step or a different check, and the agent updates the running app without losing their work.
- **Runs with the user's permissions.** The app is sandboxed and can do only what the signed-in user can, through the components and functions you declared, with no endpoints to build.

**Widgets included.** `neui.dev/nekit` includes a general-purpose widget set built to cover any business flow, from data entry to review and reporting. It takes your theme from CSS variables, and your own components work alongside it.

- **Layout.** Columns, rows and grids that compose widgets into typical layouts.
- **Form.** Declarative forms for any purpose: every field type, groups, repeatable lists and multi-step flows, with live validation.
- **Table.** A full-featured data table with paging, sorting, search, row selection and row actions.
- **Markdown cards.** A universal output block for text, tables, images and file downloads, plain or as a card, info, success, warning or danger.

**Compared with declarative generative UI.** In A2UI and similar formats the model describes the screen, and every user action is another model turn. That suits a one-off screen such as a confirmation. An agentic app is cheaper and faster as soon as the user does more than a few things.

## Chat

Neui ships a complete chat experience out of the box. Use it as is, configure its theme and features, or fork it to build something of your own.

- **Controls users already know.** Stop a run, edit and resend a message, queue the next one while the agent works, or duplicate a thread to try another path.
- **Persistence out of the box.** History, the thread list and search are built in. Use the included storages or plug in your own.
- **Runs on the server.** The conversation, the running answer and the agent's tools live on the server, and the browser only displays them. Nothing sensitive reaches the client, and a reload or a second tab picks up where the user left off.
- **Files both ways.** Users attach files to a message, and the agent returns finished files as downloads in its answer.
- **Long conversations keep working.** Older turns are summarized in the background when the context fills up, without interrupting the user.
- **Task management and more.** Background jobs show in the chat and can be cancelled, the agent can ask questions with ready answers, and your app can notify the agent when something happens.

## Work environment

Besides the chat and the page, the agent has a workspace on the server for real work.

- **Filesystem.** Each chat gets its own sandboxed filesystem, where the agent reads uploads, unpacks archives and builds new files for the user to download.
- **Scripts for bulk work.** The agent writes one script that calls its tools and works across the page, so updating 300 invoices takes a single model call.
- **Spreadsheets.** The agent reads large .xlsx files and turns raw exports into finished reports with totals, charts and highlighting.
- **Web and connectors.** The agent reads web pages, searches through a service you connect, and uses the tools of your MCP servers.
- **Background tasks.** Long operations run in the background, and the agent picks up each result when it arrives, so the user keeps chatting.
- **Contained and configurable.** Resources are capped, and you choose which tools each app or user gets.

## Compared with CopilotKit and Vercel AI SDK

The usual alternatives are CopilotKit (plus paid Intelligence), Vercel AI SDK with Sandbox, assistant-ui, OpenAI ChatKit, LangGraph with Agent Server, browser agents and WebMCP. For a Go team each of them means bolting on a JavaScript frontend and a Node runtime: a second stack.

Each row was checked against neui's code and the competitors' current docs and source. Competitor cells describe the default open-source path.

| Capability | Neui | CopilotKit | Vercel AI SDK |
|---|---|---|---|
| Agent operates the UI | **Yes.** Developers mark, right in the UI template, which elements the agent may see and use; no per-action code is needed. | **Partial.** One hand-written frontend tool per action | **No.** Fully hand-wired: each UI action needs a tool on the server and a handler in the browser |
| Agent work environment | **Built in.** Its own files and scripts; one script can call every tool and work across the whole UI in a single step | **No.** One tool call per action; sandbox only through a third-party service; files need own wiring | **Partial.** Code can call your tools, with Claude only; paid cloud sandbox; files need own wiring |
| Agentic apps | **Agent-written app.** Built from your components, it runs on the server, handles clicks and data itself, and calls the model only when a step needs judgment | **Model per click, or a free-form page.** The model fills your components and each click starts a new agent run. Or it writes its own web page, reaching your data through functions the developer registers | **Model picks a tool, you pre-wire the screen.** The model calls one of your tools and your component shows the result. Clicks run code you wrote in advance; the next agent step is another model request |
| Agent permissions | **Built into the screen.** Developers decide what the agent can see and do where they build the UI; no permission code to write | **Manual.** Developers write checks for each agent action | **Manual.** Developers write checks for each tool |
| Provider isolation | **Yes.** Keys, prompt and tools are server-only | **Weak.** Keys stay on the server, but the browser sends history, tools, context and state on every run, and the server uses them as sent | **Partial.** Tools declared on the server; chat history sent from the browser |
| Prompt-cache stability | **Yes.** Fixed tools and system prompt; page state arrives in tool results | **No.** The system prompt and the tool list change with app state | **Yes.** Server owns tools and prompt; history is only appended |
| Traffic per message | **Tiny.** Only the new message; history stays on the server, minimal delays | **Grows with the conversation.** Full history on every run and every tool call | **Grows with the conversation by default.** Full history on every request and tool call; trimming needs your own server-side store |
| Advanced chat features | **Built in.** Thread list, full-text search, rewind, clone | **Partial.** Thread list needs Intelligence with limited free tier; other features require own wiring and backend support | **No.** No thread list; thread features are yours to build |
| Auto-compaction | **Yes.** Rolling, in the background, seamless for the user, or manual, and recovers when the history outgrows the model's window, for example after a switch to a smaller model | **No** | **Partial.** Manual recipe in the docs |
| Supported platforms | **One.** Web apps built with Go and Doors | **Many.** React, Vue and Angular web frontends | **Many.** Most JavaScript frameworks |

## Getting started

```sh
go get neui.dev
```

Neui is a Go module (`neui.dev`) that plugs into a Doors app: you configure the agent once with options on the Doors app, turn it on per session once the user is known, and render `nekit.Chat` on the pages where the user should talk to the agent.

Claude, GPT, Gemini and DeepSeek providers are included, plus a generic one for any server with an OpenAI-compatible Chat Completions API, such as Ollama, vLLM or OpenRouter. SQLite thread storage and disk or S3 upload storage are included too.

Start with the [quick start](docs/00-quick-start.md) and the [back office example](examples/business-systems/backoffice), and see the package documentation in [`doc.go`](doc.go), [`api/doc.go`](api/doc.go) and [`nekit/doc.go`](nekit/doc.go), and the Doors documentation for building the app itself.
