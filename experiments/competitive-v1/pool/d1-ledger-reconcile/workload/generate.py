#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(9876)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    accounts = ["ACC-CORP-1", "ACC-CORP-2", "ACC-RETAIL-A", "ACC-RETAIL-B"]
    currencies = ["USD", "EUR", "GBP"]

    internal_records = []
    external_rows = ["ref_id,account_id,amount,currency,posted_at"]

    for i in range(1, 301):
        txn_id = f"TX-{i:05d}"
        acc = random.choice(accounts)
        curr = random.choice(currencies)
        amount = round(random.uniform(10.0, 5000.0), 2)
        ts = f"2026-10-01T{i//60:02d}:{i%60:02d}:00Z"

        # mostly clean matches
        if i % 20 == 0:
            # Unmatched internal
            internal_records.append({"txn_id": txn_id, "account_id": acc, "amount": amount, "currency": curr, "timestamp": ts})
        elif i % 25 == 0:
            # Unmatched external
            external_rows.append(f"{txn_id},{acc},{amount},{curr},{ts}")
        elif i % 15 == 0:
            # Amount mismatch
            internal_records.append({"txn_id": txn_id, "account_id": acc, "amount": amount, "currency": curr, "timestamp": ts})
            external_rows.append(f"{txn_id},{acc},{round(amount + 5.0, 2)},{curr},{ts}")
        elif i % 30 == 0:
            # Metadata mismatch
            internal_records.append({"txn_id": txn_id, "account_id": acc, "amount": amount, "currency": "USD", "timestamp": ts})
            external_rows.append(f"{txn_id},{acc},{amount},EUR,{ts}")
        else:
            # Clean match
            internal_records.append({"txn_id": txn_id, "account_id": acc, "amount": amount, "currency": curr, "timestamp": ts})
            external_rows.append(f"{txn_id},{acc},{amount},{curr},{ts}")

    with open(os.path.join(dir_path, "internal.json"), "w") as f:
        json.dump(internal_records, f, indent=2)

    with open(os.path.join(dir_path, "external.csv"), "w") as f:
        f.write("\n".join(external_rows) + "\n")

if __name__ == "__main__":
    main()
