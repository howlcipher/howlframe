#!/usr/bin/env python3
import argparse
import json
import os
import sys
import time

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("policy_path", nargs="?")
    parser.add_argument("requests_path", nargs="?")
    parser.add_argument("decisions_path", nargs="?")
    parser.add_argument("--timestamp", type=int, default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.policy_path or not args.requests_path or not args.decisions_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.policy_path, "r", encoding="utf-8") as f:
            policy = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse policy JSON: {e}", 2)

    try:
        with open(args.requests_path, "r", encoding="utf-8") as f:
            req_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse requests JSON: {e}", 2)

    if not isinstance(policy, dict) or "roles" not in policy or not isinstance(policy["roles"], dict):
        fail("Policy JSON must contain 'roles' object", 2)

    if not isinstance(req_data, dict) or "requests" not in req_data or not isinstance(req_data["requests"], list):
        fail("Requests JSON must contain 'requests' array", 2)

    current_epoch = args.timestamp if args.timestamp is not None else int(time.time())
    owner_restricted = set(policy.get("owner_restricted_actions", []))

    decisions = []
    allowed_count = 0
    denied_count = 0

    for req in req_data["requests"]:
        if not isinstance(req, dict):
            fail("Request must be an object", 2)

        req_id = req.get("id")
        user = req.get("user")
        tenant_id = req.get("tenant_id")
        resource_tenant_id = req.get("resource_tenant_id")
        action = req.get("action")
        roles = req.get("roles", [])

        if not isinstance(req_id, str) or not req_id.strip():
            fail("Request missing valid 'id'", 2)
        if not isinstance(user, str) or not user.strip():
            fail(f"Request '{req_id}' missing valid 'user'", 2)
        if not isinstance(tenant_id, str) or not tenant_id.strip():
            fail(f"Request '{req_id}' missing valid 'tenant_id'", 2)
        if not isinstance(resource_tenant_id, str) or not resource_tenant_id.strip():
            fail(f"Request '{req_id}' missing valid 'resource_tenant_id'", 2)
        if not isinstance(action, str) or not action.strip():
            fail(f"Request '{req_id}' missing valid 'action'", 2)
        if not isinstance(roles, list) or not all(isinstance(r, str) for r in roles):
            fail(f"Request '{req_id}' invalid 'roles' list", 2)

        # 1. Tenant Isolation
        if tenant_id != resource_tenant_id:
            denied_count += 1
            decisions.append({
                "id": req_id,
                "decision": "DENY",
                "reason": "tenant_mismatch",
                "effective_roles": sorted(list(set(roles))),
                "delegation_active": False
            })
            continue

        # 2. Delegation processing
        effective_roles_set = set(roles)
        delegation_active = False

        delegation = req.get("delegation")
        if isinstance(delegation, dict):
            del_roles = delegation.get("roles", [])
            valid_until = delegation.get("valid_until_epoch")
            if isinstance(del_roles, list) and isinstance(valid_until, int):
                if current_epoch <= valid_until:
                    delegation_active = True
                    for r in del_roles:
                        if isinstance(r, str):
                            effective_roles_set.add(r)

        effective_roles = sorted(list(effective_roles_set))

        # 3. Permissions check
        granted_permissions = set()
        has_wildcard = False
        for r in effective_roles:
            role_perms = policy["roles"].get(r, [])
            for p in role_perms:
                if p == "*":
                    has_wildcard = True
                granted_permissions.add(p)

        is_permitted = has_wildcard or (action in granted_permissions)
        if not is_permitted:
            denied_count += 1
            decisions.append({
                "id": req_id,
                "decision": "DENY",
                "reason": "insufficient_permissions",
                "effective_roles": effective_roles,
                "delegation_active": delegation_active
            })
            continue

        # 4. Resource ownership check
        if action in owner_restricted:
            if not has_wildcard:
                resource_owner = req.get("resource_owner")
                if not resource_owner or resource_owner != user:
                    denied_count += 1
                    decisions.append({
                        "id": req_id,
                        "decision": "DENY",
                        "reason": "not_resource_owner",
                        "effective_roles": effective_roles,
                        "delegation_active": delegation_active
                    })
                    continue

        allowed_count += 1
        decisions.append({
            "id": req_id,
            "decision": "ALLOW",
            "reason": "authorized",
            "effective_roles": effective_roles,
            "delegation_active": delegation_active
        })

    report = {
        "total_evaluated": len(decisions),
        "allowed_count": allowed_count,
        "denied_count": denied_count,
        "decisions": decisions
    }

    try:
        with open(args.decisions_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write decisions: {e}", 2)

    sys.exit(0 if denied_count == 0 else 1)

if __name__ == "__main__":
    main()
