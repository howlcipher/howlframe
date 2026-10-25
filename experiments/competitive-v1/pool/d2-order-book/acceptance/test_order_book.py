import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestOrderBook(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_place_resting_order_open(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/orders", {
                "symbol": "GOLD",
                "side": "buy",
                "price": 100.0,
                "quantity": 10
            })
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["order_id"], "ord-1")
            self.assertEqual(resp["status"], "open")
            self.assertEqual(resp["filled_quantity"], 0)
            self.assertEqual(resp["remaining_quantity"], 10)
            self.assertEqual(len(resp["trades"]), 0)

    def test_invalid_order_body_400(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/orders", {
                "symbol": "GOLD",
                "side": "hold", # bad side
                "price": 100.0,
                "quantity": 10
            })
            self.assertEqual(status, 400)

    def test_exact_full_match(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # Resting sell 10 @ 100
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 100.0, "quantity": 10})
            # Incoming buy 10 @ 100
            status, _, body = s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 100.0, "quantity": 10})
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["status"], "filled")
            self.assertEqual(resp["filled_quantity"], 10)
            self.assertEqual(len(resp["trades"]), 1)
            t = resp["trades"][0]
            self.assertEqual(t["price"], 100.0)
            self.assertEqual(t["quantity"], 10)

    def test_partial_fill(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # Resting sell 5 @ 100
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 100.0, "quantity": 5})
            # Incoming buy 10 @ 100
            status, _, body = s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 100.0, "quantity": 10})
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["status"], "partially_filled")
            self.assertEqual(resp["filled_quantity"], 5)
            self.assertEqual(resp["remaining_quantity"], 5)

    def test_price_improvement_execution_price(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # Resting sell 10 @ 100
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 100.0, "quantity": 10})
            # Incoming buy willing to pay up to 105
            status, _, body = s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 105.0, "quantity": 10})
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            # Trade must execute at the resting order's price (100.0)
            self.assertEqual(resp["trades"][0]["price"], 100.0)

    def test_price_priority_bids(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # Bid 1: buy @ 99
            s.request("POST", "/orders", {"symbol": "OIL", "side": "buy", "price": 99.0, "quantity": 10})
            # Bid 2: buy @ 101 (higher price, should match first)
            s.request("POST", "/orders", {"symbol": "OIL", "side": "buy", "price": 101.0, "quantity": 10})

            # Sell arrives @ 95 for 10 units
            status, _, body = s.request("POST", "/orders", {"symbol": "OIL", "side": "sell", "price": 95.0, "quantity": 10})
            resp = json.loads(body.decode())
            self.assertEqual(resp["trades"][0]["price"], 101.0)
            self.assertEqual(resp["trades"][0]["buy_order_id"], "ord-2")

    def test_price_priority_asks(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # Ask 1: sell @ 105
            s.request("POST", "/orders", {"symbol": "SILVER", "side": "sell", "price": 105.0, "quantity": 10})
            # Ask 2: sell @ 102 (lower price, should match first)
            s.request("POST", "/orders", {"symbol": "SILVER", "side": "sell", "price": 102.0, "quantity": 10})

            # Buy arrives @ 110 for 10 units
            status, _, body = s.request("POST", "/orders", {"symbol": "SILVER", "side": "buy", "price": 110.0, "quantity": 10})
            resp = json.loads(body.decode())
            self.assertEqual(resp["trades"][0]["price"], 102.0)
            self.assertEqual(resp["trades"][0]["sell_order_id"], "ord-2")

    def test_time_priority(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # S1 @ 100
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 100.0, "quantity": 5})
            # S2 @ 100
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 100.0, "quantity": 5})

            # Buy 5 @ 100 -> should match S1 first
            status, _, body = s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 100.0, "quantity": 5})
            resp = json.loads(body.decode())
            self.assertEqual(resp["trades"][0]["sell_order_id"], "ord-1")

    def test_cancel_order_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 90.0, "quantity": 10})
            status, _, body = s.request("DELETE", "/orders/ord-1")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["status"], "cancelled")

            # Book should now be empty for bids
            _, _, book_b = s.request("GET", "/book/GOLD")
            book = json.loads(book_b.decode())
            self.assertEqual(book["bids"], [])

    def test_cancel_filled_order_400(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 100.0, "quantity": 10})
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 100.0, "quantity": 10})
            # ord-1 is filled
            status, _, body = s.request("DELETE", "/orders/ord-1")
            self.assertEqual(status, 400)

    def test_get_book_depth_aggregated(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 99.0, "quantity": 10})
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 99.0, "quantity": 5}) # total 15 @ 99
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 98.0, "quantity": 20})
            s.request("POST", "/orders", {"symbol": "GOLD", "side": "sell", "price": 101.0, "quantity": 8})

            status, _, body = s.request("GET", "/book/GOLD")
            self.assertEqual(status, 200)
            book = json.loads(body.decode())
            self.assertEqual(book["bids"], [{"price": 99.0, "quantity": 15}, {"price": 98.0, "quantity": 20}])
            self.assertEqual(book["asks"], [{"price": 101.0, "quantity": 8}])

    def test_get_trades_history(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/orders", {"symbol": "OIL", "side": "sell", "price": 70.0, "quantity": 10})
            s.request("POST", "/orders", {"symbol": "OIL", "side": "buy", "price": 70.0, "quantity": 10})

            status, _, body = s.request("GET", "/trades/OIL")
            self.assertEqual(status, 200)
            trades = json.loads(body.decode())["trades"]
            self.assertEqual(len(trades), 1)
            self.assertEqual(trades[0]["symbol"], "OIL")
            self.assertEqual(trades[0]["price"], 70.0)

    def test_persistence_across_restart(self):
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("POST", "/orders", {"symbol": "GOLD", "side": "buy", "price": 95.0, "quantity": 7})

        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/orders/ord-1")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["quantity"], 7)

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "POST", "path": "/orders", "body": {"symbol": "G", "side": "buy", "price": 10.0, "quantity": 5}}) + "\n")
            f.write(json.dumps({"method": "GET", "path": "/orders/ord-1"}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 200)


if __name__ == "__main__":
    unittest.main()
