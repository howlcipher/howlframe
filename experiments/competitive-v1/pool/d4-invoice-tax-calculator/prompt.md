# Change Request: Multi-Category Tax, Luxury Surcharges, and Penny Balancing (v2)

You are updating `invoice-calculator` according to `baseline_spec.md`. The billing team requires multi-jurisdiction category tax rates, luxury item threshold surcharges, proportional discount distribution, and balanced integer-cent banker's rounding.

## Updated Tax Configuration (`<tax_config.json>`)

```json
{
  "categories": {
    "standard": 0.10,
    "reduced": 0.05,
    "exempt": 0.00
  },
  "luxury_surcharge": {
    "threshold_cents": 10000,
    "rate": 0.05
  }
}
```

- `categories` (object, required): Map of category names to decimal tax rate (e.g. 0.10 for 10%).
- `luxury_surcharge` (object, optional):
  - `threshold_cents` (integer): Per-unit price threshold above which surcharge applies.
  - `rate` (decimal): Surcharge rate applied strictly to the amount above `threshold_cents`.
- **Backwards compatibility**: If `tax_config.json` provides `"flat_tax_rate": R` instead of `"categories"`, treat it as `{"categories": {"standard": R}}`.

## Updated Invoice (`<invoice.json>`)

```json
{
  "items": [
    {
      "id": "item-1",
      "description": "Premium Watch",
      "category": "standard",
      "unit_price_cents": 15000,
      "quantity": 1
    },
    {
      "id": "item-2",
      "description": "Book",
      "category": "reduced",
      "unit_price_cents": 2000,
      "quantity": 1
    }
  ],
  "discount_cents": 500
}
```

- Each item may specify `"category"` (string, optional, defaults to `"standard"`).
- Category must exist in `tax_config.categories`. If unknown category, exit code **2**.

## Computation Rules (v2)

1. **Item Gross**:
   - `gross_cents = unit_price_cents * quantity`.
   - `gross_subtotal_cents = sum(item.gross_cents)`.
2. **Discount Distribution**:
   - Total `discount_cents` cannot exceed `gross_subtotal_cents`. If it does, cap discount at `gross_subtotal_cents`.
   - Distribute `discount_cents` across non-exempt items (`category != "exempt"`) proportionally to their `gross_cents` using largest remainder method:
     - If all items are exempt, distribute across all items proportionally.
     - For each eligible item: `exact_share = discount_cents * (item.gross_cents / eligible_gross)`.
     - Assign `floor(exact_share)` integer cents to each item.
     - Distribute remaining unallocated cents (1 cent each) to items with largest fractional remainders, breaking ties by `id` ascending.
     - `item.net_cents = item.gross_cents - item.discount_cents`.
3. **Item Taxes**:
   - **Base Tax**:
     - `base_tax_exact = item.net_cents * category_rate`.
     - Apply **Banker's Rounding** (round half to even) to `base_tax_exact` to get `base_tax_cents`:
       - If fraction is exactly 0.5, round to the nearest even integer (e.g. 2.5 -> 2, 3.5 -> 4).
   - **Luxury Surcharge**:
     - Applies only if `luxury_surcharge` is configured and `unit_price_cents > threshold_cents` and `category != "exempt"`.
     - Surcharge base per unit = `unit_price_cents - threshold_cents`.
     - Surcharge ratio = `surcharge_base / unit_price_cents`.
     - Net surcharge base for item = `item.net_cents * surcharge_ratio`.
     - `surcharge_exact = net_surcharge_base * luxury_surcharge.rate`.
     - Round `surcharge_exact` using banker's rounding to `surcharge_cents`.
   - `item.total_tax_cents = item.base_tax_cents + item.surcharge_cents`.
   - `item.total_cents = item.net_cents + item.total_tax_cents`.
4. **Totals**:
   - `net_subtotal_cents = gross_subtotal_cents - discount_cents`.
   - `total_base_tax_cents = sum(item.base_tax_cents)`.
   - `total_surcharge_cents = sum(item.surcharge_cents)`.
   - `total_tax_cents = total_base_tax_cents + total_surcharge_cents`.
   - `grand_total_cents = net_subtotal_cents + total_tax_cents`.

## Updated Receipt Output Format (`<receipt.json>`)

```json
{
  "gross_subtotal_cents": 17000,
  "discount_cents": 500,
  "net_subtotal_cents": 16500,
  "total_base_tax_cents": 1412,
  "total_surcharge_cents": 162,
  "total_tax_cents": 1574,
  "grand_total_cents": 18074,
  "items": [
    {
      "id": "item-1",
      "gross_cents": 15000,
      "discount_cents": 441,
      "net_cents": 14559,
      "base_tax_cents": 1456,
      "surcharge_cents": 243,
      "total_tax_cents": 1699,
      "total_cents": 16258
    }
  ]
}
```

Items in output must be sorted in the original order of `<invoice.json>`.

## Exit Codes

- `0`: Success.
- `2`: Missing arguments, negative values, unknown categories, or malformed JSON.
