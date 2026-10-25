#!/usr/bin/env python3
import argparse
import ipaddress
import json
import os
import sys
import time

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def is_ip_allowed(client_ip_str, allowed_list):
    if not allowed_list:
        return True
    try:
        ip_obj = ipaddress.ip_address(client_ip_str)
    except Exception:
        return False

    for allowed in allowed_list:
        try:
            net = ipaddress.ip_network(allowed, strict=False)
            if ip_obj in net:
                return True
        except Exception:
            continue
    return False

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("keystore_path", nargs="?")
    parser.add_argument("requests_path", nargs="?")
    parser.add_argument("audit_path", nargs="?")
    parser.add_argument("--now-epoch", type=int, default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.keystore_path or not args.requests_path or not args.audit_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.keystore_path, "r", encoding="utf-8") as f:
            keystore = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse keystore JSON: {e}", 2)

    try:
        with open(args.requests_path, "r", encoding="utf-8") as f:
            req_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse requests JSON: {e}", 2)

    if not isinstance(keystore, dict) or "keys" not in keystore or not isinstance(keystore["keys"], dict):
        fail("Keystore JSON must contain 'keys' object", 2)

    if not isinstance(req_data, dict) or "requests" not in req_data or not isinstance(req_data["requests"], list):
        fail("Requests JSON must contain 'requests' array", 2)

    now_epoch = args.now_epoch if args.now_epoch is not None else int(time.time())
    keys = keystore["keys"]

    evaluations = []
    valid_count = 0
    denied_count = 0

    for req in req_data["requests"]:
        if not isinstance(req, dict):
            fail("Request must be an object", 2)

        req_id = req.get("id")
        key_token = req.get("key")
        client_ip = req.get("client_ip")
        required_scope = req.get("required_scope")

        if not isinstance(req_id, str) or not req_id.strip():
            fail("Request missing valid 'id'", 2)
        if not isinstance(key_token, str) or not key_token.strip():
            fail(f"Request '{req_id}' missing valid 'key'", 2)
        if not isinstance(client_ip, str) or not client_ip.strip():
            fail(f"Request '{req_id}' missing valid 'client_ip'", 2)

        # 1. Existence and status
        if key_token not in keys:
            denied_count += 1
            evaluations.append({
                "id": req_id,
                "decision": "DENIED",
                "reason": "key_not_found"
            })
            continue

        key_record = keys[key_token]
        status = key_record.get("status")
        if status == "revoked":
            denied_count += 1
            evaluations.append({
                "id": req_id,
                "decision": "DENIED",
                "reason": "key_revoked"
            })
            continue

        if status != "active":
            denied_count += 1
            evaluations.append({
                "id": req_id,
                "decision": "DENIED",
                "reason": "key_inactive"
            })
            continue

        # 2. Rotation & Grace period
        rotation = key_record.get("rotation")
        rotation_notice = None
        if isinstance(rotation, dict) and "grace_until_epoch" in rotation:
            grace_until = rotation["grace_until_epoch"]
            if isinstance(grace_until, int):
                if now_epoch > grace_until:
                    denied_count += 1
                    evaluations.append({
                        "id": req_id,
                        "decision": "DENIED",
                        "reason": "key_expired_after_rotation"
                    })
                    continue
                else:
                    rotation_notice = "key_rotated_use_new_key"

        # 3. IP Whitelist
        allowed_ips = key_record.get("allowed_ips", [])
        if not is_ip_allowed(client_ip, allowed_ips):
            denied_count += 1
            evaluations.append({
                "id": req_id,
                "decision": "DENIED",
                "reason": "ip_forbidden"
            })
            continue

        # 4. Scope capability check
        if required_scope:
            scopes = set(key_record.get("scopes", []))
            if "*" not in scopes and required_scope not in scopes:
                denied_count += 1
                evaluations.append({
                    "id": req_id,
                    "decision": "DENIED",
                    "reason": "insufficient_scope"
                })
                continue

        # 5. Success
        valid_count += 1
        evaluations.append({
            "id": req_id,
            "decision": "VALID",
            "user_id": key_record.get("user_id"),
            "scope_granted": required_scope if required_scope else None,
            "rotation_notice": rotation_notice
        })

    report = {
        "total_requests": len(evaluations),
        "valid_count": valid_count,
        "denied_count": denied_count,
        "evaluations": evaluations
    }

    try:
        with open(args.audit_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write audit JSON: {e}", 2)

    sys.exit(0 if denied_count == 0 else 1)

if __name__ == "__main__":
    main()
