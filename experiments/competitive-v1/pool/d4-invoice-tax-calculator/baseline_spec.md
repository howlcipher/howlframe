# Baseline Specification: Invoice Calculator (v1)

`invoice-calculator` calculates invoice line items and tax totals.

## Invocation

```bash
./run <tax_config.json> <invoice.json> <receipt.json>
```

## Baseline Input Formats

### `<tax_config.json>`
```json
{
  "flat_tax_rate": 0.10
}
```

### `<invoice.json>`
```json
{
  "items": [
    {
      "id": "item-1",
      "description": "Standard Widget",
      "unit_price_cents": 1000,
      "quantity": 2
    }
  ],
  "discount_cents": 200
}
```

## Baseline Behavior

1. Gross subtotal = sum of `unit_price_cents * quantity` for each line item.
2. Net subtotal = `gross_subtotal - discount_cents`.
3. Total tax = `round(net_subtotal * flat_tax_rate)`.
4. Grand total = `net_subtotal + total_tax`.

## Baseline Output Format (`<receipt.json>`)

```json
{
  "gross_subtotal_cents": 2000,
  "discount_cents": 200,
  "net_subtotal_cents": 1800,
  "total_tax_cents": 180,
  "grand_total_cents": 1980
}
```

## Exit Codes

- `0`: Success.
- `2`: Missing arguments, negative values, or malformed JSON.
