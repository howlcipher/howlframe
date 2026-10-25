#!/usr/bin/env python3
import argparse
import json
import os
import sys
import time
import urllib.error
import urllib.request

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("grants_path", nargs="?")
    parser.add_argument("requests_path", nargs="?")
    parser.add_argument("audit_path", nargs="?")
    parser.add_argument("--vault-url", default="http://127.0.0.1:8000")
    parser.add_argument("--offline-vault", action="store_true", default=False)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.grants_path or not args.requests_path or not args.audit_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.grants_path, "r", encoding="utf-8") as f:
            grants = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse grants JSON: {e}", 2)

    try:
        with open(args.requests_path, "r", encoding="utf-8") as f:
            requests_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse requests JSON: {e}", 2)

    if not isinstance(grants, dict) or "subjects" not in grants or not isinstance(grants["subjects"], dict):
        fail("Grants JSON must contain 'subjects' object", 2)

    if not isinstance(requests_data, dict) or "requests" not in requests_data or not isinstance(requests_data["requests"], list):
        fail("Requests JSON must contain 'requests' array", 2)

    vault_base = args.vault_url.rstrip("/")
    dispatches = []
    authorized_dispatched = 0
    denied = 0
    had_dispatch_failure = False

    for req in requests_data["requests"]:
        if not isinstance(req, dict):
            fail("Request must be an object", 2)

        req_id = req.get("id")
        subject = req.get("subject")
        requested_scope = req.get("requested_scope")
        target_url = req.get("target_url")

        if not isinstance(req_id, str) or not req_id.strip():
            fail("Request missing valid 'id'", 2)
        if not isinstance(subject, str) or not subject.strip():
            fail(f"Request '{req_id}' missing valid 'subject'", 2)
        if not isinstance(requested_scope, str) or not requested_scope.strip():
            fail(f"Request '{req_id}' missing valid 'requested_scope'", 2)
        if not isinstance(target_url, str) or not target_url.strip():
            fail(f"Request '{req_id}' missing valid 'target_url'", 2)

        method = req.get("method", "GET").upper()
        req_headers = req.get("headers", {})
        req_body = req.get("body", None)
        timeout_ms = req.get("timeout_ms", 3000)

        # 1. Subject check
        if subject not in grants["subjects"]:
            denied += 1
            dispatches.append({
                "id": req_id,
                "decision": "denied",
                "reason": "unauthorized_subject"
            })
            continue

        subj_conf = grants["subjects"][subject]
        allowed_scopes = subj_conf.get("allowed_scopes", [])
        max_token_ttl = subj_conf.get("max_token_ttl_sec", 300)

        # 2. Scope check
        if requested_scope not in allowed_scopes:
            denied += 1
            dispatches.append({
                "id": req_id,
                "decision": "denied",
                "reason": "scope_not_permitted"
            })
            continue

        # 3. Mint token
        token = None
        if args.offline_vault:
            token = f"vault-mock-{subject}-{requested_scope}"
        else:
            mint_url = f"{vault_base}/v1/token/mint"
            mint_payload = json.dumps({
                "subject": subject,
                "scope": requested_scope,
                "ttl_sec": min(60, max_token_ttl)
            }).encode("utf-8")
            mint_req = urllib.request.Request(
                mint_url,
                data=mint_payload,
                method="POST",
                headers={"Content-Type": "application/json"}
            )
            try:
                with urllib.request.urlopen(mint_req, timeout=3.0) as resp:
                    resp_data = json.loads(resp.read())
                    token = resp_data.get("token")
            except Exception as e:
                # Vault error
                had_dispatch_failure = True
                dispatches.append({
                    "id": req_id,
                    "decision": "authorized",
                    "token_issued": None,
                    "target_status_code": None,
                    "response_body": f"Vault mint error: {e}"
                })
                continue

        # 4. Dispatch downstream
        authorized_dispatched += 1

        if args.offline_vault:
            dispatches.append({
                "id": req_id,
                "decision": "authorized",
                "token_issued": token,
                "target_status_code": 200,
                "response_body": '{"offline": true}'
            })
            continue

        # Sanitize headers: remove authorization or proxy-authorization
        clean_headers = {}
        if isinstance(req_headers, dict):
            for k, v in req_headers.items():
                if k.lower() not in ("authorization", "proxy-authorization") and isinstance(v, str):
                    clean_headers[k] = v
        clean_headers["Authorization"] = f"Bearer {token}"

        body_bytes = None
        if req_body is not None:
            body_bytes = json.dumps(req_body).encode("utf-8")
            clean_headers["Content-Type"] = "application/json"

        target_req = urllib.request.Request(
            target_url,
            data=body_bytes,
            method=method,
            headers=clean_headers
        )

        status_code = None
        resp_text = ""
        try:
            with urllib.request.urlopen(target_req, timeout=timeout_ms / 1000.0) as resp:
                status_code = resp.status
                resp_text = resp.read().decode("utf-8", errors="ignore")
                if status_code >= 500:
                    had_dispatch_failure = True
        except urllib.error.HTTPError as he:
            status_code = he.code
            resp_text = he.read().decode("utf-8", errors="ignore")
            if status_code >= 500:
                had_dispatch_failure = True
        except Exception as e:
            status_code = None
            resp_text = str(e)
            had_dispatch_failure = True

        dispatches.append({
            "id": req_id,
            "decision": "authorized",
            "token_issued": token,
            "target_status_code": status_code,
            "response_body": resp_text
        })

    report = {
        "total_requests": len(dispatches),
        "authorized_dispatched": authorized_dispatched,
        "denied": denied,
        "dispatches": dispatches
    }

    try:
        with open(args.audit_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write audit JSON: {e}", 2)

    sys.exit(1 if had_dispatch_failure else 0)

if __name__ == "__main__":
    main()
