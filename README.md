# Neui

**The runtime for agent-driven web apps.**

Neui is an AI agent and a web framework built as one stack. The framework, [Doors](https://github.com/doors-dev/doors) (Apache-2.0), lets Go teams build modern interactive apps with no separate frontend and no API layer. Neui adds the agent: it runs inside the user's session, sees the parts of the page you let it see, operates the UI like a user, and brings its own work environment for the jobs a chat alone cannot do.

Every feature you build for users is instantly usable by the agent, with a work environment powerful enough to automate the work your users do by hand.

## Contents

- [Why Neui](#why-neui)
- [What you get](#what-you-get)
- [How it works](#how-it-works)
- [Compared with CopilotKit and Vercel AI SDK](#compared-with-copilotkit-and-vercel-ai-sdk)
- [Use cases](#use-cases)
- [Getting started](#getting-started)

## Why Neui

Adding an agent to a web app today means building a second product beside the first one. Neui exists because of these problems:

- **In-app agents can't use the app.** Teams re-implement each UI action as a tool and mirror state by hand. The agent drifts from the screen, and every action needs its own authorization.
- **Copilots can't automate real work.** They act one tool call at a time, with no workspace or scripts. Bulk edits and multi-step tasks turn into dozens of model calls, and a sandbox added on the side must be wired to the app separately.
- **Generated screens force a trade-off.** Either the model sits behind every click, so each action is a model call and your data passes through it, or it writes a separate web page outside your design with its own API to your data.
- **Copilots are expensive to run.** Their prompts change with every screen update, so the AI provider's cache stops working and input costs 6 to 14 times more.
- **Copilots widen the attack surface.** The browser assembles the agent's tools and instructions, so a tampered browser can inject its own.
- **Chat features are a project.** Thread lists, search, rewind and clone are a paid tier or custom work in other copilots.

The usual alternatives are CopilotKit (plus paid Intelligence), Vercel AI SDK with Sandbox, assistant-ui, OpenAI ChatKit, LangGraph with Agent Server, browser agents and WebMCP. For a Go team each of them means bolting on a React island, CopilotKit and a Node runtime: a second stack.

## What you get

### The agent operates your UI

Developers mark which parts of the page the agent may see and use, right in the existing markup. The agent then clicks, types and reads like a user, and the app's own logic runs. There is no per-action code to write and no state to mirror.

### Capability-based tools with reactive permissions

A tool is one Go function. It lives and runs on the server, and its availability is reactive like the UI, with no endpoint or permission code to write. Developers decide what the agent can see and do right where they build the UI, so its access is scoped to each user and updates with the screen. There is no second permission system to build, audit or maintain, and that saving grows with every role and screen the product adds.

A new capability is instantly available to the agent, its scripts and the live UI. Elsewhere it must be wired separately into the agent backend, the frontend, an API with auth and a sandbox.

### An agent work environment

The agent gets its own file workspace, scripts that run bulk UI work in one step, spreadsheets, web search, MCP connectors, background tasks, and automatic handling of long conversations. One script can call every tool and work across the whole UI in a single step.

### Agent-built screens without a model call per click

The agent writes a small program from your own components, so the screen stays in your design. The program runs on your server inside the app, handles clicks and data itself with no extra API, and calls the model only when a step needs judgment.

### Chat features, built in

Thread list, full-text search, rewind and clone come built in, on any storage and any model, self-hosted included. Conversations survive reloads. Long conversations are compacted in the background, seamlessly for the user, or on demand.

### 6 to 14 times cheaper to run

The agent's instructions and tools stay fixed and page changes arrive as tool results, so the provider's prompt cache keeps working.

### Nothing sensitive in the browser

Model keys, instructions and tools live only on the server. The browser sends only the new message; history stays on the server. The browser can't add tools or change what the agent is told.

## How it works

Doors is a server-driven framework: the UI lives on the server, and the browser shows what the server renders. That is what makes the rest possible.

- **The screen is the agent's interface.** Because the server holds the live UI, the agent can read it and act on it directly, with the app's own logic running underneath. Developers mark regions, actions, context and elements of what the user sees in the page template, and the agent sees exactly those.
- **Tools are Go functions.** A module serves tools to every thread. Built-in modules cover the file workspace, scripts, spreadsheets, web access, MCP connectors and time; your own tools are ordinary Go functions registered the same way.
- **Permissions follow the screen.** The agent runs inside the signed-in user's session. What a developer draws for a user, and marks for the agent, is what the agent can use, and it updates as the page updates.
- **Agent-built screens are programs from your components.** When the agent needs a screen of its own, it is given a region of the page and composes a small program from your components. The program runs on the server inside the app and handles clicks and data itself.
- **The prompt stays stable.** System prompt and tool list are fixed; page state arrives as tool results. The provider's cache keeps hitting across the conversation.
- **The chat is a widget.** A ready-made chat widget renders on any Doors page and shows threads, search, rewind and clone. Storage and model provider are pluggable; Claude, GPT and DeepSeek providers are included.

## Compared with CopilotKit and Vercel AI SDK

Each row was checked against neui's code and the competitors' current docs and source. Competitor cells describe the default open-source path.

| Capability | Neui | CopilotKit | Vercel AI SDK |
|---|---|---|---|
| Agent operates the UI | **Yes.** Developers mark, right in the UI template, which elements the agent may see and use; no per-action code is needed. | **Partial.** One hand-written frontend tool per action | **No.** Fully hand-wired: each UI action needs a tool on the server and a handler in the browser |
| Agent work environment | **Built in.** Its own files and scripts; one script can call every tool and work across the whole UI in a single step | **No.** One tool call per action; sandbox only through a third-party service; files need own wiring | **Partial.** Code can call your tools, with Claude only; paid cloud sandbox; files need own wiring |
| Agent-built screens | **Agent-written program.** Built from your components, it runs on the server, handles clicks and data itself, and calls the model only when a step needs judgment | **Model per click, or a free-form page.** The model fills your components and each click starts a new agent run. Or it writes its own web page, reaching your data through functions the developer registers | **Model picks a tool, you pre-wire the screen.** The model calls one of your tools and your component shows the result. Clicks run code you wrote in advance; the next agent step is another model request |
| Agent permissions | **Built into the screen.** Developers decide what the agent can see and do where they build the UI; no permission code to write | **Manual.** Developers write checks for each agent action; tools sent by the browser are trusted | **Manual.** Developers write checks for each tool |
| Provider isolation | **Yes.** Keys, prompt and tools are server-only | **Weak.** Keys stay on the server, but the browser sends history, tools, context and state on every run, and the server uses them as sent | **Partial.** Tools declared on the server; chat history sent from the browser |
| Prompt-cache stability | **Yes.** Fixed tools and system prompt; page state arrives in tool results | **No.** The system prompt is rebuilt from app state every run | **Yes.** Server owns tools and prompt; history is only appended |
| Traffic per message | **Tiny.** Only the new message; history stays on the server, minimal delays | **Grows with the conversation.** Full history on every run and every tool call | **Grows with the conversation by default.** Full history on every request and tool call; trimming needs your own server-side store |
| Advanced chat features | **Built in.** Thread list, full-text search, rewind, clone | **Partial.** Thread list needs the paid tier; other features require own wiring and backend support | **No.** No thread list; thread features are yours to build |
| Auto-compaction | **Yes.** Rolling, in the background, seamless for the user, or manual | **No** | **Partial.** Manual recipe in the docs |
| Supported platforms | **One.** Web apps built with Go and Doors | **Many.** React-first web frontends | **Many.** Most JavaScript frameworks |

### Where Neui sits among its neighbours

- **CopilotKit and AG-UI** standardize the bridge between a React app and an outside agent. Neui eliminates the bridge. The agent lives inside the app, so it gets capabilities a bridge can't carry: permissions from rendering, tools with direct data access, scripts across the whole UI, and stable prompts.
- **Phoenix LiveView** is stateful, but ships no framework-level agent integration. Sagents is chat plus agent processes; Tidewave and Phoenix.new are coding agents for developers.
- **Datastar and htmx** are stateless hypermedia, so there is no server-held render tree to scope an agent to.
- **Browser agents and WebMCP** need no annotation, but they are lossy, slow, act with the user's cookies, and are open to prompt injection.

## Use cases

Neui is for Go teams that want React-class web apps with the agent built into the stack. It fits well when:

- **You build a data-heavy, multi-role product** (CRM, ERP, billing, analytics, operations platforms) and want the agent doing real work for your users, inside the screens they already use.
- **You run internal systems with complex permissions and workflows.** In regulated settings (finance, health, government) a self-hosted single binary with no vendor relay is often a requirement, and Neui runs that way, on a self-hosted model if needed.
- **You expose a portal to customers or partners.** Many users, many roles, sensitive data. Because developers scope what the agent can see and do when they build each screen, it is safe to put in front of external users.
- **You need internal tools and admin consoles quickly.** Fast to build, and automation value from day one.
- **You maintain a Go backend with a React frontend.** Every new role or dashboard means the same data in Go, the API and TypeScript. Moving the UI into Doors keeps the Go backend and puts UI and agent in one codebase. A new module can start next to the existing UI.
- **You are starting a new product.** There is nothing to migrate, so you get server, UI and a working agent in one Go codebase from the first commit.

## Getting started

Neui is a Go module (`neui.dev`) that plugs into a Doors app: you configure the agent once with options on the Doors app, turn it on per session once the user is known, and render the chat widget from the `kchat` package on the pages where the user should talk to the agent.

Installation and a runnable walkthrough are being written alongside the public release. Until then, see the package documentation in [`doc.go`](doc.go), [`api/doc.go`](api/doc.go) and [`kchat/doc.go`](kchat/doc.go), and the Doors documentation for building the app itself.
