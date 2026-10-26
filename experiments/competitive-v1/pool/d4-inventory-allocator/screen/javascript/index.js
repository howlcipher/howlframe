#!/usr/bin/env node
const fs = require('fs');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function main() {
  const args = process.argv.slice(2);
  if (args.length !== 3) {
    fail('Usage: ./run <inventory.json> <orders.json> <manifest.json>', 2);
  }

  const [invPath, ordPath, manPath] = args;

  let invData, ordData;
  try {
    invData = JSON.parse(fs.readFileSync(invPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse inventory JSON: ${err.message}`, 2);
  }

  try {
    ordData = JSON.parse(fs.readFileSync(ordPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse orders JSON: ${err.message}`, 2);
  }

  if (!invData || typeof invData !== 'object' || !invData.items || typeof invData.items !== 'object') {
    fail("Inventory JSON must contain 'items' object", 2);
  }

  if (!ordData || typeof ordData !== 'object' || !Array.isArray(ordData.orders)) {
    fail("Orders JSON must contain 'orders' array", 2);
  }

  const stock = new Map();
  for (const [sku, itemInfo] of Object.entries(invData.items)) {
    if (!itemInfo || typeof itemInfo !== 'object') {
      fail(`Item '${sku}' info must be an object`, 2);
    }
    const lots = [];
    if (Array.isArray(itemInfo.lots)) {
      for (const l of itemInfo.lots) {
        if (!l || typeof l !== 'object' || l.lot_id === undefined || typeof l.quantity !== 'number' || l.quantity < 0) {
          fail(`Invalid lot in item '${sku}'`, 2);
        }
        lots.push({
          lot_id: String(l.lot_id),
          quantity: l.quantity,
          expires: l.expires ? String(l.expires) : '9999-12-31'
        });
      }
    } else if (typeof itemInfo.quantity === 'number') {
      if (itemInfo.quantity < 0) {
        fail(`Invalid quantity in item '${sku}'`, 2);
      }
      lots.push({
        lot_id: 'default',
        quantity: itemInfo.quantity,
        expires: '9999-12-31'
      });
    } else {
      fail(`Item '${sku}' missing 'lots' or 'quantity'`, 2);
    }

    lots.sort((a, b) => {
      if (a.expires !== b.expires) return a.expires.localeCompare(b.expires);
      return a.lot_id.localeCompare(b.lot_id);
    });
    stock.set(sku, lots);
  }

  const priorityOrder = { VIP: 1, STANDARD: 2, BULK: 3 };
  const validatedOrders = [];

  for (let idx = 0; idx < ordData.orders.length; idx++) {
    const o = ordData.orders[idx];
    if (!o || typeof o !== 'object') {
      fail('Order must be an object', 2);
    }
    const { id } = o;
    if (typeof id !== 'string' || !id.trim()) {
      fail("Order missing valid 'id'", 2);
    }

    const pri = o.priority !== undefined ? o.priority : 'STANDARD';
    if (!priorityOrder[pri]) {
      fail(`Order '${id}' has invalid priority: ${pri}`, 2);
    }

    const { items } = o;
    if (!items || typeof items !== 'object' || Array.isArray(items) || Object.keys(items).length === 0) {
      fail(`Order '${id}' missing valid 'items' map`, 2);
    }

    for (const [sku, q] of Object.entries(items)) {
      if (!stock.has(sku)) {
        fail(`Order '${id}' requested unknown SKU: ${sku}`, 2);
      }
      if (typeof q !== 'number' || !Number.isInteger(q) || q < 0) {
        fail(`Order '${id}' has invalid quantity for SKU: ${sku}`, 2);
      }
    }

    validatedOrders.push({
      priorityWeight: priorityOrder[pri],
      index: idx,
      order: o,
      priority: pri
    });
  }

  validatedOrders.sort((a, b) => {
    if (a.priorityWeight !== b.priorityWeight) return a.priorityWeight - b.priorityWeight;
    return a.index - b.index;
  });

  const manifestOrders = [];
  let fulfilledCount = 0;
  let partiallyFulfilledCount = 0;
  let backorderedCount = 0;

  for (const item of validatedOrders) {
    const { order, priority } = item;
    const oid = order.id;
    const allocations = [];
    const backordered = {};

    let totalRequested = 0;
    let totalAllocated = 0;

    const requestedSkus = Object.keys(order.items).sort();
    for (const sku of requestedSkus) {
      const reqQty = order.items[sku];
      totalRequested += reqQty;
      let needed = reqQty;
      const skuLots = stock.get(sku);

      for (const lot of skuLots) {
        if (needed === 0) break;
        if (lot.quantity > 0) {
          const take = Math.min(lot.quantity, needed);
          lot.quantity -= take;
          needed -= take;
          totalAllocated += take;
          allocations.push({
            sku,
            lot_id: lot.lot_id,
            quantity: take
          });
        }
      }

      if (needed > 0) {
        backordered[sku] = needed;
      }
    }

    let status;
    if (totalAllocated === totalRequested) {
      status = 'fulfilled';
      fulfilledCount++;
    } else if (totalAllocated > 0) {
      status = 'partially_fulfilled';
      partiallyFulfilledCount++;
    } else {
      status = 'backordered';
      backorderedCount++;
    }

    allocations.sort((a, b) => {
      if (a.sku !== b.sku) return a.sku.localeCompare(b.sku);
      return a.lot_id.localeCompare(b.lot_id);
    });

    manifestOrders.push({
      id: oid,
      priority,
      status,
      allocations,
      backordered
    });
  }

  const remainingInventory = {};
  const sortedSkus = Array.from(stock.keys()).sort();
  for (const sku of sortedSkus) {
    const lots = stock.get(sku);
    lots.sort((a, b) => {
      if (a.expires !== b.expires) return a.expires.localeCompare(b.expires);
      return a.lot_id.localeCompare(b.lot_id);
    });
    const totalQty = lots.reduce((acc, l) => acc + l.quantity, 0);
    remainingInventory[sku] = {
      total_quantity: totalQty,
      lots: lots.map(l => ({
        lot_id: l.lot_id,
        quantity: l.quantity,
        expires: l.expires
      }))
    };
  }

  const report = {
    total_orders: manifestOrders.length,
    fulfilled_count: fulfilledCount,
    partially_fulfilled_count: partiallyFulfilledCount,
    backordered_count: backorderedCount,
    orders: manifestOrders,
    remaining_inventory: remainingInventory
  };

  try {
    fs.writeFileSync(manPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write manifest: ${err.message}`, 2);
  }

  process.exit(fulfilledCount === manifestOrders.length ? 0 : 1);
}

main();
