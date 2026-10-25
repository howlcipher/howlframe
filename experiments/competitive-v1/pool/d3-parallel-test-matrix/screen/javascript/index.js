#!/usr/bin/env node
const fs = require('fs');
const { spawn } = require('child_process');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function runCaseProcess(c, defaultTimeout) {
  return new Promise((resolve) => {
    const cid = c.id;
    const [bin, ...cmdArgs] = c.command;
    const timeoutMs = typeof c.timeout_ms === 'number' ? c.timeout_ms : defaultTimeout;

    const env = { ...process.env, ...(c.env || {}) };
    const t0 = Date.now();
    let stdoutStr = '';
    let stderrStr = '';
    let timedOut = false;
    let finished = false;

    const proc = spawn(bin, cmdArgs, { env });

    let timer = null;
    if (timeoutMs > 0) {
      timer = setTimeout(() => {
        if (!finished) {
          timedOut = true;
          proc.kill('SIGTERM');
          setTimeout(() => {
            if (!finished) proc.kill('SIGKILL');
          }, 300);
        }
      }, timeoutMs);
    }

    proc.stdout.on('data', d => stdoutStr += d);
    proc.stderr.on('data', d => stderrStr += d);

    proc.on('close', (code) => {
      if (finished) return;
      finished = true;
      if (timer) clearTimeout(timer);
      const durationMs = Math.max(0, Date.now() - t0);

      let status = 'passed';
      if (timedOut) {
        status = 'timed_out';
      } else if (code !== 0) {
        status = 'failed';
      }

      resolve({
        id: cid,
        status,
        exit_code: timedOut ? null : code,
        duration_ms: durationMs,
        stdout: stdoutStr,
        stderr: stderrStr
      });
    });

    proc.on('error', (err) => {
      if (finished) return;
      finished = true;
      if (timer) clearTimeout(timer);
      resolve({
        id: cid,
        status: 'failed',
        exit_code: 1,
        duration_ms: Math.max(0, Date.now() - t0),
        stdout: stdoutStr,
        stderr: stderrStr + (err.message || '')
      });
    });
  });
}

async function main() {
  const args = process.argv.slice(2);
  let matrixPath = null;
  let resultsPath = null;
  let cliConcurrency = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--concurrency') {
      if (i + 1 >= args.length) fail('Missing value for --concurrency', 2);
      cliConcurrency = parseInt(args[++i], 10);
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!matrixPath) {
      matrixPath = args[i];
    } else if (!resultsPath) {
      resultsPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!matrixPath || !resultsPath) {
    fail('Usage: ./run <matrix.json> <results.json> [--concurrency <int>]', 2);
  }

  let matrix;
  try {
    const raw = fs.readFileSync(matrixPath, 'utf8');
    matrix = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse matrix JSON: ${err.message}`, 2);
  }

  if (!matrix || typeof matrix !== 'object' || !Array.isArray(matrix.cases)) {
    fail("Matrix must contain 'cases' array", 2);
  }

  if (matrix.cases.length === 0) {
    fail("'cases' array must not be empty", 2);
  }

  const seenIds = new Set();
  for (const c of matrix.cases) {
    if (!c || typeof c !== 'object') {
      fail('Case must be an object', 2);
    }
    const { id, command } = c;
    if (typeof id !== 'string' || !id.trim()) {
      fail("Case missing valid 'id'", 2);
    }
    if (seenIds.has(id)) {
      fail(`Duplicate case id: ${id}`, 2);
    }
    seenIds.add(id);

    if (!Array.isArray(command) || command.length === 0 || !command.every(x => typeof x === 'string')) {
      fail(`Case '${id}' missing valid 'command' array`, 2);
    }
  }

  const defaultTimeout = typeof matrix.default_timeout_ms === 'number' ? matrix.default_timeout_ms : 5000;
  const failFast = Boolean(matrix.fail_fast);

  let concurrency = cliConcurrency;
  if (concurrency === null) {
    concurrency = typeof matrix.concurrency === 'number' ? matrix.concurrency : 1;
  }
  if (typeof concurrency !== 'number' || !Number.isInteger(concurrency) || concurrency < 1) {
    fail('Concurrency must be >= 1', 2);
  }

  const resultsMap = new Map();
  const queue = [...matrix.cases];
  let aborted = false;

  async function worker() {
    while (queue.length > 0) {
      if (failFast && aborted) {
        break;
      }
      const c = queue.shift();
      const res = await runCaseProcess(c, defaultTimeout);
      resultsMap.set(c.id, res);

      if (failFast && (res.status === 'failed' || res.status === 'timed_out')) {
        aborted = true;
      }
    }
  }

  const numWorkers = Math.min(concurrency, matrix.cases.length);
  const workers = [];
  for (let w = 0; w < numWorkers; w++) {
    workers.push(worker());
  }
  await Promise.all(workers);

  // If aborted, mark remaining queued cases as skipped
  for (const c of queue) {
    resultsMap.set(c.id, {
      id: c.id,
      status: 'skipped',
      reason: 'fail_fast_triggered'
    });
  }

  const casesOutput = [];
  let totalPassed = 0;
  let totalFailed = 0;
  let totalSkipped = 0;

  for (const c of matrix.cases) {
    const res = resultsMap.get(c.id) || {
      id: c.id,
      status: 'skipped',
      reason: 'fail_fast_triggered'
    };
    casesOutput.push(res);
    if (res.status === 'passed') totalPassed++;
    else if (res.status === 'failed' || res.status === 'timed_out') totalFailed++;
    else if (res.status === 'skipped') totalSkipped++;
  }

  const report = {
    total: casesOutput.length,
    passed: totalPassed,
    failed: totalFailed,
    skipped: totalSkipped,
    cases: casesOutput
  };

  try {
    fs.writeFileSync(resultsPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write results: ${err.message}`, 2);
  }

  process.exit(totalPassed === casesOutput.length ? 0 : 1);
}

main();
