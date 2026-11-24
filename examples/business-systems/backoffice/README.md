# Back office example

Kettle & Burr, a small coffee gear shop: dashboard, orders, customers and products, with the neui chat next to the pages. Data lives in memory and resets on restart.

Run it from the repo root with the API key of the provider you want; the key you set picks the model:

```sh
DEEPSEEK_API_KEY=sk-... go run ./examples/business-systems/backoffice      # DeepSeek Flash
OPENAI_API_KEY=sk-... go run ./examples/business-systems/backoffice        # OpenAI gpt-6.1-terra
ANTHROPIC_API_KEY=sk-ant-... go run ./examples/business-systems/backoffice # Claude Haiku 5.5
GEMINI_API_KEY=... go run ./examples/business-systems/backoffice           # Gemini 4 Hydrogen
GENERIC_URL=http://127.0.0.1:8080/v1 GENERIC_MODEL=qwen3.8-27b go run ./examples/business-systems/backoffice # any OpenAI-compatible server
GENERIC_URL=https://openrouter.ai/api/v1 GENERIC_KEY=sk-or-... GENERIC_MODEL=qwen/qwen3.8-27b go run ./examples/business-systems/backoffice # OpenRouter
```

`GENERIC_KEY` sets a key for a server or gateway that wants one, `GENERIC_CONTEXT` the model's window in tokens. When several are set, `GENERIC_URL` is used first, then DeepSeek, then OpenAI, then Claude, then Gemini.

`GENERIC_BODY` holds a JSON object whose fields go into every request body, for server or gateway settings such as `{"chat_template_kwargs":{"enable_thinking":false}}`.

`GENERIC_MAX_TOKENS` caps each reply at that many tokens, thinking included; unset, the server's own limit applies.

`HEADERS` holds a JSON object of headers added to every request, whichever provider runs, such as OpenRouter's app attribution `{"HTTP-Referer":"https://your.app","X-OpenRouter-Title":"Your app"}`.

A `GENERIC_URL` model reads text only: images and PDFs reach it as short notes, and text documents as their text.

Google's free tier uses your prompts and the model's answers to improve its products (https://aistudio.google.com/docs/pricing); use a paid key for anything private.

Open http://localhost:8095 (set `PORT` to change it). Threads are stored in `neui.db` in the working directory.

## Models & tiers

`TIER` (`neui.WithTier`) sets what the agent may do; pick the highest tier your model handles well:

- `developer`, the default: works the pages, writes scripts, and builds small apps and reports in the Workbench.
- `agent`: works the pages and writes scripts. The shop's data and operations are an `api` space its scripts call; there is no Workbench.
- `assistant`: works the pages and calls tools one at a time, the `api` space included. No scripts, smaller files and xls tool sets, shorter tool descriptions.

To compare models, keep one OpenRouter key and change only the model and the tier; set `GENERIC_CONTEXT` to the window on the model's OpenRouter page:

```sh
export GENERIC_URL=https://openrouter.ai/api/v1 GENERIC_KEY=sk-or-...
GENERIC_MODEL=qwen/qwen3.8-27b GENERIC_CONTEXT=262144 TIER=developer go run ./examples/business-systems/backoffice
GENERIC_MODEL=qwen/qwen3.6-35b-a3b GENERIC_CONTEXT=262144 TIER=agent go run ./examples/business-systems/backoffice
```

`GENERIC_BODY` picks OpenRouter's upstream for a model, such as `{"provider":{"order":["parasail"]}}`.

A small model can run on your own machine. For example, Nanbeige4.2-3B at the assistant tier on a llama.cpp server, with `GENERIC_CONTEXT` set to the server's `-c`:

```sh
llama-server -m Nanbeige4.2-3B-Q8_0.gguf -c 65536
GENERIC_URL=http://127.0.0.1:8080/v1 GENERIC_CONTEXT=65536 TIER=assistant go run ./examples/business-systems/backoffice
```

If a server refuses the tool schemas (LM Studio rejects `$ref`), add `SIMPLE_SCHEMAS=1` (sets `SimpleSchemas` on the generic provider's `api.Model`): references are inlined and every object gets `properties`.

Each tier's prompts are below.

## Development

After changing a `.gox` file run `gox gen ./examples/business-systems/backoffice`; after changing classes run `examples/business-systems/backoffice/css/build.sh` (Tailwind CSS v4 CLI) and commit `css/output.css`.

## Try it

Open the chat with the button in the corner and try these at any tier:

- On Orders: "show unpaid orders from the last two weeks, biggest first". Your table's filters and sort change; the address bar follows, so Back and reload keep them.
- Anywhere: "open New order and fill it for Grace Hopper: 2 travel mugs and a cold brew jar, already paid". The form fills in front of you and is submitted; a quantity over stock shows its error and gets fixed.
- "Open order 1037 and fix its shipping address in the edit form with the customer's address on file".
- "What is running low on stock? Give me the list as a spreadsheet."
- On Customers: "add Hedy Lamarr from Spread Spectrum Inc, hedy@spread.example, 1 Frequency Lane, Vienna, in the form". Leave out the email and the form refuses it.
- "Delete Ken Thompson". He has orders, so he can't be deleted. Deleting customers and products is always your own click.

Changes by id from any page, at any tier; the open page follows and a toast says what changed:

- "Create an order for Ada Lovelace: 3 espresso cups and a digital scale, paid, and show it to me".
- "Mark order 1037 paid", "add 2 paper filters to it", "take the burr grinder off" or "ship it to 10 Downing Street, London SW1A 2AA".
- "Go through Grace Hopper's orders and set every travel mug quantity to 1", without leaving the page.
- "Refund order 1038" or "cancel order 1040".
- "Remind the customer of order 1037 to pay, and give them two more weeks". The order page shows the reminder and the new due date.
- "We received 12 pour-over kettles" or "raise the price of the scale to 42".
- "Grace moved to 1 Navy Yard, Washington, DC 20374".

With `TIER=agent` and `TIER=developer` the agent also writes scripts, so bulk work runs in one go instead of one call at a time:

- "Raise every product's price by 5%, rounded to end in .99".
- "Remind every customer with an unpaid order older than a week, and give each of them a week more".
- "Make me a spreadsheet of all orders: customer, date, status and total, with a sheet that totals each customer".
- "Which products sold nothing in the last 30 days? Cut their prices by 10%".

With `TIER=developer` it also builds small apps. The Workbench is a panel over the page; it appears while an app runs and goes away when the app ends:

- "Build me a sales dashboard."
- "I need to see all orders with the cold brew jar and adjust the quantities."
- "Go through the unpaid orders older than a week one by one. For each, show the order and the customer's history, and let me choose: remind, extend, or cancel."
- "Plan a restock: show what's running low, how fast each product sold over the last 30 days, and suggest how many to reorder. Let me adjust the numbers, then give me the purchase list as a spreadsheet."
