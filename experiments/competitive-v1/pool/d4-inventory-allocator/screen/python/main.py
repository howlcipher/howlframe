#!/usr/bin/env python3
import json
import os
import sys

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    if len(sys.argv) != 4:
        fail("Usage: ./run <inventory.json> <orders.json> <manifest.json>", 2)

    inv_path = sys.argv[1]
    ord_path = sys.argv[2]
    man_path = sys.argv[3]

    try:
        with open(inv_path, "r", encoding="utf-8") as f:
            inv_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse inventory JSON: {e}", 2)

    try:
        with open(ord_path, "r", encoding="utf-8") as f:
            ord_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse orders JSON: {e}", 2)

    if not isinstance(inv_data, dict) or "items" not in inv_data or not isinstance(inv_data["items"], dict):
        fail("Inventory JSON must contain 'items' object", 2)

    if not isinstance(ord_data, dict) or "orders" not in ord_data or not isinstance(ord_data["orders"], list):
        fail("Orders JSON must contain 'orders' array", 2)

    # Normalize inventory items into lots
    # sku -> list of {"lot_id": str, "quantity": int, "expires": str}
    stock = {}
    for sku, item_info in inv_data["items"].items():
        if not isinstance(item_info, dict):
            fail(f"Item '{sku}' info must be an object", 2)
        lots = []
        if "lots" in item_info:
            if not isinstance(item_info["lots"], list):
                fail(f"Item '{sku}' 'lots' must be a list", 2)
            for l in item_info["lots"]:
                if not isinstance(l, dict) or "lot_id" not in l or "quantity" not in l:
                    fail(f"Invalid lot in item '{sku}'", 2)
                qty = l["quantity"]
                if not isinstance(qty, int) or qty < 0:
                    fail(f"Invalid quantity in lot for item '{sku}'", 2)
                lots.append({
                    "lot_id": str(l["lot_id"]),
                    "quantity": qty,
                    "expires": str(l.get("expires", "9999-12-31"))
                })
        elif "quantity" in item_info:
            qty = item_info["quantity"]
            if not isinstance(qty, int) or qty < 0:
                fail(f"Invalid quantity in item '{sku}'", 2)
            lots.append({
                "lot_id": "default",
                "quantity": qty,
                "expires": "9999-12-31"
            })
        else:
            fail(f"Item '{sku}' missing 'lots' or 'quantity'", 2)

        # Sort lots by expires ascending, then lot_id ascending
        lots.sort(key=lambda x: (x["expires"], x["lot_id"]))
        stock[sku] = lots

    # Validate and categorize orders
    priority_order = {"VIP": 1, "STANDARD": 2, "BULK": 3}
    validated_orders = []

    for idx, o in enumerate(ord_data["orders"]):
        if not isinstance(o, dict):
            fail("Order must be an object", 2)
        oid = o.get("id")
        if not isinstance(oid, str) or not oid.strip():
            fail("Order missing valid 'id'", 2)

        pri = o.get("priority", "STANDARD")
        if pri not in priority_order:
            fail(f"Order '{oid}' has invalid priority: {pri}", 2)

        items = o.get("items")
        if not isinstance(items, dict) or len(items) == 0:
            fail(f"Order '{oid}' missing valid 'items' map", 2)

        for sku, q in items.items():
            if sku not in stock:
                fail(f"Order '{oid}' requested unknown SKU: {sku}", 2)
            if not isinstance(q, int) or q < 0:
                fail(f"Order '{oid}' has invalid quantity for SKU: {sku}", 2)

        validated_orders.append((priority_order[pri], idx, o, pri))

    # Sort orders: priority tier ascending, original index ascending
    validated_orders.sort(key=lambda x: (x[0], x[1]))

    manifest_orders = []
    fulfilled_count = 0
    partially_fulfilled_count = 0
    backordered_count = 0

    for _, _, o, pri in validated_orders:
        oid = o["id"]
        allocations = []
        backordered = {}

        total_requested_units = 0
        total_allocated_units = 0

        # Process each requested SKU
        for sku, req_qty in sorted(o["items"].items()):
            total_requested_units += req_qty
            needed = req_qty
            sku_lots = stock[sku]

            for lot in sku_lots:
                if needed == 0:
                    break
                if lot["quantity"] > 0:
                    take = min(lot["quantity"], needed)
                    lot["quantity"] -= take
                    needed -= take
                    total_allocated_units += take
                    allocations.append({
                        "sku": sku,
                        "lot_id": lot["lot_id"],
                        "quantity": take
                    })

            if needed > 0:
                backordered[sku] = needed

        # Determine status
        if total_allocated_units == total_requested_units:
            status = "fulfilled"
            fulfilled_count += 1
        elif total_allocated_units > 0:
            status = "partially_fulfilled"
            partially_fulfilled_count += 1
        else:
            status = "backordered"
            backordered_count += 1

        # Sort allocations by sku ascending, lot_id ascending
        allocations.sort(key=lambda a: (a["sku"], a["lot_id"]))

        manifest_orders.append({
            "id": oid,
            "priority": pri,
            "status": status,
            "allocations": allocations,
            "backordered": backordered
        })

    # Prepare remaining inventory
    remaining_inventory = {}
    for sku in sorted(stock.keys()):
        lots = stock[sku]
        lots_sorted = sorted(lots, key=lambda l: (l["expires"], l["lot_id"]))
        tot_qty = sum(l["quantity"] for l in lots_sorted)
        remaining_inventory[sku] = {
            "total_quantity": tot_qty,
            "lots": [
                {"lot_id": l["lot_id"], "quantity": l["quantity"], "expires": l["expires"]}
                for l in lots_sorted
            ]
        }

    report = {
        "total_orders": len(manifest_orders),
        "fulfilled_count": fulfilled_count,
        "partially_fulfilled_count": partially_fulfilled_count,
        "backordered_count": backordered_count,
        "orders": manifest_orders,
        "remaining_inventory": remaining_inventory
    }

    try:
        with open(man_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write manifest: {e}", 2)

    sys.exit(0 if fulfilled_count == len(manifest_orders) else 1)

if __name__ == "__main__":
    main()
