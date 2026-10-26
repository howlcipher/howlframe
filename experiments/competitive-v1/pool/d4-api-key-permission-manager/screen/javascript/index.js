#!/usr/bin/env node
const fs = require('fs');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function ipv4ToInt(ipStr) {
  const parts = ipStr.trim().split('.');
  if (parts.length !== 4) return null;
  let val = 0;
  for (let i = 0; i < 4; i++) {
    const n = Number(parts[i]);
    if (isNaN(n) || n < 0 || n > 255) return null;
    val = (val << 8) | n;
  }
  return val >>> 0;
}

function matchesCidrOrIp(clientIpStr, allowedEntry) {
  const clientInt = ipv4ToInt(clientIpStr);
  if (clientInt === null) return false;

  const [netStr, maskStr] = allowedEntry.split('/');
  const netInt = ipv4ToInt(netStr);
  if (netInt === null) return false;

  let maskVal;
  if (maskStr !== undefined) {
    const bits = parseInt(maskStr, 10);
    if (isNaN(bits) || bits < 0 || bits > 32) return false;
    maskVal = bits === 0 ? 0 : ((~0 << (32 - bits)) >>> 0);
  } else {
    maskVal = 0xFFFFFFFF >>> 0;
  }

  return (clientInt & maskVal) === (netInt & maskVal);
}

function isIpAllowed(clientIpStr, allowedList) {
  if (!Array.isArray(allowedList) || allowedList.length === 0) {
    return true;
  }
  for (const entry of allowedList) {
    if (typeof entry === 'string' && matchesCidrOrIp(clientIpStr, entry)) {
      return true;
    }
  }
  return false;
}

function main() {
  const args = process.argv.slice(2);
  let keystorePath = null;
  let requestsPath = null;
  let auditPath = null;
  let nowEpoch = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--now-epoch') {
      if (i + 1 >= args.length) fail('Missing value for --now-epoch', 2);
      nowEpoch = parseInt(args[++i], 10);
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!keystorePath) {
      keystorePath = args[i];
    } else if (!requestsPath) {
      requestsPath = args[i];
    } else if (!auditPath) {
      auditPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!keystorePath || !requestsPath || !auditPath) {
    fail('Usage: ./run <keystore.json> <requests.json> <audit.json> [--now-epoch <epoch>]', 2);
  }

  let keystore, reqData;
  try {
    keystore = JSON.parse(fs.readFileSync(keystorePath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse keystore JSON: ${err.message}`, 2);
  }

  try {
    reqData = JSON.parse(fs.readFileSync(requestsPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse requests JSON: ${err.message}`, 2);
  }

  if (!keystore || typeof keystore !== 'object' || !keystore.keys || typeof keystore.keys !== 'object') {
    fail("Keystore JSON must contain 'keys' object", 2);
  }

  if (!reqData || typeof reqData !== 'object' || !Array.isArray(reqData.requests)) {
    fail("Requests JSON must contain 'requests' array", 2);
  }

  const currentEpoch = nowEpoch !== null ? nowEpoch : Math.floor(Date.now() / 1000);
  const keys = keystore.keys;

  const evaluations = [];
  let validCount = 0;
  let deniedCount = 0;

  for (const req of reqData.requests) {
    if (!req || typeof req !== 'object') {
      fail('Request must be an object', 2);
    }

    const { id: reqId, key: keyToken, client_ip: clientIp, required_scope: requiredScope } = req;

    if (typeof reqId !== 'string' || !reqId.trim()) {
      fail("Request missing valid 'id'", 2);
    }
    if (typeof keyToken !== 'string' || !keyToken.trim()) {
      fail(`Request '${reqId}' missing valid 'key'`, 2);
    }
    if (typeof clientIp !== 'string' || !clientIp.trim()) {
      fail(`Request '${reqId}' missing valid 'client_ip'`, 2);
    }

    // 1. Existence and status
    if (!keys[keyToken]) {
      deniedCount++;
      evaluations.push({
        id: reqId,
        decision: 'DENIED',
        reason: 'key_not_found'
      });
      continue;
    }

    const keyRecord = keys[keyToken];
    const status = keyRecord.status;
    if (status === 'revoked') {
      deniedCount++;
      evaluations.push({
        id: reqId,
        decision: 'DENIED',
        reason: 'key_revoked'
      });
      continue;
    }

    if (status !== 'active') {
      deniedCount++;
      evaluations.push({
        id: reqId,
        decision: 'DENIED',
        reason: 'key_inactive'
      });
      continue;
    }

    // 2. Rotation & Grace period
    let rotationNotice = null;
    if (keyRecord.rotation && typeof keyRecord.rotation === 'object') {
      const graceUntil = keyRecord.rotation.grace_until_epoch;
      if (typeof graceUntil === 'number') {
        if (currentEpoch > graceUntil) {
          deniedCount++;
          evaluations.push({
            id: reqId,
            decision: 'DENIED',
            reason: 'key_expired_after_rotation'
          });
          continue;
        } else {
          rotationNotice = 'key_rotated_use_new_key';
        }
      }
    }

    // 3. IP Whitelist
    if (!isIpAllowed(clientIp, keyRecord.allowed_ips)) {
      deniedCount++;
      evaluations.push({
        id: reqId,
        decision: 'DENIED',
        reason: 'ip_forbidden'
      });
      continue;
    }

    // 4. Scope capability check
    if (requiredScope) {
      const scopes = Array.isArray(keyRecord.scopes) ? keyRecord.scopes : [];
      if (!scopes.includes('*') && !scopes.includes(requiredScope)) {
        deniedCount++;
        evaluations.push({
          id: reqId,
          decision: 'DENIED',
          reason: 'insufficient_scope'
        });
        continue;
      }
    }

    // 5. Valid
    validCount++;
    evaluations.push({
      id: reqId,
      decision: 'VALID',
      user_id: keyRecord.user_id,
      scope_granted: requiredScope || null,
      rotation_notice: rotationNotice
    });
  }

  const report = {
    total_requests: evaluations.length,
    valid_count: validCount,
    denied_count: deniedCount,
    evaluations
  };

  try {
    fs.writeFileSync(auditPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write audit JSON: ${err.message}`, 2);
  }

  process.exit(deniedCount === 0 ? 0 : 1);
}

main();
