#!/usr/bin/env node
const fs = require('fs');
const path = require('path');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function main() {
  const args = process.argv.slice(2);
  let rulesPath = null;
  let logDir = null;
  let reportPath = null;
  let dryRun = false;
  let nowEpoch = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--dry-run') {
      dryRun = true;
    } else if (args[i] === '--now-epoch') {
      if (i + 1 >= args.length) fail('Missing value for --now-epoch', 2);
      nowEpoch = parseInt(args[++i], 10);
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!rulesPath) {
      rulesPath = args[i];
    } else if (!logDir) {
      logDir = args[i];
    } else if (!reportPath) {
      reportPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!rulesPath || !logDir || !reportPath) {
    fail('Usage: ./run <rules.json> <log_dir> <report.json> [--dry-run] [--now-epoch <epoch>]', 2);
  }

  let rules;
  try {
    rules = JSON.parse(fs.readFileSync(rulesPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse rules JSON: ${err.message}`, 2);
  }

  if (!rules || typeof rules !== 'object') {
    fail('Rules must be an object', 2);
  }

  if (!fs.existsSync(logDir) || !fs.statSync(logDir).isDirectory()) {
    fail(`Log directory '${logDir}' does not exist or is not a directory`, 2);
  }

  let defaultDays = rules.default_retention_days !== undefined ? rules.default_retention_days : rules.retention_days;
  if (typeof defaultDays !== 'number' || !Number.isInteger(defaultDays) || defaultDays < 0) {
    fail("Missing valid 'default_retention_days' or 'retention_days'", 2);
  }

  const tagRetention = (rules.tag_retention_days && typeof rules.tag_retention_days === 'object') ? rules.tag_retention_days : {};
  const maxStorageBytes = typeof rules.max_storage_bytes === 'number' ? rules.max_storage_bytes : null;

  const currentEpoch = nowEpoch !== null ? nowEpoch : Math.floor(Date.now() / 1000);

  let entries = [];
  try {
    entries = fs.readdirSync(logDir).sort();
  } catch (err) {
    fail(`Failed to list directory: ${err.message}`, 2);
  }

  const scannedFiles = [];
  for (const fname of entries) {
    if (!fname.endsWith('.json')) continue;
    const fpath = path.join(logDir, fname);
    let stat;
    try {
      stat = fs.statSync(fpath);
      if (!stat.isFile()) continue;
    } catch (e) {
      continue;
    }

    let content;
    try {
      content = JSON.parse(fs.readFileSync(fpath, 'utf8'));
    } catch (e) {
      continue;
    }

    const ts = typeof content.timestamp_epoch === 'number' ? content.timestamp_epoch : currentEpoch;
    const tags = Array.isArray(content.tags) ? content.tags.map(String) : [];

    scannedFiles.push({
      filename: fname,
      path: fpath,
      size: stat.size,
      timestamp_epoch: ts,
      tags
    });
  }

  const purged = new Map();
  const surviving = [];

  // 1. Tag-based age expiry
  for (const fileInfo of scannedFiles) {
    const matchingDays = fileInfo.tags
      .filter(t => typeof tagRetention[t] === 'number')
      .map(t => tagRetention[t]);

    const effectiveDays = matchingDays.length > 0 ? Math.max(...matchingDays) : defaultDays;
    const ageSeconds = currentEpoch - fileInfo.timestamp_epoch;

    if (ageSeconds > (effectiveDays * 86400)) {
      purged.set(fileInfo.filename, {
        filename: fileInfo.filename,
        reason: 'age_expired',
        bytes: fileInfo.size
      });
    } else {
      surviving.push(fileInfo);
    }
  }

  // 2. Disk Quota Trimming
  let quotaPurgedCount = 0;
  if (maxStorageBytes !== null) {
    let currentStorage = surviving.reduce((acc, f) => acc + f.size, 0);

    while (currentStorage > maxStorageBytes) {
      const candidates = surviving.filter(f => !f.tags.includes('compliance'));
      if (candidates.length === 0) break;

      candidates.sort((a, b) => {
        if (a.timestamp_epoch !== b.timestamp_epoch) return a.timestamp_epoch - b.timestamp_epoch;
        return a.filename.localeCompare(b.filename);
      });

      const victim = candidates[0];
      purged.set(victim.filename, {
        filename: victim.filename,
        reason: 'quota_exceeded',
        bytes: victim.size
      });

      const idx = surviving.indexOf(victim);
      if (idx !== -1) surviving.splice(idx, 1);

      currentStorage -= victim.size;
      quotaPurgedCount++;
    }
  }

  // Perform deletions if not --dry-run
  if (!dryRun) {
    for (const [fname] of purged) {
      const fpath = path.join(logDir, fname);
      try {
        if (fs.existsSync(fpath)) {
          fs.unlinkSync(fpath);
        }
      } catch (e) {}
    }
  }

  const purgedList = Array.from(purged.values()).sort((a, b) => a.filename.localeCompare(b.filename));
  const agePurgedCount = purgedList.filter(p => p.reason === 'age_expired').length;
  const totalBytesFreed = purgedList.reduce((acc, p) => acc + p.bytes, 0);
  const finalStorageBytes = surviving.reduce((acc, f) => acc + f.size, 0);

  const report = {
    dry_run: dryRun,
    total_scanned: scannedFiles.length,
    age_purged_count: agePurgedCount,
    quota_purged_count: quotaPurgedCount,
    retained_count: surviving.length,
    purged_files: purgedList,
    total_bytes_freed: totalBytesFreed,
    final_storage_bytes: finalStorageBytes
  };

  try {
    fs.writeFileSync(reportPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write report: ${err.message}`, 2);
  }

  const quotaExceeded = (maxStorageBytes !== null && finalStorageBytes > maxStorageBytes);
  process.exit(quotaExceeded ? 1 : 0);
}

main();
