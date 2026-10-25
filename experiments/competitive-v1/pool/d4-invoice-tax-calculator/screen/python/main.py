#!/usr/bin/env python3
import decimal
import json
import math
import os
import sys

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def bankers_round(val):
    d = decimal.Decimal(str(val))
    return int(d.quantize(decimal.Decimal('1'), rounding=decimal.ROUND_HALF_EVEN))

def main():
    if len(sys.argv) != 4:
        fail("Usage: ./run <tax_config.json> <invoice.json> <receipt.json>", 2)

    cfg_path = sys.argv[1]
    inv_path = sys.argv[2]
    rec_path = sys.argv[3]

    try:
        with open(cfg_path, "r", encoding="utf-8") as f:
            cfg = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse tax config JSON: {e}", 2)

    try:
        with open(inv_path, "r", encoding="utf-8") as f:
            inv = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse invoice JSON: {e}", 2)

    if not isinstance(cfg, dict):
        fail("Tax config must be an object", 2)
    if not isinstance(inv, dict) or "items" not in inv or not isinstance(inv["items"], list):
        fail("Invoice must contain 'items' array", 2)

    # Categories
    categories = {}
    if "categories" in cfg:
        if not isinstance(cfg["categories"], dict):
            fail("'categories' in tax config must be an object", 2)
        for cat, rate in cfg["categories"].items():
            if not isinstance(rate, (int, float)) or rate < 0:
                fail(f"Invalid rate for category '{cat}'", 2)
            categories[cat] = float(rate)
    elif "flat_tax_rate" in cfg:
        rate = cfg["flat_tax_rate"]
        if not isinstance(rate, (int, float)) or rate < 0:
            fail("Invalid flat_tax_rate", 2)
        categories["standard"] = float(rate)
    else:
        fail("Tax config missing 'categories' or 'flat_tax_rate'", 2)

    luxury_surcharge = cfg.get("luxury_surcharge")
    surcharge_thresh = None
    surcharge_rate = 0.0
    if luxury_surcharge is not None:
        if not isinstance(luxury_surcharge, dict):
            fail("'luxury_surcharge' must be an object", 2)
        surcharge_thresh = luxury_surcharge.get("threshold_cents", 0)
        surcharge_rate = float(luxury_surcharge.get("rate", 0.0))

    # Validate items
    raw_items = inv["items"]
    items = []
    for it in raw_items:
        if not isinstance(it, dict):
            fail("Invoice item must be an object", 2)
        iid = it.get("id")
        price = it.get("unit_price_cents")
        qty = it.get("quantity")
        cat = it.get("category", "standard")

        if not isinstance(iid, str) or not iid.strip():
            fail("Item missing valid 'id'", 2)
        if not isinstance(price, int) or price < 0:
            fail(f"Item '{iid}' has invalid unit_price_cents", 2)
        if not isinstance(qty, int) or qty < 0:
            fail(f"Item '{iid}' has invalid quantity", 2)
        if cat not in categories:
            fail(f"Item '{iid}' has unknown category '{cat}'", 2)

        items.append({
            "id": iid,
            "description": it.get("description", ""),
            "unit_price_cents": price,
            "quantity": qty,
            "category": cat,
            "gross_cents": price * qty
        })

    gross_subtotal = sum(it["gross_cents"] for it in items)
    requested_discount = inv.get("discount_cents", 0)
    if not isinstance(requested_discount, int) or requested_discount < 0:
        fail("Invalid discount_cents", 2)
    discount_cents = min(requested_discount, gross_subtotal)

    # Distribute discount using largest remainder method
    non_exempt = [it for it in items if it["category"] != "exempt"]
    eligible_items = non_exempt if len(non_exempt) > 0 else items
    eligible_gross = sum(it["gross_cents"] for it in eligible_items)

    item_discounts = {it["id"]: 0 for it in items}

    if discount_cents > 0 and eligible_gross > 0:
        allocated = 0
        remainders = []
        for it in eligible_items:
            exact = discount_cents * (it["gross_cents"] / eligible_gross)
            base_d = math.floor(exact)
            rem = exact - base_d
            item_discounts[it["id"]] = base_d
            allocated += base_d
            remainders.append((rem, it["id"]))

        leftover = discount_cents - allocated
        # Sort by remainder descending, tie-break by id ascending
        remainders.sort(key=lambda x: (-x[0], x[1]))
        for i in range(leftover):
            item_discounts[remainders[i][1]] += 1

    receipt_items = []
    total_base_tax = 0
    total_surcharge = 0

    for it in items:
        iid = it["id"]
        gross = it["gross_cents"]
        d = item_discounts[iid]
        net = gross - d
        cat = it["category"]
        rate = categories[cat]

        # Base tax
        base_tax_exact = net * rate
        base_tax = bankers_round(base_tax_exact)

        # Luxury surcharge
        surcharge = 0
        if surcharge_thresh is not None and it["unit_price_cents"] > surcharge_thresh and cat != "exempt":
            sur_base_per_unit = it["unit_price_cents"] - surcharge_thresh
            if it["unit_price_cents"] > 0:
                sur_ratio = sur_base_per_unit / float(it["unit_price_cents"])
                net_sur_base = net * sur_ratio
                sur_exact = net_sur_base * surcharge_rate
                surcharge = bankers_round(sur_exact)

        total_item_tax = base_tax + surcharge
        total_item_cents = net + total_item_tax

        total_base_tax += base_tax
        total_surcharge += surcharge

        receipt_items.append({
            "id": iid,
            "gross_cents": gross,
            "discount_cents": d,
            "net_cents": net,
            "base_tax_cents": base_tax,
            "surcharge_cents": surcharge,
            "total_tax_cents": total_item_tax,
            "total_cents": total_item_cents
        })

    net_subtotal = gross_subtotal - discount_cents
    total_tax = total_base_tax + total_surcharge
    grand_total = net_subtotal + total_tax

    receipt = {
        "gross_subtotal_cents": gross_subtotal,
        "discount_cents": discount_cents,
        "net_subtotal_cents": net_subtotal,
        "total_base_tax_cents": total_base_tax,
        "total_surcharge_cents": total_surcharge,
        "total_tax_cents": total_tax,
        "grand_total_cents": grand_total,
        "items": receipt_items
    }

    try:
        with open(rec_path, "w", encoding="utf-8") as f:
            json.dump(receipt, f, indent=2)
    except Exception as e:
        fail(f"Failed to write receipt: {e}", 2)

    sys.exit(0)

if __name__ == "__main__":
    main()
