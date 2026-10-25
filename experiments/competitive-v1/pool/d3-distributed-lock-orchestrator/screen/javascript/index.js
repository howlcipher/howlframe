#!/usr/bin/env node
const fs = require('fs');
const http = require('http');
const { spawn } = require('child_process');
const { URL } = require('url');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function sleep(ms) {
  return new Promise(resolve => setTimeout(resolve, ms));
}

function httpPost(urlStr, data, timeoutMs = 5000) {
  return new Promise((resolve) => {
    let u;
    try {
      u = new URL(urlStr);
    } catch (e) {
      return resolve({ status: null, data: {} });
    }

    const body = Buffer.from(JSON.stringify(data), 'utf8');
    const options = {
      method: 'POST',
      hostname: u.hostname,
      port: u.port || 80,
      path: u.pathname + (u.search || ''),
      headers: {
        'Content-Type': 'application/json',
        'Content-Length': body.length
      },
      timeout: timeoutMs
    };

    const req = http.request(options, (res) => {
      let buf = '';
      res.on('data', d => buf += d);
      res.on('end', () => {
        let parsed = {};
        try {
          parsed = JSON.parse(buf);
        } catch (e) {}
        resolve({ status: res.statusCode, data: parsed });
      });
    });

    req.on('timeout', () => {
      req.destroy();
      resolve({ status: null, data: {} });
    });

    req.on('error', () => {
      resolve({ status: null, data: {} });
    });

    req.write(body);
    req.end();
  });
}

function runSubprocess(command, timeoutMs) {
  return new Promise((resolve) => {
    const [bin, ...cmdArgs] = command;
    let stdoutStr = '';
    let stderrStr = '';
    let timedOut = false;
    let finished = false;

    const proc = spawn(bin, cmdArgs);

    let timer = null;
    if (timeoutMs && timeoutMs > 0) {
      timer = setTimeout(() => {
        if (!finished) {
          timedOut = true;
          proc.kill('SIGTERM');
          setTimeout(() => {
            if (!finished) proc.kill('SIGKILL');
          }, 500);
        }
      }, timeoutMs);
    }

    proc.stdout.on('data', d => stdoutStr += d);
    proc.stderr.on('data', d => stderrStr += d);

    proc.on('close', (code) => {
      if (finished) return;
      finished = true;
      if (timer) clearTimeout(timer);
      resolve({
        exitCode: timedOut ? null : code,
        stdout: stdoutStr,
        stderr: stderrStr,
        timedOut
      });
    });

    proc.on('error', (err) => {
      if (finished) return;
      finished = true;
      if (timer) clearTimeout(timer);
      resolve({
        exitCode: 1,
        stdout: stdoutStr,
        stderr: stderrStr + (err.message || ''),
        timedOut: false
      });
    });
  });
}

async function main() {
  const args = process.argv.slice(2);
  let jobPath = null;
  let resultPath = null;
  let server = 'http://127.0.0.1:8080';
  let offlineMock = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--server') {
      if (i + 1 >= args.length) fail('Missing value for --server', 2);
      server = args[++i];
    } else if (args[i] === '--offline-mock') {
      if (i + 1 >= args.length) fail('Missing value for --offline-mock', 2);
      offlineMock = args[++i];
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!jobPath) {
      jobPath = args[i];
    } else if (!resultPath) {
      resultPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!jobPath || !resultPath) {
    fail('Usage: ./run <job.json> <result.json> [--server <url>] [--offline-mock <mock.json>]', 2);
  }

  let job;
  try {
    const raw = fs.readFileSync(jobPath, 'utf8');
    job = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse job JSON: ${err.message}`, 2);
  }

  if (!job || typeof job !== 'object') {
    fail('Job must be an object', 2);
  }

  const { lock_name, owner_id, command } = job;
  if (typeof lock_name !== 'string' || !lock_name.trim()) {
    fail("Missing valid 'lock_name'", 2);
  }
  if (typeof owner_id !== 'string' || !owner_id.trim()) {
    fail("Missing valid 'owner_id'", 2);
  }
  if (!Array.isArray(command) || command.length === 0 || !command.every(c => typeof c === 'string')) {
    fail("Missing valid 'command' array of strings", 2);
  }

  const lease_ttl_ms = typeof job.lease_ttl_ms === 'number' ? job.lease_ttl_ms : 2000;
  const renew_interval_ms = typeof job.renew_interval_ms === 'number' ? job.renew_interval_ms : 500;
  const max_acquire_wait_ms = typeof job.max_acquire_wait_ms === 'number' ? job.max_acquire_wait_ms : 3000;
  const timeout_ms = typeof job.timeout_ms === 'number' ? job.timeout_ms : 10000;

  const serverBase = server.replace(/\/+$/, '');
  let acquired = false;
  let token = null;

  if (offlineMock) {
    try {
      const mockRaw = fs.readFileSync(offlineMock, 'utf8');
      const mockData = JSON.parse(mockRaw);
      if (mockData.acquire_status === 200) {
        acquired = true;
        token = mockData.token || 'mock-token-offline';
      } else {
        acquired = false;
      }
    } catch (e) {
      fail(`Failed reading mock: ${e.message}`, 2);
    }
  } else {
    const deadline = Date.now() + max_acquire_wait_ms;
    while (Date.now() <= deadline) {
      const res = await httpPost(`${serverBase}/lock/acquire`, {
        lock_name,
        owner_id,
        ttl_ms: lease_ttl_ms
      });
      if (res.status === 200 && (res.data.acquired === undefined || res.data.acquired === true)) {
        acquired = true;
        token = res.data.token || `tok-${lock_name}`;
        break;
      }
      await sleep(50);
    }
  }

  if (!acquired) {
    const report = {
      lock_acquired: false,
      token: null,
      renew_count: 0,
      command_executed: false,
      exit_code: null,
      stdout: '',
      stderr: '',
      lock_released: false
    };
    try {
      fs.writeFileSync(resultPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
    } catch (e) {
      fail(`Failed to write result: ${e.message}`, 2);
    }
    process.exit(1);
  }

  // Heartbeat renew interval
  let renewCount = 0;
  let running = true;

  const renewTimer = setInterval(async () => {
    if (!running) return;
    if (!offlineMock) {
      const res = await httpPost(`${serverBase}/lock/renew`, {
        lock_name,
        owner_id,
        token,
        ttl_ms: lease_ttl_ms
      });
      if (res.status === 200) {
        renewCount++;
      }
    } else {
      renewCount++;
    }
  }, renew_interval_ms);

  const procRes = await runSubprocess(command, timeout_ms);

  running = false;
  clearInterval(renewTimer);

  // Release lock
  let released = false;
  if (!offlineMock) {
    const relRes = await httpPost(`${serverBase}/lock/release`, {
      lock_name,
      owner_id,
      token
    });
    released = (relRes.status === 200);
  } else {
    released = true;
  }

  const report = {
    lock_acquired: true,
    token,
    renew_count: renewCount,
    command_executed: true,
    exit_code: procRes.exitCode,
    stdout: procRes.stdout,
    stderr: procRes.stderr,
    lock_released: released
  };

  try {
    fs.writeFileSync(resultPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (e) {
    fail(`Failed to write result: ${e.message}`, 2);
  }

  process.exit(procRes.exitCode === 0 && !procRes.timedOut ? 0 : 1);
}

main();
