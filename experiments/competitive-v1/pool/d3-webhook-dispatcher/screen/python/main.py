#!/usr/bin/env python3
import argparse
import hashlib
import hmac
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
    parser.add_argument("events_path", nargs="?")
    parser.add_argument("report_path", nargs="?")
    parser.add_argument("--endpoint", default="http://127.0.0.1:8080/events")
    parser.add_argument("--secret", default="default-secret")
    parser.add_argument("--offline-status", type=int, default=None)

    try:
        args, extra = parser.parse_known_args()
        if extra or not args.events_path or not args.report_path:
            fail("Missing required arguments", 2)
    except Exception as e:
        fail(f"Invalid CLI arguments: {e}", 2)

    try:
        with open(args.events_path, "r", encoding="utf-8") as f:
            data = json.load(f)
    except Exception as e:
        fail(f"Failed to read/parse events json: {e}", 2)

    if not isinstance(data, dict) or "events" not in data or not isinstance(data["events"], list):
        fail("Input JSON must contain top-level object with 'events' array", 2)

    deliveries = []
    total_delivered = 0
    total_failed = 0

    for ev in data["events"]:
        if not isinstance(ev, dict):
            fail("Event must be an object", 2)
        eid = ev.get("id")
        topic = ev.get("topic")
        if not isinstance(eid, str) or not eid.strip():
            fail("Event missing valid 'id'", 2)
        if not isinstance(topic, str) or not topic.strip():
            fail(f"Event '{eid}' missing valid 'topic'", 2)
        if "payload" not in ev:
            fail(f"Event '{eid}' missing 'payload'", 2)

        max_attempts = ev.get("max_attempts", 3)
        if not isinstance(max_attempts, int) or max_attempts < 1:
            fail(f"Event '{eid}' invalid max_attempts", 2)

        timeout_ms = ev.get("timeout_ms", 2000)
        if not isinstance(timeout_ms, int) or timeout_ms <= 0:
            fail(f"Event '{eid}' invalid timeout_ms", 2)

        body_bytes = json.dumps(ev["payload"], separators=(',', ':')).encode("utf-8")
        sig_hex = hmac.new(args.secret.encode("utf-8"), body_bytes, hashlib.sha256).hexdigest()
        sig_header = f"sha256={sig_hex}"

        attempts = 0
        final_status_code = None
        delivered = False

        for attempt in range(max_attempts):
            attempts += 1
            if args.offline_status is not None:
                final_status_code = args.offline_status
                if 200 <= final_status_code <= 299:
                    delivered = True
                    break
                elif 400 <= final_status_code <= 499:
                    delivered = False
                    break
                else:
                    if attempt + 1 < max_attempts:
                        delay = min(0.05, 0.01 * (attempt + 1))
                        time.sleep(delay)
                    continue

            req = urllib.request.Request(
                args.endpoint,
                data=body_bytes,
                method="POST",
                headers={
                    "Content-Type": "application/json",
                    "X-Event-ID": eid,
                    "X-Event-Topic": topic,
                    "X-Signature": sig_header
                }
            )

            try:
                with urllib.request.urlopen(req, timeout=timeout_ms / 1000.0) as resp:
                    final_status_code = resp.status
                    if 200 <= resp.status <= 299:
                        delivered = True
                        break
            except urllib.error.HTTPError as he:
                final_status_code = he.code
                if 400 <= he.code <= 499:
                    delivered = False
                    break
                # 5xx retries
            except (urllib.error.URLError, TimeoutError, OSError):
                final_status_code = None

            if attempt + 1 < max_attempts:
                delay = min(0.05, 0.01 * (attempt + 1))
                time.sleep(delay)

        status_str = "delivered" if delivered else "failed"
        if delivered:
            total_delivered += 1
        else:
            total_failed += 1

        deliveries.append({
            "id": eid,
            "status": status_str,
            "attempts": attempts,
            "last_status_code": final_status_code,
            "signature": sig_header
        })

    report = {
        "total_events": len(deliveries),
        "delivered": total_delivered,
        "failed": total_failed,
        "deliveries": deliveries
    }

    try:
        with open(args.report_path, "w", encoding="utf-8") as f:
            json.dump(report, f, indent=2)
    except Exception as e:
        fail(f"Failed to write report: {e}", 2)

    sys.exit(0 if total_failed == 0 else 1)

if __name__ == "__main__":
    main()
