# Back office example

Kettle & Burr, a small coffee gear shop: dashboard, orders, customers and products, with the neui chat next to the pages. Data lives in memory and resets on restart.

Run it from the repo root:

```sh
DEEPSEEK_API_KEY=sk-... go run ./examples/business-systems/backoffice
```

With `OPENAI_API_KEY` instead, it runs on OpenAI, and with `ANTHROPIC_API_KEY` on Claude; when several are set, the first of DeepSeek, OpenAI and Claude is used.

Open http://localhost:8095 (set `PORT` to change it). Threads are stored in `neui.db` in the working directory.

After changing a `.gox` file run `gox gen ./examples/business-systems/backoffice`; after changing classes run `examples/business-systems/backoffice/css/build.sh` (Tailwind CSS v4 CLI) and commit `css/output.css`.

Open the chat with the button in the corner and try:

- On Orders: "show unpaid orders from the last two weeks, biggest first". Your table's filters and sort change; the address bar follows, so Back and reload keep them.
- Anywhere: "open New order and fill it for Grace Hopper: 2 travel mugs and a cold brew jar, already paid". The form fills in front of you and is submitted; a quantity over stock shows its error and gets fixed.
- "Open order 1037 and fix its shipping address in the edit form with the customer's address on file".
- On the dashboard: "which customers order the most?". A report appears in the Report panel.
- "What is running low on stock?"
- On Customers: "add Hedy Lamarr from Spread Spectrum Inc, hedy@spread.example, 1 Frequency Lane, Vienna, in the form". Leave out the email and the form refuses it.
- "Delete Ken Thompson". He has orders, so he can't be deleted. Deleting customers and products is always your own click.

Changes by id from any page; the open page follows and a toast says what changed:

- "Create an order for Ada Lovelace: 3 espresso cups and a digital scale, paid, and show it to me".
- "Mark order 1037 paid", "add 2 paper filters to it", "take the burr grinder off" or "ship it to 10 Downing Street, London SW1A 2AA".
- "Go through Grace Hopper's orders and set every travel mug quantity to 1", without leaving the page.
- "Refund order 1038" or "cancel order 1040".
- "Remind the customer of order 1037 to pay, and give them two more weeks". The order page shows the reminder and the new due date.
- "We received 12 pour-over kettles" or "raise the price of the scale to 42".
- "Grace moved to 1 Navy Yard, Washington, DC 20374".

The Workbench is a panel over the page where the agent builds small apps; it appears while an app runs and goes away when the app ends:

- "I need to see all orders with the cold brew jar and adjust the quantities."
- "Go through the unpaid orders older than a week one by one. For each, show the order and the customer's history, and let me choose: remind, extend, or cancel."
- "Plan a restock: show what's running low, how fast each product sold over the last 30 days, and suggest how many to reorder. Let me adjust the numbers, then give me the purchase list as a spreadsheet."
