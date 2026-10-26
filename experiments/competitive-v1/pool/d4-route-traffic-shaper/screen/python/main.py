#!/usr/bin/env python3
import hashlib
import json
import os
import sys

def fail(msg, code=2):
    sys.stderr.write(f"Error: {msg}\n")
    sys.exit(code)

def main():
    if len(sys.argv) != 4:
        fail("Usage: ./run <routing_table.json> <requests.json> <dispatch_log.json>", 2)

    tbl_path = sys.argv[1]
    req_path = sys.argv[2]
    log_path = sys.argv[3]

    try:
        with open(tbl_path, "r", encoding="utf-8") as f:
            table_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse routing table JSON: {e}", 2)

    try:
        with open(req_path, "r", encoding="utf-8") as f:
            req_data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse requests JSON: {e}", 2)

    if not isinstance(table_data, dict) or "routes" not in table_data or not isinstance(table_data["routes"], list):
        fail("Routing table must contain 'routes' array", 2)

    if not isinstance(req_data, dict) or "requests" not in req_data or not isinstance(req_data["requests"], list):
        fail("Requests JSON must contain 'requests' array", 2)

    backends_health = table_data.get("backends_health", {})
    if not isinstance(backends_health, dict):
        backends_health = {}

    routes = table_data["routes"]
    dispatches = []
    routed_count = 0
    failed_count = 0

    for req in req_data["requests"]:
        if not isinstance(req, dict):
            fail("Request must be an object", 2)

        req_id = req.get("id")
        req_path_str = req.get("path")
        if not isinstance(req_id, str) or not req_id.strip():
            fail("Request missing valid 'id'", 2)
        if not isinstance(req_path_str, str) or not req_path_str.strip():
            fail(f"Request '{req_id}' missing valid 'path'", 2)

        req_headers = req.get("headers", {})
        if not isinstance(req_headers, dict):
            req_headers = {}
        # Lowercase headers map for matching
        lower_headers = {k.lower(): str(v) for k, v in req_headers.items()}

        matched_route = None
        for r in routes:
            prefix = r.get("path_prefix", "")
            if not req_path_str.startswith(prefix):
                continue

            # Header match conditions
            h_match = r.get("headers_match")
            if isinstance(h_match, dict) and h_match:
                headers_satisfied = True
                for hk, hv in h_match.items():
                    if lower_headers.get(hk.lower()) != str(hv):
                        headers_satisfied = False
                        break
                if not headers_satisfied:
                    continue

            matched_route = r
            break

        if matched_route is None:
            failed_count += 1
            dispatches.append({
                "id": req_id,
                "matched_route": None,
                "selected_target": None,
                "canary_routed": False,
                "fallback_triggered": False,
                "status": "404_not_found"
            })
            continue

        # Target selection (Canary check)
        canary = matched_route.get("canary")
        candidate_target = matched_route.get("target")
        canary_routed = False

        if isinstance(canary, dict) and canary.get("weight_pct", 0) > 0:
            weight_pct = canary["weight_pct"]
            user_id = req.get("user_id")
            client_ip = req.get("client_ip", "127.0.0.1")
            hash_key = str(user_id) if (user_id and str(user_id).strip()) else str(client_ip)

            h_hex = hashlib.sha256(hash_key.encode("utf-8")).hexdigest()
            bucket = int(h_hex[:8], 16) % 100
            if bucket < weight_pct:
                candidate_target = canary.get("target")
                canary_routed = True

        # Health check & Fallback
        is_healthy = backends_health.get(candidate_target, True)
        selected_target = None
        fallback_triggered = False
        status = "503_service_unavailable"

        if is_healthy:
            selected_target = candidate_target
            fallback_triggered = False
            status = "routed"
        else:
            fallback_target = matched_route.get("fallback_target")
            if fallback_target and backends_health.get(fallback_target, True):
                selected_target = fallback_target
                fallback_triggered = True
                status = "routed"
            else:
                selected_target = None
                fallback_triggered = False
                status = "503_service_unavailable"

        if status == "routed":
            routed_count += 1
        else:
            failed_count += 1

        dispatches.append({
            "id": req_id,
            "matched_route": matched_route.get("path_prefix"),
            "selected_target": selected_target,
            "canary_routed": canary_routed,
            "fallback_triggered": fallback_triggered,
            "status": status
        })

    report = {
        "total_requests": len(dispatches),
        "routed_count": routed_count,
        "failed_count": failed_count,
        "dispatches": dispatches
    }

    try:
        with open(log_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write dispatch log: {e}", 2)

    sys.exit(0 if failed_count == 0 else 1)

if __name__ == "__main__":
    main()
