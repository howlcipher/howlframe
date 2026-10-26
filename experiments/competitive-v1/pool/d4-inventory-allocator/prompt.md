# Change Request: Priority Tiering, Lot Expiration, and Backorder Splitting (v2)

You are updating the warehouse inventory allocator according to `baseline_spec.md`. The business requires priority order handling, expiration-date-aware stock allocation, and partial order fulfillment with backorders.

## Requested Changes

### 1. Order Priority Tiering
Orders in `<orders.json>` now support an optional `"priority"` string field:
- Permitted values: `"VIP"`, `"STANDARD"`, `"BULK"`.
- Default if omitted: `"STANDARD"`.
- Priority order: `"VIP"` orders must be processed first, followed by `"STANDARD"` orders, followed by `"BULK"` orders.
- Within the same priority tier, maintain the original relative order from `<orders.json>`.
- In the final `<manifest.json>` `"orders"` output array, maintain the execution processing order (`VIP` first, then `STANDARD`, then `BULK`).

### 2. Lot Expiration Date Allocation (FIFO by Expiry)
`<inventory.json>` items now track stock across distinct lots:

```json
{
  "items": {
    "SKU-A": {
      "lots": [
        {"lot_id": "L-02", "quantity": 10, "expires": "2026-12-01"},
        {"lot_id": "L-01", "quantity": 5, "expires": "2026-10-15"}
      ]
    }
  }
}
```

- When allocating units of an item, always deduct from the lot with the earliest `expires` date first.
- If two lots have the same `expires` date, sort by `lot_id` ascending.
- **Backwards compatibility**: If an item in `inventory.json` provides `"quantity": N` directly instead of a `"lots"` array, treat it as a single lot: `[{"lot_id": "default", "quantity": N, "expires": "9999-12-31"}]`.

### 3. Partial Fulfillment and Backorder Splitting
Eliminate all-or-nothing rejection:
- For each requested item in an order, allocate as many units as currently available across valid lots (up to the requested amount).
- If available units < requested units:
  - Allocate all available units.
  - Record the unallocated difference under `"backordered"`.
- Order `status`:
  - `"fulfilled"`: Every requested item was fully satisfied (`backordered` sum is 0).
  - `"partially_fulfilled"`: At least one item had units allocated > 0, but one or more items had units backordered > 0.
  - `"backordered"`: Zero units were allocated across all items in the order (all items completely backordered).

### 4. Updated Output Schema (`<manifest.json>`)

```json
{
  "total_orders": 2,
  "fulfilled_count": 1,
  "partially_fulfilled_count": 1,
  "backordered_count": 0,
  "orders": [
    {
      "id": "ord-vip",
      "priority": "VIP",
      "status": "fulfilled",
      "allocations": [
        {
          "sku": "SKU-A",
          "lot_id": "L-01",
          "quantity": 4
        }
      ],
      "backordered": {}
    },
    {
      "id": "ord-std",
      "priority": "STANDARD",
      "status": "partially_fulfilled",
      "allocations": [
        {
          "sku": "SKU-A",
          "lot_id": "L-01",
          "quantity": 1
        },
        {
          "sku": "SKU-A",
          "lot_id": "L-02",
          "quantity": 2
        }
      ],
      "backordered": {
        "SKU-A": 3
      }
    }
  ],
  "remaining_inventory": {
    "SKU-A": {
      "total_quantity": 8,
      "lots": [
        {"lot_id": "L-01", "quantity": 0, "expires": "2026-10-15"},
        {"lot_id": "L-02", "quantity": 8, "expires": "2026-12-01"}
      ]
    }
  }
}
```

Order object fields:
- `id`: Order id.
- `priority`: `"VIP"`, `"STANDARD"`, or `"BULK"`.
- `status`: `"fulfilled"`, `"partially_fulfilled"`, or `"backordered"`.
- `allocations`: List of `{ "sku": string, "lot_id": string, "quantity": int }` objects sorted by `sku` ascending, then `lot_id` ascending. (Omit allocations with quantity 0).
- `backordered`: Map of SKU to integer backordered count (only include SKUs with backordered quantity > 0).

Remaining inventory:
- Keyed by SKU in ascending code-point order.
- `total_quantity`: Sum of remaining quantities across lots.
- `lots`: List of remaining lots sorted by `expires` ascending, then `lot_id` ascending.

## Exit Codes

- `0`: All orders fully fulfilled (`fulfilled_count == total_orders`).
- `1`: One or more orders were partially fulfilled or backordered.
- `2`: Invalid CLI arguments, malformed JSON, invalid priority, unknown SKU, or negative quantities.
