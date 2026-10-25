#!/usr/bin/env node
const fs = require('fs');
const http = require('http');
const crypto = require('crypto');
const { URL } = require('url');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function sendHttp(endpointUrl, bodyBuf, headers, timeoutMs) {
  return new Promise((resolve) => {
    let u;
    try {
      u = new URL(endpointUrl);
    } catch (e) {
      return resolve({ status: null, error: e });
    }

    const options = {
      method: 'POST',
      hostname: u.hostname,
      port: u.port || 80,
      path: u.pathname + (u.search || ''),
      headers: {
        ...headers,
        'Content-Length': bodyBuf.length
      },
      timeout: timeoutMs
    };

    const req = http.request(options, (res) => {
      res.on('data', () => {});
      res.on('end', () => {
        resolve({ status: res.statusCode, error: null });
      });
    });

    req.on('timeout', () => {
      req.destroy();
      resolve({ status: null, error: new Error('timeout') });
    });

    req.on('error', (err) => {
      resolve({ status: null, error: err });
    });

    req.write(bodyBuf);
    req.end();
  });
}

async function main() {
  const args = process.argv.slice(2);
  let eventsPath = null;
  let reportPath = null;
  let endpoint = 'http://127.0.0.1:8080/events';
  let secret = 'default-secret';
  let offlineStatus = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--endpoint') {
      if (i + 1 >= args.length) fail('Missing value for --endpoint', 2);
      endpoint = args[++i];
    } else if (args[i] === '--secret') {
      if (i + 1 >= args.length) fail('Missing value for --secret', 2);
      secret = args[++i];
    } else if (args[i] === '--offline-status') {
      if (i + 1 >= args.length) fail('Missing value for --offline-status', 2);
      offlineStatus = parseInt(args[++i], 10);
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!eventsPath) {
      eventsPath = args[i];
    } else if (!reportPath) {
      reportPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!eventsPath || !reportPath) {
    fail('Usage: ./run <events.json> <report.json> [--endpoint <url>] [--secret <string>] [--offline-status <code>]', 2);
  }

  let data;
  try {
    const raw = fs.readFileSync(eventsPath, 'utf8');
    data = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse events json: ${err.message}`, 2);
  }

  if (!data || typeof data !== 'object' || !Array.isArray(data.events)) {
    fail("Input JSON must contain top-level object with 'events' array", 2);
  }

  const deliveries = [];
  let totalDelivered = 0;
  let totalFailed = 0;

  for (const ev of data.events) {
    if (!ev || typeof ev !== 'object') {
      fail('Event must be an object', 2);
    }
    const { id, topic, payload } = ev;
    if (typeof id !== 'string' || !id.trim()) {
      fail("Event missing valid 'id'", 2);
    }
    if (typeof topic !== 'string' || !topic.trim()) {
      fail(`Event '${id}' missing valid 'topic'`, 2);
    }
    if (payload === undefined) {
      fail(`Event '${id}' missing 'payload'`, 2);
    }

    const max_attempts = ev.max_attempts !== undefined ? ev.max_attempts : 3;
    if (typeof max_attempts !== 'number' || !Number.isInteger(max_attempts) || max_attempts < 1) {
      fail(`Event '${id}' invalid max_attempts`, 2);
    }

    const timeout_ms = ev.timeout_ms !== undefined ? ev.timeout_ms : 2000;
    if (typeof timeout_ms !== 'number' || !Number.isInteger(timeout_ms) || timeout_ms <= 0) {
      fail(`Event '${id}' invalid timeout_ms`, 2);
    }

    const bodyStr = JSON.stringify(payload);
    const bodyBuf = Buffer.from(bodyStr, 'utf8');
    const hmacSig = crypto.createHmac('sha256', secret).update(bodyBuf).digest('hex');
    const sigHeader = `sha256=${hmacSig}`;

    let attempts = 0;
    let finalStatusCode = null;
    let delivered = false;

    for (let attempt = 0; attempt < max_attempts; attempt++) {
      attempts++;
      if (offlineStatus !== null) {
        finalStatusCode = offlineStatus;
        if (finalStatusCode >= 200 && finalStatusCode <= 299) {
          delivered = true;
          break;
        } else if (finalStatusCode >= 400 && finalStatusCode <= 499) {
          delivered = false;
          break;
        } else {
          if (attempt + 1 < max_attempts) {
            await sleep(Math.min(50, 10 * (attempt + 1)));
          }
          continue;
        }
      }

      const headers = {
        'Content-Type': 'application/json',
        'X-Event-ID': id,
        'X-Event-Topic': topic,
        'X-Signature': sigHeader
      };

      const res = await sendHttp(endpoint, bodyBuf, headers, timeout_ms);
      finalStatusCode = res.status;

      if (res.status && res.status >= 200 && res.status <= 299) {
        delivered = true;
        break;
      }
      if (res.status && res.status >= 400 && res.status <= 499) {
        delivered = false;
        break;
      }

      if (attempt + 1 < max_attempts) {
        await sleep(Math.min(50, 10 * (attempt + 1)));
      }
    }

    if (delivered) {
      totalDelivered++;
    } else {
      totalFailed++;
    }

    deliveries.append ? null : deliveries.push({
      id,
      status: delivered ? 'delivered' : 'failed',
      attempts,
      last_status_code: finalStatusCode,
      signature: sigHeader
    });
  }

  const report = {
    total_events: deliveries.length,
    delivered: totalDelivered,
    failed: totalFailed,
    deliveries
  };

  try {
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write report: ${err.message}`, 2);
  }

  process.exit(totalFailed === 0 ? 0 : 1);
}

main();
