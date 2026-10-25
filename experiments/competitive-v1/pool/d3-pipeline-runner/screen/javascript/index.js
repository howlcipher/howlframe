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
  if (args.length !== 2) {
    fail('Usage: ./run <pipeline.json> <report.json>', 2);
  }

  const [pipelinePath, reportPath] = args;

  let data;
  try {
    const raw = fs.readFileSync(pipelinePath, 'utf8');
    data = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse pipeline json: ${err.message}`, 2);
  }

  if (!data || typeof data !== 'object' || !Array.isArray(data.tasks)) {
    fail("Input JSON must have top-level object with 'tasks' array", 2);
  }

  const taskMap = new Map();
  for (const item of data.tasks) {
    if (!item || typeof item !== 'object') {
      fail('Task must be an object', 2);
    }
    const { id } = item;
    if (typeof id !== 'string' || !id.trim()) {
      fail("Task 'id' must be a non-empty string", 2);
    }
    if (taskMap.has(id)) {
      fail(`Duplicate task id: ${id}`, 2);
    }

    const { command } = item;
    if (!Array.isArray(command) || command.length === 0 || !command.every(c => typeof c === 'string')) {
      fail(`Task '${id}' must have a non-empty 'command' array of strings`, 2);
    }

    const timeout_ms = item.timeout_ms !== undefined ? item.timeout_ms : 5000;
    if (typeof timeout_ms !== 'number' || !Number.isInteger(timeout_ms) || timeout_ms <= 0) {
      fail(`Task '${id}' invalid timeout_ms`, 2);
    }

    const max_retries = item.max_retries !== undefined ? item.max_retries : 0;
    if (typeof max_retries !== 'number' || !Number.isInteger(max_retries) || max_retries < 0) {
      fail(`Task '${id}' invalid max_retries`, 2);
    }

    const depends_on = item.depends_on !== undefined ? item.depends_on : [];
    if (!Array.isArray(depends_on) || !depends_on.every(d => typeof d === 'string')) {
      fail(`Task '${id}' invalid depends_on`, 2);
    }

    taskMap.set(id, {
      id,
      command,
      timeout_ms,
      max_retries,
      depends_on
    });
  }

  // Validate dependencies exist
  for (const [id, t] of taskMap.entries()) {
    for (const dep of t.depends_on) {
      if (!taskMap.has(dep)) {
        fail(`Task '${id}' depends on unknown task '${dep}'`, 2);
      }
    }
  }

  // Cycle detection
  const visited = new Map(); // id -> 0 (visiting), 1 (visited)
  function dfs(node) {
    visited.set(node, 0);
    const t = taskMap.get(node);
    for (const dep of t.depends_on) {
      if (visited.get(dep) === 0) {
        fail(`Cycle detected involving '${node}' and '${dep}'`, 2);
      }
      if (!visited.has(dep)) {
        dfs(dep);
      }
    }
    visited.set(node, 1);
  }

  for (const id of taskMap.keys()) {
    if (!visited.has(id)) {
      dfs(id);
    }
  }

  const results = {};
  const pending = new Set(taskMap.keys());

  while (pending.size > 0) {
    const ready = [];
    const pendingArr = Array.from(pending).sort();
    for (const id of pendingArr) {
      const t = taskMap.get(id);
      if (t.depends_on.every(d => d in results)) {
        ready.push(id);
      }
    }

    if (ready.length === 0) {
      fail('Deadlock or unresolved dependencies in pipeline', 2);
    }

    for (const id of ready) {
      const t = taskMap.get(id);
      const depFailed = t.depends_on.some(d => results[d].status === 'failed' || results[d].status === 'skipped');
      if (depFailed) {
        results[id] = {
          status: 'skipped',
          reason: 'dependency_failed'
        };
        pending.delete(id);
        continue;
      }

      let attempts = 0;
      const maxAttempts = 1 + t.max_retries;
      let finalStatus = 'failed';
      let finalCode = null;
      let finalTimeout = false;
      let finalOut = '';
      let finalErr = '';

      for (let attempt = 0; attempt < maxAttempts; attempt++) {
        attempts++;
        const [bin, ...cmdArgs] = t.command;
        const res = spawnSync(bin, cmdArgs, {
          timeout: t.timeout_ms,
          encoding: 'utf8'
        });

        if (res.error && res.error.code === 'ETIMEDOUT') {
          finalTimeout = true;
          finalCode = null;
          finalOut = res.stdout || '';
          finalErr = res.stderr || '';
        } else {
          finalTimeout = false;
          finalCode = res.status !== null ? res.status : 1;
          finalOut = res.stdout || '';
          finalErr = res.stderr || '';
          if (res.status === 0) {
            finalStatus = 'success';
            break;
          }
        }
      }

      results[id] = {
        status: finalStatus,
        attempts,
        exit_code: finalCode,
        timed_out: finalTimeout,
        stdout: finalOut,
        stderr: finalErr
      };
      pending.delete(id);
    }
  }

  const total_tasks = taskMap.size;
  const completed_tasks = Object.values(results).filter(r => r.status === 'success').length;
  const failed_tasks = Object.values(results).filter(r => r.status === 'failed').length;
  const skipped_tasks = Object.values(results).filter(r => r.status === 'skipped').length;
  const all_success = completed_tasks === total_tasks;

  const sortedTasks = {};
  for (const k of Object.keys(results).sort()) {
    sortedTasks[k] = results[k];
  }

  const report = {
    success: all_success,
    total_tasks,
    completed_tasks,
    failed_tasks,
    skipped_tasks,
    tasks: sortedTasks
  };

  try {
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write report: ${err.message}`, 2);
  }

  process.exit(all_success ? 0 : 1);
}

main();
