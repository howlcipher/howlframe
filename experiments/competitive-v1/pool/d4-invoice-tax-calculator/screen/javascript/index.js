#!/usr/bin/env node
const fs = require('fs');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function bankersRound(val) {
  const floorVal = Math.floor(val);
  const diff = val - floorVal;
  if (Math.abs(diff - 0.5) < 1e-9) {
    return floorVal % 2 === 0 ? floorVal : floorVal + 1;
  }
  return Math.round(val);
}

function main() {
  const args = process.argv.slice(2);
  if (args.length !== 3) {
    fail('Usage: ./run <tax_config.json> <invoice.json> <receipt.json>', 2);
  }

  const [cfgPath, invPath, recPath] = args;

  let cfg, inv;
  try {
    cfg = JSON.parse(fs.readFileSync(cfgPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse tax config JSON: ${err.message}`, 2);
  }

  try {
    inv = JSON.parse(fs.readFileSync(invPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse invoice JSON: ${err.message}`, 2);
  }

  if (!cfg || typeof cfg !== 'object') {
    fail('Tax config must be an object', 2);
  }
  if (!inv || typeof inv !== 'object' || !Array.isArray(inv.items)) {
    fail("Invoice must contain 'items' array", 2);
  }

  const categories = {};
  if (cfg.categories && typeof cfg.categories === 'object') {
    for (const [cat, rate] of Object.entries(cfg.categories)) {
      if (typeof rate !== 'number' || rate < 0) {
        fail(`Invalid rate for category '${cat}'`, 2);
      }
      categories[cat] = rate;
    }
  } else if (typeof cfg.flat_tax_rate === 'number' && cfg.flat_tax_rate >= 0) {
    categories['standard'] = cfg.flat_tax_rate;
  } else {
    fail("Tax config missing 'categories' or 'flat_tax_rate'", 2);
  }

  let surchargeThresh = null;
  let surchargeRate = 0.0;
  if (cfg.luxury_surcharge && typeof cfg.luxury_surcharge === 'object') {
    surchargeThresh = typeof cfg.luxury_surcharge.threshold_cents === 'number' ? cfg.luxury_surcharge.threshold_cents : 0;
    surchargeRate = typeof cfg.luxury_surcharge.rate === 'number' ? cfg.luxury_surcharge.rate : 0.0;
  }

  const items = [];
  for (const it of inv.items) {
    if (!it || typeof it !== 'object') {
      fail('Invoice item must be an object', 2);
    }
    const { id, unit_price_cents: price, quantity: qty } = it;
    const cat = it.category !== undefined ? it.category : 'standard';

    if (typeof id !== 'string' || !id.trim()) {
      fail("Item missing valid 'id'", 2);
    }
    if (typeof price !== 'number' || !Number.isInteger(price) || price < 0) {
      fail(`Item '${id}' has invalid unit_price_cents`, 2);
    }
    if (typeof qty !== 'number' || !Number.isInteger(qty) || qty < 0) {
      fail(`Item '${id}' has invalid quantity`, 2);
    }
    if (categories[cat] === undefined) {
      fail(`Item '${id}' has unknown category '${cat}'`, 2);
    }

    items.push({
      id,
      description: it.description || '',
      unit_price_cents: price,
      quantity: qty,
      category: cat,
      gross_cents: price * qty
    });
  }

  const grossSubtotal = items.reduce((acc, it) => acc + it.gross_cents, 0);
  const requestedDiscount = typeof inv.discount_cents === 'number' ? inv.discount_cents : 0;
  if (requestedDiscount < 0 || !Number.isInteger(requestedDiscount)) {
    fail('Invalid discount_cents', 2);
  }
  const discountCents = Math.min(requestedDiscount, grossSubtotal);

  // Proportional discount distribution
  const nonExempt = items.filter(it => it.category !== 'exempt');
  const eligibleItems = nonExempt.length > 0 ? nonExempt : items;
  const eligibleGross = eligibleItems.reduce((acc, it) => acc + it.gross_cents, 0);

  const itemDiscounts = {};
  for (const it of items) {
    itemDiscounts[it.id] = 0;
  }

  if (discountCents > 0 && eligibleGross > 0) {
    let allocated = 0;
    const remainders = [];
    for (const it of eligibleItems) {
      const exact = discountCents * (it.gross_cents / eligibleGross);
      const baseD = Math.floor(exact);
      const rem = exact - baseD;
      itemDiscounts[it.id] = baseD;
      allocated += baseD;
      remainders.push({ rem, id: it.id });
    }

    const leftover = discountCents - allocated;
    remainders.sort((a, b) => {
      if (Math.abs(b.rem - a.rem) > 1e-9) return b.rem - a.rem;
      return a.id.localeCompare(b.id);
    });

    for (let i = 0; i < leftover; i++) {
      itemDiscounts[remainders[i].id] += 1;
    }
  }

  const receiptItems = [];
  let totalBaseTax = 0;
  let totalSurcharge = 0;

  for (const it of items) {
    const gross = it.gross_cents;
    const d = itemDiscounts[it.id];
    const net = gross - d;
    const rate = categories[it.category];

    const baseTax = bankersRound(net * rate);

    let surcharge = 0;
    if (surchargeThresh !== null && it.unit_price_cents > surchargeThresh && it.category !== 'exempt') {
      const surBasePerUnit = it.unit_price_cents - surchargeThresh;
      if (it.unit_price_cents > 0) {
        const surRatio = surBasePerUnit / it.unit_price_cents;
        const netSurBase = net * surRatio;
        surcharge = bankersRound(netSurBase * surchargeRate);
      }
    }

    const totalItemTax = baseTax + surcharge;
    const totalItemCents = net + totalItemTax;

    totalBaseTax += baseTax;
    totalSurcharge += surcharge;

    receiptItems.push({
      id: it.id,
      gross_cents: gross,
      discount_cents: d,
      net_cents: net,
      base_tax_cents: baseTax,
      surcharge_cents: surcharge,
      total_tax_cents: totalItemTax,
      total_cents: totalItemCents
    });
  }

  const netSubtotal = grossSubtotal - discountCents;
  const totalTax = totalBaseTax + totalSurcharge;
  const grandTotal = netSubtotal + totalTax;

  const receipt = {
    gross_subtotal_cents: grossSubtotal,
    discount_cents: discountCents,
    net_subtotal_cents: netSubtotal,
    total_base_tax_cents: totalBaseTax,
    total_surcharge_cents: totalSurcharge,
    total_tax_cents: totalTax,
    grand_total_cents: grandTotal,
    items: receiptItems
  };

  try {
    fs.writeFileSync(recPath, JSON.stringify(receipt, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write receipt: ${err.message}`, 2);
  }

  process.exit(0);
}

main();
