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

def probe_http(url, timeout_ms, fixtures, method="GET", body=None):
    if fixtures is not None:
        if url in fixtures:
            fix = fixtures[url]
            st = fix.get("status", 200)
            b = fix.get("body", {})
            return st, b
        return 404, {"error": "not in fixtures"}

    data_bytes = json.dumps(body).encode("utf-8") if body is not None else None
    req = urllib.request.Request(
        url,
        data=data_bytes,
        method=method,
        headers={"Content-Type": "application/json"} if data_bytes else {}
    )

    try:
        with urllib.request.urlopen(req, timeout=timeout_ms / 1000.0) as resp:
            resp_bytes = resp.read()
            try:
                parsed = json.loads(resp_bytes)
            except Exception:
                parsed = {}
            return resp.status, parsed
    except urllib.error.HTTPError as he:
        resp_bytes = he.read()
        try:
            parsed = json.loads(resp_bytes)
        except Exception:
            parsed = {}
        return he.code, parsed
    except Exception:
        return None, {}

def main():
    parser = argparse.ArgumentParser(add_help=False)
    parser.add_argument("cluster_path", nargs="?")
    parser.add_argument("status_path", nargs="?")
    parser.add_argument("--offline-fixtures", default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.cluster_path or not args.status_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI args: {e}", 2)

    try:
        with open(args.cluster_path, "r", encoding="utf-8") as f:
            cluster = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse cluster JSON: {e}", 2)

    if not isinstance(cluster, dict) or "services" not in cluster or not isinstance(cluster["services"], list):
        fail("Cluster must contain 'services' array", 2)

    fixtures = None
    if args.offline_fixtures:
        try:
            with open(args.offline_fixtures, "r", encoding="utf-8") as f:
                fixtures = json.load(f)
        except Exception as e:
            fail(f"Failed to read fixtures JSON: {e}", 2)

    timeout_ms = cluster.get("timeout_ms", 2000)

    services_results = []
    healthy_count = 0
    failed_over_count = 0
    degraded_count = 0

    for s in cluster["services"]:
        if not isinstance(s, dict):
            fail("Service must be an object", 2)

        name = s.get("name")
        primary_url = s.get("primary_url")
        standby_url = s.get("standby_url")
        activate_url = s.get("activate_url")

        if not isinstance(name, str) or not name.strip():
            fail("Service missing valid 'name'", 2)
        if not isinstance(primary_url, str) or not primary_url.strip():
            fail(f"Service '{name}' missing valid 'primary_url'", 2)
        if not isinstance(standby_url, str) or not standby_url.strip():
            fail(f"Service '{name}' missing valid 'standby_url'", 2)
        if not isinstance(activate_url, str) or not activate_url.strip():
            fail(f"Service '{name}' missing valid 'activate_url'", 2)

        # 1. Primary probe
        p_status, p_body = probe_http(primary_url, timeout_ms, fixtures, method="GET")
        p_is_healthy = (p_status == 200 and isinstance(p_body, dict) and p_body.get("status") == "healthy")

        if p_is_healthy:
            healthy_count += 1
            services_results.append({
                "name": name,
                "state": "primary_active",
                "primary_health": "healthy",
                "standby_health": "not_checked",
                "activated": False
            })
            continue

        # Primary unhealthy -> probe standby
        s_status, s_body = probe_http(standby_url, timeout_ms, fixtures, method="GET")
        s_is_healthy = (s_status == 200 and isinstance(s_body, dict) and s_body.get("status") == "healthy")

        if s_is_healthy:
            # Trigger activation
            act_status, _ = probe_http(activate_url, timeout_ms, fixtures, method="POST", body={})
            if act_status in (200, 204):
                failed_over_count += 1
                services_results.append({
                    "name": name,
                    "state": "failed_over_to_standby",
                    "primary_health": "unhealthy",
                    "standby_health": "healthy",
                    "activated": True
                })
            else:
                degraded_count += 1
                services_results.append({
                    "name": name,
                    "state": "degraded",
                    "primary_health": "unhealthy",
                    "standby_health": "healthy",
                    "activated": False
                })
        else:
            degraded_count += 1
            services_results.append({
                "name": name,
                "state": "degraded",
                "primary_health": "unhealthy",
                "standby_health": "unhealthy",
                "activated": False
            })

    report = {
        "total_services": len(services_results),
        "healthy": healthy_count,
        "failed_over": failed_over_count,
        "degraded": degraded_count,
        "services": services_results
    }

    try:
        with open(args.status_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write status: {e}", 2)

    sys.exit(0 if degraded_count == 0 else 1)

if __name__ == "__main__":
    main()
