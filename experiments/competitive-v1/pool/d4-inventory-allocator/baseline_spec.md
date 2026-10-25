# Baseline Specification: Warehouse Inventory Allocator (v1)

`inventory-allocator` is a command-line tool that processes customer orders against warehouse stock levels.

## CLI Invocation

```bash
./run <inventory.json> <orders.json> <manifest.json>
```

## Baseline Input Formats

### `<inventory.json>`
```json
{
  "items": {
    "SKU-A": {"quantity": 10},
    "SKU-B": {"quantity": 5}
  }
}
```

### `<orders.json>`
```json
{
  "orders": [
    {
      "id": "ord-001",
      "items": {
        "SKU-A": 4,
        "SKU-B": 2
      }
    }
  ]
}
```

## Baseline Allocation Rules (v1)

1. Orders are processed in the exact order they appear in `orders.json`.
2. An order is evaluated all-or-nothing:
   - If every requested item in the order has available warehouse quantity >= requested quantity:
     - Deduct the requested quantities from warehouse stock.
     - Record the order status as `"fulfilled"`.
   - If ANY requested item in the order has available warehouse quantity < requested quantity:
     - Deduct nothing from warehouse stock.
     - Record the order status as `"unfulfilled"`.
3. An unknown SKU or negative requested quantity causes validation failure: exit code **2**, error message printed to `stderr`, and `<manifest.json>` is not written.

## Baseline Output Format (`<manifest.json>`)

```json
{
  "total_orders": 1,
  "fulfilled_count": 1,
  "unfulfilled_count": 0,
  "orders": [
    {
      "id": "ord-001",
      "status": "fulfilled",
      "allocated": {
        "SKU-A": 4,
        "SKU-B": 2
      }
    }
  ],
  "remaining_inventory": {
    "SKU-A": 6,
    "SKU-B": 3
  }
}
```

## Exit Codes

- `0`: All orders processed.
- `1`: One or more orders were unfulfilled.
- `2`: Malformed JSON, missing files, or validation errors.
