#!/usr/bin/env python3
import json
import os
import random

def main():
    random.seed(777111)
    dir_path = os.path.dirname(os.path.abspath(__file__))

    requests = []
    symbols = ["GOLD", "SILVER", "OIL"]

    for i in range(1, 301):
        sym = random.choice(symbols)
        side = random.choice(["buy", "sell"])
        base_price = 100.0 if sym == "GOLD" else (25.0 if sym == "SILVER" else 75.0)
        price = round(base_price + random.uniform(-5.0, 5.0), 1)
        qty = random.randint(1, 20)

        requests.append({"method": "POST", "path": "/orders", "body": {
            "symbol": sym,
            "side": side,
            "price": price,
            "quantity": qty
        }})

        if i % 30 == 0:
            requests.append({"method": "GET", "path": f"/book/{sym}"})
        if i % 50 == 0:
            requests.append({"method": "GET", "path": f"/trades/{sym}"})

    with open(os.path.join(dir_path, "requests.jsonl"), "w") as f:
        for req in requests:
            f.write(json.dumps(req) + "\n")

    with open(os.path.join(dir_path, "store.json"), "w") as f:
        json.dump({"orders": {}, "trades": [], "next_order_id": 1, "next_trade_id": 1}, f)

if __name__ == "__main__":
    main()
