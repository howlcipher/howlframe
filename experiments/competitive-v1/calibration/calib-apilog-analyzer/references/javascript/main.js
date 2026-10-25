#!/usr/bin/env node
'use strict';

const fs = require('fs');
const readline = require('readline');

const ALLOWED_METHODS = new Set(['GET', 'POST', 'PUT', 'DELETE', 'PATCH', 'HEAD', 'OPTIONS']);

function parseArgs() {
  const args = process.argv.slice(2);
  let inputPath = null;
  let outputPath = null;
  let slaLatencyMs = 500;
  let errorRateThreshold = 0.05;
  let windowSeconds = 300;

  for (let i = 0; i < args.length; i++) {
    const arg = args[i];
    let key = arg;
    let val = null;

    if (arg.startsWith('--') && arg.includes('=')) {
      const idx = arg.indexOf('=');
      key = arg.slice(0, idx);
      val = arg.slice(idx + 1);
    } else if (arg.startsWith('--')) {
      key = arg;
      if (i + 1 < args.length) {
        val = args[++i];
      } else {
        process.stderr.write(`Error: Missing value for option ${key}\n`);
        process.exit(1);
      }
    } else {
      process.stderr.write(`Error: Unrecognized argument: ${arg}\n`);
      process.exit(1);
    }

    switch (key) {
      case '--input':
        inputPath = val;
        break;
      case '--output':
        outputPath = val;
        break;
      case '--sla-latency-ms': {
        const n = Number(val);
        if (!Number.isInteger(n) || n < 0) {
          process.stderr.write(`Error: --sla-latency-ms must be a non-negative integer, got '${val}'\n`);
          process.exit(1);
        }
        slaLatencyMs = n;
        break;
      }
      case '--error-rate-threshold': {
        const f = Number(val);
        if (Number.isNaN(f) || f < 0.0 || f > 1.0) {
          process.stderr.write(`Error: --error-rate-threshold must be between 0.0 and 1.0, got '${val}'\n`);
          process.exit(1);
        }
        errorRateThreshold = f;
        break;
      }
      case '--window-seconds': {
        const n = Number(val);
        if (!Number.isInteger(n) || n <= 0) {
          process.stderr.write(`Error: --window-seconds must be a positive integer, got '${val}'\n`);
          process.exit(1);
        }
        windowSeconds = n;
        break;
      }
      default:
        process.stderr.write(`Error: Unknown option ${key}\n`);
        process.exit(1);
    }
  }

  return { inputPath, outputPath, slaLatencyMs, errorRateThreshold, windowSeconds };
}

function roundTo(val, decimals) {
  const factor = Math.pow(10, decimals);
  return Math.round((val + Number.EPSILON) * factor) / factor;
}

async function main() {
  const { inputPath, outputPath, slaLatencyMs, errorRateThreshold, windowSeconds } = parseArgs();

  let inStream;
  if (!inputPath || inputPath === '-') {
    inStream = process.stdin;
  } else {
    try {
      const stat = fs.statSync(inputPath);
      if (stat.isDirectory()) {
        throw new Error('path is a directory');
      }
      fs.accessSync(inputPath, fs.constants.R_OK);
      inStream = fs.createReadStream(inputPath, { encoding: 'utf8' });
    } catch (err) {
      process.stderr.write(`Error: Cannot open input file '${inputPath}': ${err.message}\n`);
      process.exit(2);
    }
  }

  // Pre-check output path writability if specified and not stdout
  let outFd = null;
  if (outputPath && outputPath !== '-') {
    try {
      outFd = fs.openSync(outputPath, 'w');
    } catch (err) {
      process.stderr.write(`Error: Cannot open output file '${outputPath}': ${err.message}\n`);
      process.exit(1);
    }
  }

  let totalRecords = 0;
  let validRecords = 0;
  let invalidRecords = 0;
  let earliestTimestamp = null;
  let latestTimestamp = null;
  let totalBytesSent = 0;

  const services = new Map();
  const clients = new Map();
  const windows = new Map();

  const rl = readline.createInterface({
    input: inStream,
    crlfDelay: Infinity,
  });

  for await (const line of rl) {
    const trimmed = line.trim();
    if (!trimmed) {
      continue;
    }

    totalRecords++;

    let record;
    try {
      record = JSON.parse(trimmed);
    } catch {
      invalidRecords++;
      continue;
    }

    if (!record || typeof record !== 'object' || Array.isArray(record)) {
      invalidRecords++;
      continue;
    }

    const {
      timestamp: ts,
      service: svc,
      client_id: cid,
      method,
      path,
      status,
      latency_ms: lat,
      bytes_sent: bytesSent,
    } = record;

    if (
      typeof ts !== 'number' || !Number.isSafeInteger(ts) || ts < 0 ||
      typeof svc !== 'string' || svc.length === 0 ||
      typeof cid !== 'string' || cid.length === 0 ||
      typeof method !== 'string' || !ALLOWED_METHODS.has(method) ||
      typeof path !== 'string' || !path.startsWith('/') ||
      typeof status !== 'number' || !Number.isInteger(status) || status < 100 || status > 599 ||
      typeof lat !== 'number' || !Number.isSafeInteger(lat) || lat < 0 ||
      typeof bytesSent !== 'number' || !Number.isSafeInteger(bytesSent) || bytesSent < 0
    ) {
      invalidRecords++;
      continue;
    }

    validRecords++;

    if (earliestTimestamp === null || ts < earliestTimestamp) {
      earliestTimestamp = ts;
    }
    if (latestTimestamp === null || ts > latestTimestamp) {
      latestTimestamp = ts;
    }
    totalBytesSent += bytesSent;

    // Service aggregation
    let s = services.get(svc);
    if (!s) {
      s = {
        total: 0,
        success: 0,
        clientErr: 0,
        serverErr: 0,
        sumLatency: 0,
        minLatency: lat,
        maxLatency: lat,
        totalBytes: 0,
      };
      services.set(svc, s);
    }
    s.total++;
    if (status >= 200 && status < 400) {
      s.success++;
    } else if (status >= 400 && status < 500) {
      s.clientErr++;
    } else if (status >= 500) {
      s.serverErr++;
    }
    s.sumLatency += lat;
    if (lat < s.minLatency) s.minLatency = lat;
    if (lat > s.maxLatency) s.maxLatency = lat;
    s.totalBytes += bytesSent;

    // Client aggregation
    let c = clients.get(cid);
    if (!c) {
      c = { count: 0, totalBytes: 0, errors: 0 };
      clients.set(cid, c);
    }
    c.count++;
    c.totalBytes += bytesSent;
    if (status >= 400) {
      c.errors++;
    }

    // Window aggregation
    const wStart = ts - (ts % windowSeconds);
    let w = windows.get(wStart);
    if (!w) {
      w = { total: 0, errors: 0, sumLatency: 0 };
      windows.set(wStart, w);
    }
    w.total++;
    if (status >= 400) {
      w.errors++;
    }
    w.sumLatency += lat;
  }

  // Compile services
  const sortedServiceNames = Array.from(services.keys()).sort();
  let servicesWithSlaBreach = 0;
  const svcList = [];

  for (const name of sortedServiceNames) {
    const s = services.get(name);
    const errCount = s.clientErr + s.serverErr;
    const errRate = s.total > 0 ? roundTo(errCount / s.total, 4) : 0.0;
    const avgLat = s.total > 0 ? roundTo(s.sumLatency / s.total, 2) : 0.0;
    const breach = (errRate > errorRateThreshold) || (avgLat > slaLatencyMs);
    if (breach) {
      servicesWithSlaBreach++;
    }
    svcList.push({
      service: name,
      total_requests: s.total,
      successful_requests: s.success,
      client_error_requests: s.clientErr,
      server_error_requests: s.serverErr,
      error_rate: errRate,
      min_latency_ms: s.minLatency,
      max_latency_ms: s.maxLatency,
      avg_latency_ms: avgLat,
      total_bytes_sent: s.totalBytes,
      sla_breach: breach,
    });
  }

  // Compile top clients
  const clientEntries = Array.from(clients.entries()).map(([cid, c]) => ({
    client_id: cid,
    request_count: c.count,
    total_bytes: c.totalBytes,
    error_count: c.errors,
  }));
  clientEntries.sort((a, b) => {
    if (b.request_count !== a.request_count) {
      return b.request_count - a.request_count;
    }
    return a.client_id < b.client_id ? -1 : a.client_id > b.client_id ? 1 : 0;
  });
  const topClients = clientEntries.slice(0, 10);

  // Compile windows
  const sortedWindowStarts = Array.from(windows.keys()).sort((a, b) => a - b);
  const windowList = [];
  for (const wStart of sortedWindowStarts) {
    const w = windows.get(wStart);
    const avgLat = w.total > 0 ? roundTo(w.sumLatency / w.total, 2) : 0.0;
    windowList.push({
      window_start: wStart,
      window_end: wStart + windowSeconds,
      total_requests: w.total,
      error_requests: w.errors,
      avg_latency_ms: avgLat,
    });
  }

  const report = {
    summary: {
      total_records: totalRecords,
      valid_records: validRecords,
      invalid_records: invalidRecords,
      earliest_timestamp: earliestTimestamp,
      latest_timestamp: latestTimestamp,
      total_bytes_sent: totalBytesSent,
      total_services: services.size,
      services_with_sla_breach: servicesWithSlaBreach,
    },
    services: svcList,
    top_clients: topClients,
    windows: windowList,
  };

  const jsonStr = JSON.stringify(report, null, 2) + '\n';

  if (outFd !== null) {
    fs.writeSync(outFd, jsonStr);
    fs.closeSync(outFd);
  } else {
    process.stdout.write(jsonStr);
  }

  process.exit(0);
}

main().catch((err) => {
  process.stderr.write(`Fatal error: ${err.message}\n`);
  process.exit(1);
});
