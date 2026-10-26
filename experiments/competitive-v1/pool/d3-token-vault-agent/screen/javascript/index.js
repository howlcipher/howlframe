#!/usr/bin/env node
const fs = require('fs');
const http = require('http');
const { URL } = require('url');

function fail(msg, code = 2) {
  process.stderr.write(`Error: ${msg}\n`);
  process.exit(code);
}

function doHttpRequest(urlStr, method, headers, bodyBytes, timeoutMs = 5000) {
  return new Promise((resolve) => {
    let u;
    try {
      u = new URL(urlStr);
    } catch (e) {
      return resolve({ status: null, body: e.message, error: e });
    }

    const options = {
      method,
      hostname: u.hostname,
      port: u.port || 80,
      path: u.pathname + (u.search || ''),
      headers: {
        ...headers,
        ...(bodyBytes ? { 'Content-Length': bodyBytes.length } : {})
      },
      timeout: timeoutMs
    };

    const req = http.request(options, (res) => {
      let data = '';
      res.on('data', d => data += d);
      res.on('end', () => {
        resolve({ status: res.statusCode, body: data, error: null });
      });
    });

    req.on('timeout', () => {
      req.destroy();
      resolve({ status: null, body: 'timeout', error: new Error('timeout') });
    });

    req.on('error', (err) => {
      resolve({ status: null, body: err.message, error: err });
    });

    if (bodyBytes) {
      req.write(bodyBytes);
    }
    req.end();
  });
}

async function main() {
  const args = process.argv.slice(2);
  let grantsPath = null;
  let requestsPath = null;
  let auditPath = null;
  let vaultUrl = 'http://127.0.0.1:8000';
  let offlineVault = false;

  for (let i = 0; i < args.length; i++) {
    if (args[i] === '--vault-url') {
      if (i + 1 >= args.length) fail('Missing value for --vault-url', 2);
      vaultUrl = args[++i];
    } else if (args[i] === '--offline-vault') {
      offlineVault = true;
    } else if (args[i].startsWith('--')) {
      fail(`Unknown flag: ${args[i]}`, 2);
    } else if (!grantsPath) {
      grantsPath = args[i];
    } else if (!requestsPath) {
      requestsPath = args[i];
    } else if (!auditPath) {
      auditPath = args[i];
    } else {
      fail(`Unexpected argument: ${args[i]}`, 2);
    }
  }

  if (!grantsPath || !requestsPath || !auditPath) {
    fail('Usage: ./run <grants.json> <requests.json> <audit.json> [--vault-url <url>] [--offline-vault]', 2);
  }

  let grants;
  try {
    const raw = fs.readFileSync(grantsPath, 'utf8');
    grants = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse grants JSON: ${err.message}`, 2);
  }

  let requestsData;
  try {
    const raw = fs.readFileSync(requestsPath, 'utf8');
    requestsData = JSON.parse(raw);
  } catch (err) {
    fail(`Failed to read/parse requests JSON: ${err.message}`, 2);
  }

  if (!grants || typeof grants !== 'object' || !grants.subjects || typeof grants.subjects !== 'object') {
    fail("Grants JSON must contain 'subjects' object", 2);
  }

  if (!requestsData || typeof requestsData !== 'object' || !Array.isArray(requestsData.requests)) {
    fail("Requests JSON must contain 'requests' array", 2);
  }

  const vaultBase = vaultUrl.replace(/\/+$/, '');
  const dispatches = [];
  let authorizedDispatched = 0;
  let denied = 0;
  let hadDispatchFailure = false;

  for (const req of requestsData.requests) {
    if (!req || typeof req !== 'object') {
      fail('Request must be an object', 2);
    }

    const { id: reqId, subject, requested_scope, target_url } = req;
    if (typeof reqId !== 'string' || !reqId.trim()) {
      fail("Request missing valid 'id'", 2);
    }
    if (typeof subject !== 'string' || !subject.trim()) {
      fail(`Request '${reqId}' missing valid 'subject'`, 2);
    }
    if (typeof requested_scope !== 'string' || !requested_scope.trim()) {
      fail(`Request '${reqId}' missing valid 'requested_scope'`, 2);
    }
    if (typeof target_url !== 'string' || !target_url.trim()) {
      fail(`Request '${reqId}' missing valid 'target_url'`, 2);
    }

    const method = typeof req.method === 'string' ? req.method.toUpperCase() : 'GET';
    const reqHeaders = req.headers || {};
    const reqBody = req.body !== undefined ? req.body : null;
    const timeoutMs = typeof req.timeout_ms === 'number' ? req.timeout_ms : 3000;

    // 1. Subject check
    if (!grants.subjects[subject]) {
      denied++;
      dispatches.push({
        id: reqId,
        decision: 'denied',
        reason: 'unauthorized_subject'
      });
      continue;
    }

    const subjConf = grants.subjects[subject];
    const allowedScopes = Array.isArray(subjConf.allowed_scopes) ? subjConf.allowed_scopes : [];
    const maxTokenTtl = typeof subjConf.max_token_ttl_sec === 'number' ? subjConf.max_token_ttl_sec : 300;

    // 2. Scope check
    if (!allowedScopes.includes(requested_scope)) {
      denied++;
      dispatches.push({
        id: reqId,
        decision: 'denied',
        reason: 'scope_not_permitted'
      });
      continue;
    }

    // 3. Mint token
    let token = null;
    if (offlineVault) {
      token = `vault-mock-${subject}-${requested_scope}`;
    } else {
      const mintUrl = `${vaultBase}/v1/token/mint`;
      const mintPayload = Buffer.from(JSON.stringify({
        subject,
        scope: requested_scope,
        ttl_sec: Math.min(60, maxTokenTtl)
      }), 'utf8');

      const mintRes = await doHttpRequest(mintUrl, 'POST', { 'Content-Type': 'application/json' }, mintPayload, 3000);
      try {
        const parsed = JSON.parse(mintRes.body);
        token = parsed.token;
      } catch (e) {
        hadDispatchFailure = true;
        dispatches.push({
          id: reqId,
          decision: 'authorized',
          token_issued: null,
          target_status_code: null,
          response_body: `Vault mint error: ${mintRes.body}`
        });
        continue;
      }
    }

    // 4. Dispatch downstream
    authorizedDispatched++;

    if (offlineVault) {
      dispatches.push({
        id: reqId,
        decision: 'authorized',
        token_issued: token,
        target_status_code: 200,
        response_body: '{"offline": true}'
      });
      continue;
    }

    // Scrub headers
    const cleanHeaders = {};
    if (reqHeaders && typeof reqHeaders === 'object') {
      for (const [k, v] of Object.entries(reqHeaders)) {
        if (!['authorization', 'proxy-authorization'].includes(k.toLowerCase()) && typeof v === 'string') {
          cleanHeaders[k] = v;
        }
      }
    }
    cleanHeaders['Authorization'] = `Bearer ${token}`;

    let bodyBytes = null;
    if (reqBody !== null) {
      bodyBytes = Buffer.from(JSON.stringify(reqBody), 'utf8');
      cleanHeaders['Content-Type'] = 'application/json';
    }

    const targetRes = await doHttpRequest(target_url, method, cleanHeaders, bodyBytes, timeoutMs);
    if (targetRes.status === null || targetRes.status >= 500) {
      hadDispatchFailure = true;
    }

    dispatches.push({
      id: reqId,
      decision: 'authorized',
      token_issued: token,
      target_status_code: targetRes.status,
      response_body: targetRes.body
    });
  }

  const report = {
    total_requests: dispatches.length,
    authorized_dispatched: authorizedDispatched,
    denied,
    dispatches
  };

  try {
    fs.writeFileSync(auditPath, JSON.stringify(report, null, 2) + '\n', 'utf8');
  } catch (err) {
    fail(`Failed to write audit JSON: ${err.message}`, 2);
  }

  process.exit(hadDispatchFailure ? 1 : 0);
}

main();
