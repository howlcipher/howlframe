#!/usr/bin/env node
const fs = require('fs');
const http = require('http');
const { URL } = require('url');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function probeHttp(urlStr, timeoutMs, fixtures, method = 'GET', body = null) {
  if (fixtures !== null) {
    if (fixtures[urlStr]) {
      const fix = fixtures[urlStr];
      return Promise.resolve({
        status: fix.status !== undefined ? fix.status : 200,
        body: fix.body || {}
      });
    }
    return Promise.resolve({ status: 404, body: { error: 'not in fixtures' } });
  }

  return new Promise((resolve) => {
    let u;
    try {
      u = new URL(urlStr);
    } catch (e) {
      return resolve({ status: null, body: {} });
    }

    const bodyBuf = body !== null ? Buffer.from(JSON.stringify(body), 'utf8') : null;
    const options = {
      method,
      hostname: u.hostname,
      port: u.port || 80,
      path: u.pathname + (u.search || ''),
      headers: bodyBuf ? { 'Content-Type': 'application/json', 'Content-Length': bodyBuf.length } : {},
      timeout: timeoutMs
    };

    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', d => data += d);
      res.on('end', () => {
        let parsed = {};
        try {
          parsed = JSON.parse(data);
        } catch (e) {}
        resolve({ status: res.statusCode, body: parsed });
      });
    });

    req.on('timeout', () => {
      req.destroy();
      resolve({ status: null, body: {} });
    });

    req.on('error', () => {
      resolve({ status: null, body: {} });
    });

    if (bodyBuf) {
      req.write(bodyBuf);
    }
    req.end();
  });
}

async function main() {
  const args = process.argv.slice(2);
  let clusterPath = null;
  let statusPath = null;
  let offlineFixtures = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--offline-fixtures') {
      if (i + 1 >= args.length) fail('Missing value for --offline-fixtures', 2);
      offlineFixtures = args[++i];
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!clusterPath) {
      clusterPath = args[i];
    } else if (!statusPath) {
      statusPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!clusterPath || !statusPath) {
    fail('Usage: ./run <cluster.json> <status.json> [--offline-fixtures <fixtures.json>]', 2);
  }

  let cluster;
  try {
    const raw = fs.readFileSync(clusterPath, 'utf8');
    cluster = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse cluster JSON: ${err.message}`, 2);
  }

  if (!cluster || typeof cluster !== 'object' || !Array.isArray(cluster.services)) {
    fail("Cluster must contain 'services' array", 2);
  }

  let fixtures = null;
  if (offlineFixtures) {
    try {
      const raw = fs.readFileSync(offlineFixtures, 'utf8');
      fixtures = JSON.parse(raw);
    } catch (err) {
      fail(`Failed to read fixtures JSON: ${err.message}`, 2);
    }
  }

  const timeoutMs = typeof cluster.timeout_ms === 'number' ? cluster.timeout_ms : 2000;

  const servicesResults = [];
  let healthyCount = 0;
  let failedOverCount = 0;
  let degradedCount = 0;

  for (const s of cluster.services) {
    if (!s || typeof s !== 'object') {
      fail('Service must be an object', 2);
    }

    const { name, primary_url, standby_url, activate_url } = s;
    if (typeof name !== 'string' || !name.trim()) {
      fail("Service missing valid 'name'", 2);
    }
    if (typeof primary_url !== 'string' || !primary_url.trim()) {
      fail(`Service '${name}' missing valid 'primary_url'`, 2);
    }
    if (typeof standby_url !== 'string' || !standby_url.trim()) {
      fail(`Service '${name}' missing valid 'standby_url'`, 2);
    }
    if (typeof activate_url !== 'string' || !activate_url.trim()) {
      fail(`Service '${name}' missing valid 'activate_url'`, 2);
    }

    // 1. Primary probe
    const pRes = await probeHttp(primary_url, timeoutMs, fixtures, 'GET');
    const pIsHealthy = (pRes.status === 200 && pRes.body && pRes.body.status === 'healthy');

    if (pIsHealthy) {
      healthyCount++;
      servicesResults.push({
        name,
        state: 'primary_active',
        primary_health: 'healthy',
        standby_health: 'not_checked',
        activated: false
      });
      continue;
    }

    // Primary down -> probe standby
    const sRes = await probeHttp(standby_url, timeoutMs, fixtures, 'GET');
    const sIsHealthy = (sRes.status === 200 && sRes.body && sRes.body.status === 'healthy');

    if (sIsHealthy) {
      const actRes = await probeHttp(activate_url, timeoutMs, fixtures, 'POST', {});
      if (actRes.status === 200 || actRes.status === 204) {
        failedOverCount++;
        servicesResults.push({
          name,
          state: 'failed_over_to_standby',
          primary_health: 'unhealthy',
          standby_health: 'healthy',
          activated: true
        });
      } else {
        degradedCount++;
        servicesResults.push({
          name,
          state: 'degraded',
          primary_health: 'unhealthy',
          standby_health: 'healthy',
          activated: false
        });
      }
    } else {
      degradedCount++;
      servicesResults.push({
        name,
        state: 'degraded',
        primary_health: 'unhealthy',
        standby_health: 'unhealthy',
        activated: false
      });
    }
  }

  const report = {
    total_services: servicesResults.length,
    healthy: healthyCount,
    failed_over: failedOverCount,
    degraded: degradedCount,
    services: servicesResults
  };

  try {
    fs.writeFileSync(statusPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write status: ${err.message}`, 2);
  }

  process.exit(degradedCount === 0 ? 0 : 1);
}

main();
