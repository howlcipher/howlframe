# Commodity Matching Engine Order Book

Build an HTTP service and offline replay processor that simulates an exchange order matching engine. The engine matches buy and sell limit orders using price-time priority, handles partial fills, maintains resting order book depth, records trade executions, and persists state to local storage.

## Execution Modes

The executable is located at `./run`. It must support two execution modes:

### 1. HTTP Server Mode
```bash
./run [--store <store_path>] --port <PORT>
```
Starts an HTTP JSON server on `127.0.0.1:<PORT>`.
- `--port <PORT>`: Loopback port. Required in server mode. (Also check `PORT` environment variable).
- `--store <store_path>`: Path to persistence JSON file. Optional; defaults to `store.json`.

### 2. Batch Replay Mode
```bash
./run replay [--store <store_path>] <requests_jsonl>
```
Executes requests sequentially offline through the identical matching engine logic, printing one JSON response line per request to standard output:
```json
{"status": 201, "body": {"order_id": "ord-1", "status": "open"}}
```

## Order and Matching Model

### Orders
- `order_id`: string sequential identifier (`"ord-1"`, `"ord-2"`, ...).
- `symbol`: uppercase commodity symbol string (e.g. `"GOLD"`, `"OIL"`).
- `side`: `"buy"` or `"sell"`.
- `price`: positive number limit price.
- `quantity`: positive number total order size.
- `filled_quantity`: number of units filled so far (starts at `0`).
- `remaining_quantity`: remaining unfilled units (`quantity - filled_quantity`).
- `status`: `"open"`, `"partially_filled"`, `"filled"`, or `"cancelled"`.
- `created_at`: float epoch timestamp.

### Price-Time Matching Logic
When an incoming order arrives:
1. **Buy Order**:
   - Matches against resting sell orders for the same symbol with `resting.price <= buy.price`.
   - Resting sells are evaluated in price-time priority: lowest `price` first; ties broken by earliest `created_at`.
2. **Sell Order**:
   - Matches against resting buy orders for the same symbol with `resting.price >= sell.price`.
   - Resting buys are evaluated in price-time priority: highest `price` first; ties broken by earliest `created_at`.
3. **Execution Price**:
   - A trade always executes at the **resting order's price**.
4. **Execution Trade**:
   - Traded quantity: `match_qty = min(incoming.remaining_quantity, resting.remaining_quantity)`.
   - Update both orders' `filled_quantity` and `remaining_quantity`.
   - If an order's `remaining_quantity == 0`, its status transitions to `"filled"`; otherwise `"partially_filled"`.
   - Generate trade object:
     ```json
     {
       "trade_id": "t-1",
       "symbol": "<symbol>",
       "buy_order_id": "<buy_id>",
       "sell_order_id": "<sell_id>",
       "price": <execution_price>,
       "quantity": <match_qty>,
       "timestamp": <now>
     }
     ```
5. Any remaining unfilled quantity of the incoming order rests in the book with status `"open"` (or `"partially_filled"`).

## HTTP Endpoints

All request and response bodies use JSON (`Content-Type: application/json`).

### 1. `POST /orders`
Submit a limit order.
- Request Body:
  ```json
  {
    "symbol": "GOLD",
    "side": "buy",
    "price": 100.5,
    "quantity": 10
  }
  ```
  Validation: `side` must be `"buy"` or `"sell"`, `price > 0`, `quantity > 0`, `symbol` non-empty string. If invalid, return `400 Bad Request` with `{"error": "invalid_order"}`.
- Response: `201 Created`
  ```json
  {
    "order_id": "ord-1",
    "symbol": "GOLD",
    "side": "buy",
    "price": 100.5,
    "quantity": 10,
    "filled_quantity": 10,
    "remaining_quantity": 0,
    "status": "filled",
    "trades": [
      {
        "trade_id": "t-1",
        "symbol": "GOLD",
        "buy_order_id": "ord-1",
        "sell_order_id": "ord-0",
        "price": 100.0,
        "quantity": 10,
        "timestamp": 1700000000.0
      }
    ]
  }
  ```

### 2. `DELETE /orders/<id>`
Cancel an active resting order.
- If order does not exist: return `404 Not Found`.
- If order status is already `"filled"` or `"cancelled"`: return `400 Bad Request` with `{"error": "order_not_cancellable"}`.
- Transitions status to `"cancelled"`.
- Response: `200 OK`
  ```json
  {"order_id": "ord-1", "status": "cancelled"}
  ```

### 3. `GET /orders/<id>`
Inspect order status and details.
- Returns `200 OK` with order object, or `404 Not Found`.

### 4. `GET /book/<symbol>`
Get current order book aggregated price levels for active resting orders.
- Aggregates remaining quantities by price level.
- `bids`: array of `{"price": P, "quantity": Q}` sorted by `price` descending.
- `asks`: array of `{"price": P, "quantity": Q}` sorted by `price` ascending.
- Response: `200 OK`
  ```json
  {
    "symbol": "GOLD",
    "bids": [{"price": 99.5, "quantity": 15}],
    "asks": [{"price": 101.0, "quantity": 25}]
  }
  ```

### 5. `GET /trades/<symbol>`
List executed trades for a symbol in chronological order.
- Response: `200 OK`
  ```json
  {
    "symbol": "GOLD",
    "trades": [...]
  }
  ```
