#!/usr/bin/env node
const fs = require('fs');
const path = require('path');
const { spawnSync } = require('child_process');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function main() {
  const args = process.argv.slice(2);
  if (args.length !== 3) {
    fail('Usage: ./run <policy.json> <requests.json> <audit.json>', 2);
  }

  const [policyPath, requestsPath, auditPath] = args;

  let policy;
  try {
    const raw = fs.readFileSync(policyPath, 'utf8');
    policy = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse policy JSON: ${err.message}`, 2);
  }

  let requestsData;
  try {
    const raw = fs.readFileSync(requestsPath, 'utf8');
    requestsData = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse requests JSON: ${err.message}`, 2);
  }

  if (!policy || typeof policy !== 'object' || !policy.roles || typeof policy.roles !== 'object') {
    fail("Policy JSON must contain 'roles' object", 2);
  }

  if (!requestsData || typeof requestsData !== 'object' || !Array.isArray(requestsData.requests)) {
    fail("Requests JSON must contain 'requests' array", 2);
  }

  let jailRoot = null;
  if (policy.jail_root) {
    try {
      jailRoot = fs.realpathSync(path.resolve(policy.jail_root));
    } catch (e) {
      jailRoot = path.resolve(policy.jail_root);
    }
  }

  const executions = [];
  let allowedAndExecuted = 0;
  let denied = 0;
  let hadExecutionFailure = false;

  for (const req of requestsData.requests) {
    if (!req || typeof req !== 'object') {
      fail('Request must be an object', 2);
    }

    const { id: reqId, role: roleName, command } = req;
    if (typeof reqId !== 'string' || !reqId.trim()) {
      fail("Request missing valid 'id'", 2);
    }
    if (typeof roleName !== 'string' || !roleName.trim()) {
      fail(`Request '${reqId}' missing valid 'role'`, 2);
    }
    if (!Array.isArray(command) || command.length === 0 || !command.every(c => typeof c === 'string')) {
      fail(`Request '${reqId}' missing valid 'command' array`, 2);
    }

    // 1. Role existence
    if (!policy.roles[roleName]) {
      denied++;
      executions.push({
        id: reqId,
        decision: 'deny',
        reason: 'unknown_role'
      });
      continue;
    }

    const role = policy.roles[roleName];
    const allowedBins = Array.isArray(role.allowed_binaries) ? role.allowed_binaries : [];
    const disallowedArgs = Array.isArray(role.disallowed_args) ? role.disallowed_args : [];
    const allowEnv = new Set(Array.isArray(role.allow_env) ? role.allow_env : []);
    const maxTimeoutMs = typeof role.max_timeout_ms === 'number' ? role.max_timeout_ms : 5000;
    const maxOutputBytes = typeof role.max_output_bytes === 'number' ? role.max_output_bytes : 1048576;

    // 2. Binary whitelist check
    const binary = command[0];
    if (!allowedBins.includes(binary)) {
      denied++;
      executions.push({
        id: reqId,
        decision: 'deny',
        reason: 'binary_not_permitted'
      });
      continue;
    }

    // 3. Disallowed args check
    let hasForbiddenArg = false;
    for (const arg of command) {
      for (const forbidden of disallowedArgs) {
        if (arg.includes(forbidden)) {
          hasForbiddenArg = true;
          break;
        }
      }
      if (hasForbiddenArg) break;
    }

    if (hasForbiddenArg) {
      denied++;
      executions.push({
        id: reqId,
        decision: 'deny',
        reason: 'forbidden_argument'
      });
      continue;
    }

    // 4. Jail check
    let realWorkDir = process.cwd();
    if (req.working_dir) {
      try {
        realWorkDir = fs.realpathSync(path.resolve(req.working_dir));
      } catch (e) {
        realWorkDir = path.resolve(req.working_dir);
      }
    }

    if (jailRoot !== null) {
      if (realWorkDir !== jailRoot && !realWorkDir.startsWith(jailRoot + path.sep)) {
        denied++;
        executions.push({
          id: reqId,
          decision: 'deny',
          reason: 'jail_violation'
        });
        continue;
      }
    }

    // 5. Env sanitization
    const cleanEnv = { PATH: process.env.PATH || '/usr/bin:/bin' };
    if (req.env && typeof req.env === 'object') {
      for (const [k, v] of Object.entries(req.env)) {
        if (allowEnv.has(k) && typeof v === 'string') {
          cleanEnv[k] = v;
        }
      }
    }

    // 6. Limits and spawn
    const reqTimeout = typeof req.timeout_ms === 'number' ? req.timeout_ms : 5000;
    const effectiveTimeout = Math.min(reqTimeout, maxTimeoutMs);

    allowedAndExecuted++;
    let timedOut = false;
    let exitCode = null;
    let stdoutStr = '';
    let stderrStr = '';
    let truncated = false;
    let status = 'failed';

    const [bin, ...cmdArgs] = command;
    const res = spawnSync(bin, cmdArgs, {
      cwd: realWorkDir,
      env: cleanEnv,
      timeout: effectiveTimeout,
      encoding: 'utf8'
    });

    if (res.error && res.error.code === 'ETIMEDOUT') {
      timedOut = true;
      hadExecutionFailure = true;
      status = 'timed_out';
      stdoutStr = res.stdout || '';
      stderrStr = res.stderr || '';
    } else {
      timedOut = false;
      exitCode = res.status !== null ? res.status : 1;
      stdoutStr = res.stdout || '';
      stderrStr = res.stderr || '';
      if (exitCode === 0) {
        status = 'success';
      } else {
        hadExecutionFailure = true;
      }
    }

    const outBuf = Buffer.from(stdoutStr, 'utf8');
    if (outBuf.length > maxOutputBytes) {
      truncated = true;
      stdoutStr = outBuf.subarray(0, maxOutputBytes).toString('utf8');
    }

    executions.push({
      id: reqId,
      decision: 'allow',
      status,
      exit_code: exitCode,
      stdout: stdoutStr,
      stderr: stderrStr,
      timed_out: timedOut,
      truncated
    });
  }

  const report = {
    total_requests: executions.length,
    allowed_and_executed: allowedAndExecuted,
    denied,
    executions
  };

  try {
    fs.writeFileSync(auditPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write audit report: ${err.message}`, 2);
  }

  process.exit(hadExecutionFailure ? 1 : 0);
}

main();
