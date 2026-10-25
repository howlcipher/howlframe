#!/usr/bin/env node
const fs = require('fs');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function main() {
  const args = process.argv.slice(2);
  let policyPath = null;
  let requestsPath = null;
  let decisionsPath = null;
  let timestamp = null;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--timestamp') {
      if (i + 1 >= args.length) fail('Missing value for --timestamp', 2);
      timestamp = parseInt(args[++i], 10);
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!policyPath) {
      policyPath = args[i];
    } else if (!requestsPath) {
      requestsPath = args[i];
    } else if (!decisionsPath) {
      decisionsPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!policyPath || !requestsPath || !decisionsPath) {
    fail('Usage: ./run <policy.json> <requests.json> <decisions.json> [--timestamp <epoch>]', 2);
  }

  let policy, reqData;
  try {
    policy = JSON.parse(fs.readFileSync(policyPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse policy JSON: ${err.message}`, 2);
  }

  try {
    reqData = JSON.parse(fs.readFileSync(requestsPath, 'utf8'));
  } catch (err) {
    fail(`Failed to read/parse requests JSON: ${err.message}`, 2);
  }

  if (!policy || typeof policy !== 'object' || !policy.roles || typeof policy.roles !== 'object') {
    fail("Policy JSON must contain 'roles' object", 2);
  }

  if (!reqData || typeof reqData !== 'object' || !Array.isArray(reqData.requests)) {
    fail("Requests JSON must contain 'requests' array", 2);
  }

  const currentEpoch = timestamp !== null ? timestamp : Math.floor(Date.now() / 1000);
  const ownerRestricted = new Set(Array.isArray(policy.owner_restricted_actions) ? policy.owner_restricted_actions : []);

  const decisions = [];
  let allowedCount = 0;
  let deniedCount = 0;

  for (const req of reqData.requests) {
    if (!req || typeof req !== 'object') {
      fail('Request must be an object', 2);
    }

    const { id: reqId, user, tenant_id, resource_tenant_id, action } = req;
    const roles = Array.isArray(req.roles) ? req.roles : [];

    if (typeof reqId !== 'string' || !reqId.trim()) {
      fail("Request missing valid 'id'", 2);
    }
    if (typeof user !== 'string' || !user.trim()) {
      fail(`Request '${reqId}' missing valid 'user'`, 2);
    }
    if (typeof tenant_id !== 'string' || !tenant_id.trim()) {
      fail(`Request '${reqId}' missing valid 'tenant_id'`, 2);
    }
    if (typeof resource_tenant_id !== 'string' || !resource_tenant_id.trim()) {
      fail(`Request '${reqId}' missing valid 'resource_tenant_id'`, 2);
    }
    if (typeof action !== 'string' || !action.trim()) {
      fail(`Request '${reqId}' missing valid 'action'`, 2);
    }
    if (!roles.every(r => typeof r === 'string')) {
      fail(`Request '${reqId}' invalid 'roles' array`, 2);
    }

    // 1. Tenant Isolation
    if (tenant_id !== resource_tenant_id) {
      deniedCount++;
      decisions.push({
        id: reqId,
        decision: 'DENY',
        reason: 'tenant_mismatch',
        effective_roles: Array.from(new Set(roles)).sort(),
        delegation_active: false
      });
      continue;
    }

    // 2. Delegation processing
    const effectiveRolesSet = new Set(roles);
    let delegationActive = false;

    if (req.delegation && typeof req.delegation === 'object') {
      const { roles: delRoles, valid_until_epoch } = req.delegation;
      if (Array.isArray(delRoles) && typeof valid_until_epoch === 'number') {
        if (currentEpoch <= valid_until_epoch) {
          delegationActive = true;
          for (const r of delRoles) {
            if (typeof r === 'string') effectiveRolesSet.add(r);
          }
        }
      }
    }

    const effectiveRoles = Array.from(effectiveRolesSet).sort();

    // 3. Permissions check
    const grantedPermissions = new Set();
    let hasWildcard = false;

    for (const r of effectiveRoles) {
      const rolePerms = policy.roles[r] || [];
      for (const p of rolePerms) {
        if (p === '*') hasWildcard = true;
        grantedPermissions.add(p);
      }
    }

    const isPermitted = hasWildcard || grantedPermissions.has(action);
    if (!isPermitted) {
      deniedCount++;
      decisions.push({
        id: reqId,
        decision: 'DENY',
        reason: 'insufficient_permissions',
        effective_roles: effectiveRoles,
        delegation_active: delegationActive
      });
      continue;
    }

    // 4. Resource ownership check
    if (ownerRestricted.has(action)) {
      if (!hasWildcard) {
        const resourceOwner = req.resource_owner;
        if (!resourceOwner || resourceOwner !== user) {
          deniedCount++;
          decisions.push({
            id: reqId,
            decision: 'DENY',
            reason: 'not_resource_owner',
            effective_roles: effectiveRoles,
            delegation_active: delegationActive
          });
          continue;
        }
      }
    }

    allowedCount++;
    decisions.push({
      id: reqId,
      decision: 'ALLOW',
      reason: 'authorized',
      effective_roles: effectiveRoles,
      delegation_active: delegationActive
    });
  }

  const report = {
    total_evaluated: decisions.length,
    allowed_count: allowedCount,
    denied_count: deniedCount,
    decisions
  };

  try {
    fs.writeFileSync(decisionsPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write decisions: ${err.message}`, 2);
  }

  process.exit(deniedCount === 0 ? 0 : 1);
}

main();
