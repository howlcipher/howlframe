#!/usr/bin/env python3
"""Deterministic workload generator for calib-apilog-analyzer."""

import argparse
import json
import os
import random
import sys


def generate_workload(num_records: int, output_path: str, seed: int = 42):
    rng = random.Random(seed)

    services = [
        "auth-service",
        "billing-service",
        "catalog-service",
        "order-service",
        "notification-service",
        "search-service",
        "user-service",
        "inventory-service",
    ]

    endpoints = [
        ("GET", "/api/v1/health"),
        ("GET", "/api/v1/users/profile"),
        ("POST", "/api/v1/auth/login"),
        ("POST", "/api/v1/orders/checkout"),
        ("GET", "/api/v1/catalog/items"),
        ("GET", "/api/v1/search/query"),
        ("PUT", "/api/v1/inventory/stock"),
        ("DELETE", "/api/v1/orders/cancel"),
        ("PATCH", "/api/v1/users/settings"),
    ]

    clients = [f"client-{i:03d}" for i in range(1, 51)]

    # Weights for status codes: mostly 200, some 4xx, fewer 5xx
    status_choices = [200, 201, 204, 301, 304, 400, 401, 403, 404, 500, 502, 503]
    status_weights = [70, 10, 5, 2, 2, 3, 2, 2, 2, 1, 0.5, 0.5]

    base_time = 1710000000

    out_dir = os.path.dirname(os.path.abspath(output_path))
    if out_dir:
        os.makedirs(out_dir, exist_ok=True)
    with open(output_path, "w", encoding="utf-8") as f:
        for i in range(num_records):
            # Advance time deterministically
            ts = base_time + (i * 3)

            # Inject occasional invalid records (roughly 0.2%)
            if i % 500 == 13:
                # Malformed JSON
                f.write('{"timestamp": 1710000000, "service": "broken-json", missing_quote\n')
                continue
            elif i % 500 == 77:
                # Negative timestamp
                rec = {
                    "timestamp": -10,
                    "service": "auth-service",
                    "client_id": "client-001",
                    "method": "GET",
                    "path": "/api/v1/health",
                    "status": 200,
                    "latency_ms": 15,
                    "bytes_sent": 120,
                }
                f.write(json.dumps(rec) + "\n")
                continue
            elif i % 500 == 142:
                # Invalid HTTP method
                rec = {
                    "timestamp": ts,
                    "service": "catalog-service",
                    "client_id": "client-002",
                    "method": "INVALID_METHOD",
                    "path": "/api/v1/items",
                    "status": 200,
                    "latency_ms": 25,
                    "bytes_sent": 300,
                }
                f.write(json.dumps(rec) + "\n")
                continue
            elif i % 500 == 211:
                # Missing required field (path)
                rec = {
                    "timestamp": ts,
                    "service": "order-service",
                    "client_id": "client-003",
                    "method": "POST",
                    "status": 200,
                    "latency_ms": 100,
                    "bytes_sent": 512,
                }
                f.write(json.dumps(rec) + "\n")
                continue
            elif i % 500 == 333:
                # Invalid status code (999)
                rec = {
                    "timestamp": ts,
                    "service": "user-service",
                    "client_id": "client-004",
                    "method": "GET",
                    "path": "/api/v1/users/profile",
                    "status": 999,
                    "latency_ms": 30,
                    "bytes_sent": 250,
                }
                f.write(json.dumps(rec) + "\n")
                continue

            svc = rng.choice(services)
            client = rng.choice(clients)
            method, path = rng.choice(endpoints)
            status = rng.choices(status_choices, weights=status_weights)[0]

            # Latency: base + variability depending on status
            if status >= 500:
                latency = rng.randint(400, 1200)
            else:
                latency = rng.randint(5, 350)

            bytes_sent = rng.randint(50, 8000)

            rec = {
                "timestamp": ts,
                "service": svc,
                "client_id": client,
                "method": method,
                "path": path,
                "status": status,
                "latency_ms": latency,
                "bytes_sent": bytes_sent,
            }
            f.write(json.dumps(rec) + "\n")


def main():
    parser = argparse.ArgumentParser(description="Generate benchmark workload logs.")
    parser.add_argument(
        "--records",
        type=int,
        default=250000,
        help="Number of records to generate (default: 250000)",
    )
    parser.add_argument(
        "--output",
        default=os.path.join(os.path.dirname(__file__), "requests.jsonl"),
        help="Output file path",
    )
    parser.add_argument(
        "--seed",
        type=int,
        default=42,
        help="Deterministic random seed (default: 42)",
    )
    args = parser.parse_args()
    generate_workload(args.records, args.output, args.seed)
    print(f"Generated {args.records} records to {args.output}")


if __name__ == "__main__":
    main()
