#!/usr/bin/env python3
"""Reference implementation in Python for calib-apilog-analyzer."""

import json
import math
import os
import sys

ALLOWED_METHODS = {"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}


def parse_args():
    input_path = None
    output_path = None
    sla_latency_ms = 500
    error_rate_threshold = 0.05
    window_seconds = 300

    args = sys.argv[1:]
    i = 0
    while i < len(args):
        arg = args[i]
        key = arg
        val = None

        if arg.startswith("--") and "=" in arg:
            key, val = arg.split("=", 1)
        elif arg.startswith("--"):
            key = arg
            if i + 1 < len(args):
                val = args[i + 1]
                i += 1
            else:
                sys.stderr.write(f"Error: Missing value for option {key}\n")
                sys.exit(1)
        else:
            sys.stderr.write(f"Error: Unrecognized argument: {arg}\n")
            sys.exit(1)

        if key == "--input":
            input_path = val
        elif key == "--output":
            output_path = val
        elif key == "--sla-latency-ms":
            try:
                sla_latency_ms = int(val)
                if sla_latency_ms < 0:
                    raise ValueError
            except (ValueError, TypeError):
                sys.stderr.write(
                    f"Error: --sla-latency-ms must be a non-negative integer, got '{val}'\n"
                )
                sys.exit(1)
        elif key == "--error-rate-threshold":
            try:
                error_rate_threshold = float(val)
                if not (0.0 <= error_rate_threshold <= 1.0) or math.isnan(
                    error_rate_threshold
                ):
                    raise ValueError
            except (ValueError, TypeError):
                sys.stderr.write(
                    f"Error: --error-rate-threshold must be between 0.0 and 1.0, got '{val}'\n"
                )
                sys.exit(1)
        elif key == "--window-seconds":
            try:
                window_seconds = int(val)
                if window_seconds <= 0:
                    raise ValueError
            except (ValueError, TypeError):
                sys.stderr.write(
                    f"Error: --window-seconds must be a positive integer, got '{val}'\n"
                )
                sys.exit(1)
        else:
            sys.stderr.write(f"Error: Unknown option {key}\n")
            sys.exit(1)

        i += 1

    return (
        input_path,
        output_path,
        sla_latency_ms,
        error_rate_threshold,
        window_seconds,
    )


MAX_SAFE_INTEGER = 2**53 - 1


def open_input_stream(input_path):
    if input_path is None or input_path == "-":
        return sys.stdin, False
    if os.path.isdir(input_path):
        sys.stderr.write(
            f"Error: Cannot open input file '{input_path}': path is a directory\n"
        )
        sys.exit(2)
    try:
        f = open(input_path, "r", encoding="utf-8")
        return f, True
    except OSError as e:
        sys.stderr.write(
            f"Error: Cannot open input file '{input_path}': {e}\n"
        )
        sys.exit(2)


def open_output_stream(output_path):
    if output_path is None or output_path == "-":
        return sys.stdout, False
    try:
        f = open(output_path, "w", encoding="utf-8")
        return f, True
    except OSError as e:
        sys.stderr.write(
            f"Error: Cannot open output file '{output_path}': {e}\n"
        )
        sys.exit(1)


def is_valid_int(val, min_val=None, max_val=None):
    if isinstance(val, bool) or not isinstance(val, (int, float)):
        return False
    if math.isnan(val) or math.isinf(val):
        return False
    if val != math.floor(val):
        return False
    if val > MAX_SAFE_INTEGER:
        return False
    if min_val is not None and val < min_val:
        return False
    if max_val is not None and val > max_val:
        return False
    return True


def main():
    (
        input_path,
        output_path,
        sla_latency_ms,
        error_rate_threshold,
        window_seconds,
    ) = parse_args()

    in_f, close_in = open_input_stream(input_path)
    out_f, close_out = open_output_stream(output_path)

    total_records = 0
    valid_records = 0
    invalid_records = 0
    earliest_timestamp = None
    latest_timestamp = None
    total_bytes_sent = 0

    services = {}
    clients = {}
    windows = {}

    try:
        for line in in_f:
            stripped = line.strip()
            if not stripped:
                continue

            total_records += 1

            try:
                record = json.loads(stripped)
            except Exception:
                invalid_records += 1
                continue

            if not isinstance(record, dict):
                invalid_records += 1
                continue

            ts = record.get("timestamp")
            svc = record.get("service")
            cid = record.get("client_id")
            method = record.get("method")
            path = record.get("path")
            status = record.get("status")
            lat = record.get("latency_ms")
            bytes_sent = record.get("bytes_sent")

            if (
                not is_valid_int(ts, min_val=0)
                or not isinstance(svc, str)
                or len(svc) == 0
                or not isinstance(cid, str)
                or len(cid) == 0
                or not isinstance(method, str)
                or method not in ALLOWED_METHODS
                or not isinstance(path, str)
                or not path.startswith("/")
                or not is_valid_int(status, min_val=100, max_val=599)
                or not is_valid_int(lat, min_val=0)
                or not is_valid_int(bytes_sent, min_val=0)
            ):
                invalid_records += 1
                continue

            ts = int(ts)
            status = int(status)
            lat = int(lat)
            bytes_sent = int(bytes_sent)

            valid_records += 1

            if earliest_timestamp is None or ts < earliest_timestamp:
                earliest_timestamp = ts
            if latest_timestamp is None or ts > latest_timestamp:
                latest_timestamp = ts
            total_bytes_sent += bytes_sent

            # Service aggregation
            if svc not in services:
                services[svc] = {
                    "total": 0,
                    "success": 0,
                    "client_err": 0,
                    "server_err": 0,
                    "sum_lat": 0,
                    "min_lat": lat,
                    "max_lat": lat,
                    "total_bytes": 0,
                }
            s_entry = services[svc]
            s_entry["total"] += 1
            if 200 <= status < 400:
                s_entry["success"] += 1
            elif 400 <= status < 500:
                s_entry["client_err"] += 1
            elif status >= 500:
                s_entry["server_err"] += 1
            s_entry["sum_lat"] += lat
            if lat < s_entry["min_lat"]:
                s_entry["min_lat"] = lat
            if lat > s_entry["max_lat"]:
                s_entry["max_lat"] = lat
            s_entry["total_bytes"] += bytes_sent

            # Client aggregation
            if cid not in clients:
                clients[cid] = {"count": 0, "bytes": 0, "errors": 0}
            c_entry = clients[cid]
            c_entry["count"] += 1
            c_entry["bytes"] += bytes_sent
            if status >= 400:
                c_entry["errors"] += 1

            # Window aggregation
            w_start = ts - (ts % window_seconds)
            if w_start not in windows:
                windows[w_start] = {"total": 0, "errors": 0, "sum_lat": 0}
            w_entry = windows[w_start]
            w_entry["total"] += 1
            if status >= 400:
                w_entry["errors"] += 1
            w_entry["sum_lat"] += lat

    finally:
        if close_in:
            in_f.close()

    # Compile service list
    svc_list = []
    services_with_sla_breach = 0
    for s_name in sorted(services.keys()):
        s = services[s_name]
        tot = s["total"]
        err_count = s["client_err"] + s["server_err"]
        err_rate = round(err_count / tot, 4) if tot > 0 else 0.0
        avg_lat = round(s["sum_lat"] / tot, 2) if tot > 0 else 0.0
        breach = (err_rate > error_rate_threshold) or (avg_lat > sla_latency_ms)
        if breach:
            services_with_sla_breach += 1
        svc_list.append(
            {
                "service": s_name,
                "total_requests": tot,
                "successful_requests": s["success"],
                "client_error_requests": s["client_err"],
                "server_error_requests": s["server_err"],
                "error_rate": err_rate,
                "min_latency_ms": s["min_lat"],
                "max_latency_ms": s["max_lat"],
                "avg_latency_ms": avg_lat,
                "total_bytes_sent": s["total_bytes"],
                "sla_breach": breach,
            }
        )

    # Compile top 10 clients
    top_clients = []
    sorted_client_keys = sorted(
        clients.keys(), key=lambda cid: (-clients[cid]["count"], cid)
    )
    for cid in sorted_client_keys[:10]:
        c = clients[cid]
        top_clients.append(
            {
                "client_id": cid,
                "request_count": c["count"],
                "total_bytes": c["bytes"],
                "error_count": c["errors"],
            }
        )

    # Compile windows
    window_list = []
    for w_start in sorted(windows.keys()):
        w = windows[w_start]
        tot_w = w["total"]
        window_list.append(
            {
                "window_start": w_start,
                "window_end": w_start + window_seconds,
                "total_requests": tot_w,
                "error_requests": w["errors"],
                "avg_latency_ms": (
                    round(w["sum_lat"] / tot_w, 2) if tot_w > 0 else 0.0
                ),
            }
        )

    summary = {
        "total_records": total_records,
        "valid_records": valid_records,
        "invalid_records": invalid_records,
        "earliest_timestamp": earliest_timestamp,
        "latest_timestamp": latest_timestamp,
        "total_bytes_sent": total_bytes_sent,
        "total_services": len(services),
        "services_with_sla_breach": services_with_sla_breach,
    }

    report = {
        "summary": summary,
        "services": svc_list,
        "top_clients": top_clients,
        "windows": window_list,
    }

    try:
        json.dump(report, out_f, indent=2)
        out_f.write("\n")
    finally:
        if close_out:
            out_f.close()

    sys.exit(0)


if __name__ == "__main__":
    main()
