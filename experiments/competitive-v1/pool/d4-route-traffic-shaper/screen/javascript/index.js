#!/usr/bin/env node
const fs = require('fs');
const crypto = require('crypto');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function main() {
  const args = process.argv.slice(2);
  if (args.length !== 3) {
    fail('Usage: ./run <routing_table.json> <requests.json> <dispatch_log.json>', 2);
  }

  const [tblPath, reqPath, logPath] = args;

  let tableData, reqData;
  try {
    tableData = JSON.parse(fs.readFileSync(tblPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse routing table JSON: ${err.message}`, 2);
  }

  try {
    reqData = JSON.parse(fs.readFileSync(reqPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse requests JSON: ${err.message}`, 2);
  }

  if (!tableData || typeof tableData !== 'object' || !Array.isArray(tableData.routes)) {
    fail("Routing table must contain 'routes' array", 2);
  }

  if (!reqData || typeof reqData !== 'object' || !Array.isArray(reqData.requests)) {
    fail("Requests JSON must contain 'requests' array", 2);
  }

  const backendsHealth = (tableData.backends_health && typeof tableData.backends_health === 'object')
    ? tableData.backends_health
    : {};

  const routes = tableData.routes;
  const dispatches = [];
  let routedCount = 0;
  let failedCount = 0;

  for (const req of reqData.requests) {
    if (!req || typeof req !== 'object') {
      fail('Request must be an object', 2);
    }

    const { id: reqId, path: reqPathStr } = req;
    if (typeof reqId !== 'string' || !reqId.trim()) {
      fail("Request missing valid 'id'", 2);
    }
    if (typeof reqPathStr !== 'string' || !reqPathStr.trim()) {
      fail(`Request '${reqId}' missing valid 'path'`, 2);
    }

    const reqHeaders = req.headers && typeof req.headers === 'object' ? req.headers : {};
    const lowerHeaders = {};
    for (const [k, v] of Object.entries(reqHeaders)) {
      lowerHeaders[k.toLowerCase()] = String(v);
    }

    let matchedRoute = null;
    for (const r of routes) {
      const prefix = typeof r.path_prefix === 'string' ? r.path_prefix : '';
      if (!reqPathStr.startsWith(prefix)) {
        continue;
      }

      if (r.headers_match && typeof r.headers_match === 'object') {
        let headersMatch = true;
        for (const [hk, hv] of Object.entries(r.headers_match)) {
          if (lowerHeaders[hk.toLowerCase()] !== String(hv)) {
            headersMatch = false;
            break;
          }
        }
        if (!headersMatch) continue;
      }

      matchedRoute = r;
      break;
    }

    if (!matchedRoute) {
      failedCount++;
      dispatches.push({
        id: reqId,
        matched_route: null,
        selected_target: null,
        canary_routed: false,
        fallback_triggered: false,
        status: '404_not_found'
      });
      continue;
    }

    // Target selection (Canary check)
    const canary = matchedRoute.canary;
    let candidateTarget = matchedRoute.target;
    let canaryRouted = false;

    if (canary && typeof canary === 'object' && typeof canary.weight_pct === 'number' && canary.weight_pct > 0) {
      const weightPct = canary.weight_pct;
      const userId = req.user_id;
      const clientIp = req.client_ip || '127.0.0.1';
      const hashKey = (userId && String(userId).trim()) ? String(userId) : String(clientIp);

      const hHex = crypto.createHash('sha256').update(Buffer.from(hashKey, 'utf8')).digest('hex');
      const bucket = parseInt(hHex.substring(0, 8), 16) % 100;

      if (bucket < weightPct) {
        candidateTarget = canary.target;
        canaryRouted = true;
      }
    }

    // Health check & Fallback
    const isHealthy = backendsHealth[candidateTarget] !== undefined ? backendsHealth[candidateTarget] : true;
    let selectedTarget = null;
    let fallbackTriggered = false;
    let status = '503_service_unavailable';

    if (isHealthy) {
      selectedTarget = candidateTarget;
      fallbackTriggered = false;
      status = 'routed';
    } else {
      const fallbackTarget = matchedRoute.fallback_target;
      const fallbackHealthy = fallbackTarget ? (backendsHealth[fallbackTarget] !== undefined ? backendsHealth[fallbackTarget] : true) : false;
      if (fallbackTarget && fallbackHealthy) {
        selectedTarget = fallbackTarget;
        fallbackTriggered = true;
        status = 'routed';
      } else {
        selectedTarget = null;
        fallbackTriggered = false;
        status = '503_service_unavailable';
      }
    }

    if (status === 'routed') {
      routedCount++;
    } else {
      failedCount++;
    }

    dispatches.push({
      id: reqId,
      matched_route: matchedRoute.path_prefix || null,
      selected_target: selectedTarget,
      canary_routed: canaryRouted,
      fallback_triggered: fallbackTriggered,
      status
    });
  }

  const report = {
    total_requests: dispatches.length,
    routed_count: routedCount,
    failed_count: failedCount,
    dispatches
  };

  try {
    fs.writeFileSync(logPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write dispatch log: ${err.message}`, 2);
  }

  process.exit(failedCount === 0 ? 0 : 1);
}

main();
