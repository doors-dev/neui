# Back office example

Kettle & Burr, a small coffee gear shop: dashboard, orders, customers and products, with the neui chat next to the pages. Data lives in memory and resets on restart.

Run it from the repo root:

```sh
DEEPSEEK_API_KEY=sk-... go run ./examples/business-systems/backoffice
```

Open http://localhost:8095 (set `PORT` to change it). Threads are stored in `neui.db` in the working directory.

After changing a `.gox` file run `gox gen ./examples/business-systems/backoffice`; after changing classes run `examples/business-systems/backoffice/css/build.sh` (Tailwind CSS v4 CLI) and commit `css/output.css`.

Open the chat with the button in the corner and try:

- On Orders: "show unpaid orders from the last two weeks, biggest first". The agent sets the filters and sort of your table; the address bar follows, so Back and reload keep them.
- Anywhere: "create an order for Grace Hopper: 2 travel mugs and a cold brew jar, already paid". The agent opens New order, picks the customer and products, adds a row and presses Create order. If a quantity is over stock, the form shows the error and the agent fixes it.
- "Order 1037 has a broken shipping address, fix it with the customer's address on file". The agent opens the edit form, corrects the address and saves.
- "Refund order 1038". The agent opens the order and presses Refund order; only you can press Confirm refund.
- On the dashboard: "which customers order the most?". The agent builds a report in the Report panel.
- "What is running low on stock?". The agent sorts the products page by stock.
- On Customers: "add Hedy Lamarr from Spread Spectrum Inc, hedy@spread.example, 1 Frequency Lane, Vienna". The agent fills New customer and saves; leave out the email and the form refuses it.
- "Delete Ken Thompson". The edit page says he has orders, so he can't be deleted.

Page tools change the data directly; you see the result on the page and a toast:

- On Orders: "create an order for Ada Lovelace: 3 espresso cups and a digital scale, paid". The agent calls `create_order` and the new order opens.
- On an order page: "mark this order paid", "add 2 paper filters", "take the travel mug off" or "ship it to 10 Downing Street, London SW1A 2AA". The agent calls `set_status`, `add_item`, `remove_item` or `update_shipping_address`; refund and cancel still wait for your Confirm.
- On Products: "we received 12 pour-over kettles" or "raise the price of the scale to 42". The agent calls `adjust_stock` or `update_product` and the table updates.
- On a customer page: "Grace moved to 1 Navy Yard, Washington, DC 20374". The agent calls `update_customer`.

The Workbench is a panel over the page where the agent builds small apps; it appears while an app runs and goes away when the app ends:

- "Compare Ada Lovelace and Grace Hopper side by side": orders, spending and favourite products in two columns.
- "Build me a form to bulk-adjust stock": one row per product, you enter the changes and press a button; the stock changes and the products table follows.
