import json
import os
import tempfile
import unittest
import testkit

class TestInventoryAllocator(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.inv_path = os.path.join(self.tmpdir.name, "inventory.json")
        self.ord_path = os.path.join(self.tmpdir.name, "orders.json")
        self.man_path = os.path.join(self.tmpdir.name, "manifest.json")

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_priority_tier_order_processing(self):
        # Order 1 is BULK, Order 2 is VIP, Order 3 is STANDARD
        # VIP should process first and take the stock
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": {
                    "SKU-1": {
                        "lots": [{"lot_id": "L1", "quantity": 10, "expires": "2026-12-01"}]
                    }
                }
            }, f)
        with open(self.ord_path, "w") as f:
            json.dump({
                "orders": [
                    {"id": "o-bulk", "priority": "BULK", "items": {"SKU-1": 10}},
                    {"id": "o-vip", "priority": "VIP", "items": {"SKU-1": 6}},
                    {"id": "o-std", "priority": "STANDARD", "items": {"SKU-1": 4}}
                ]
            }, f)

        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 1) # o-bulk will be backordered
        with open(self.man_path) as f:
            data = json.load(f)

        # Output order must be VIP, STANDARD, BULK
        order_ids = [o["id"] for o in data["orders"]]
        self.assertEqual(order_ids, ["o-vip", "o-std", "o-bulk"])
        self.assertEqual(data["orders"][0]["status"], "fulfilled")
        self.assertEqual(data["orders"][1]["status"], "fulfilled")
        self.assertEqual(data["orders"][2]["status"], "backordered")
        self.assertEqual(data["orders"][2]["backordered"], {"SKU-1": 10})

    def test_fifo_by_expiry_date(self):
        # Two lots: L-later expires 2026-12-01, L-earlier expires 2026-10-01
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": {
                    "ITEM-X": {
                        "lots": [
                            {"lot_id": "L-later", "quantity": 10, "expires": "2026-12-01"},
                            {"lot_id": "L-earlier", "quantity": 5, "expires": "2026-10-01"}
                        ]
                    }
                }
            }, f)
        with open(self.ord_path, "w") as f:
            json.dump({
                "orders": [
                    {"id": "o-1", "items": {"ITEM-X": 7}}
                ]
            }, f)

        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 0)
        with open(self.man_path) as f:
            data = json.load(f)

        allocs = data["orders"][0]["allocations"]
        # Allocations: 5 from L-earlier, 2 from L-later
        # sorted by sku, then lot_id: "L-earlier", then "L-later"
        self.assertEqual(len(allocs), 2)
        self.assertEqual(allocs[0]["lot_id"], "L-earlier")
        self.assertEqual(allocs[0]["quantity"], 5)
        self.assertEqual(allocs[1]["lot_id"], "L-later")
        self.assertEqual(allocs[1]["quantity"], 2)

    def test_expiry_tiebreaker_lot_id(self):
        # Same expiry date, lot B vs lot A -> lot A allocated first
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": {
                    "ITEM-Y": {
                        "lots": [
                            {"lot_id": "LOT-B", "quantity": 5, "expires": "2026-11-01"},
                            {"lot_id": "LOT-A", "quantity": 5, "expires": "2026-11-01"}
                        ]
                    }
                }
            }, f)
        with open(self.ord_path, "w") as f:
            json.dump({
                "orders": [{"id": "o-1", "items": {"ITEM-Y": 6}}]
            }, f)

        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 0)
        with open(self.man_path) as f:
            data = json.load(f)
        allocs = data["orders"][0]["allocations"]
        self.assertEqual(allocs[0]["lot_id"], "LOT-A")
        self.assertEqual(allocs[0]["quantity"], 5)
        self.assertEqual(allocs[1]["lot_id"], "LOT-B")
        self.assertEqual(allocs[1]["quantity"], 1)

    def test_partial_fulfillment_status(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": {
                    "ITEM-Z": {"lots": [{"lot_id": "L1", "quantity": 3, "expires": "2026-10-10"}]}
                }
            }, f)
        with open(self.ord_path, "w") as f:
            json.dump({
                "orders": [{"id": "o-part", "items": {"ITEM-Z": 5}}]
            }, f)

        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 1)
        with open(self.man_path) as f:
            data = json.load(f)
        self.assertEqual(data["partially_fulfilled_count"], 1)
        self.assertEqual(data["orders"][0]["status"], "partially_fulfilled")
        self.assertEqual(data["orders"][0]["allocations"][0]["quantity"], 3)
        self.assertEqual(data["orders"][0]["backordered"], {"ITEM-Z": 2})

    def test_backwards_compatible_baseline_format(self):
        # Baseline simple quantity format
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": {
                    "SKU-OLD": {"quantity": 8}
                }
            }, f)
        with open(self.ord_path, "w") as f:
            json.dump({
                "orders": [{"id": "o-compat", "items": {"SKU-OLD": 5}}]
            }, f)

        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 0)
        with open(self.man_path) as f:
            data = json.load(f)
        self.assertEqual(data["fulfilled_count"], 1)
        self.assertEqual(data["orders"][0]["allocations"][0]["lot_id"], "default")
        self.assertEqual(data["orders"][0]["allocations"][0]["quantity"], 5)
        self.assertEqual(data["remaining_inventory"]["SKU-OLD"]["total_quantity"], 3)

    def test_all_orders_fulfilled_exit_0(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": {
                    "S1": {"lots": [{"lot_id": "L1", "quantity": 10, "expires": "2026-10-10"}]}
                }
            }, f)
        with open(self.ord_path, "w") as f:
            json.dump({
                "orders": [{"id": "o1", "items": {"S1": 10}}]
            }, f)
        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 0)
        with open(self.man_path) as f:
            data = json.load(f)
        self.assertEqual(data["fulfilled_count"], 1)
        self.assertEqual(data["partially_fulfilled_count"], 0)
        self.assertEqual(data["backordered_count"], 0)

    def test_invalid_priority(self):
        with open(self.inv_path, "w") as f:
            json.dump({"items": {"S1": {"quantity": 10}}}, f)
        with open(self.ord_path, "w") as f:
            json.dump({"orders": [{"id": "o1", "priority": "SUPER_URGENT", "items": {"S1": 1}}]}, f)
        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 2)

    def test_unknown_sku_error(self):
        with open(self.inv_path, "w") as f:
            json.dump({"items": {"S1": {"quantity": 10}}}, f)
        with open(self.ord_path, "w") as f:
            json.dump({"orders": [{"id": "o1", "items": {"UNKNOWN_SKU": 1}}]}, f)
        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 2)

    def test_negative_quantity_error(self):
        with open(self.inv_path, "w") as f:
            json.dump({"items": {"S1": {"quantity": 10}}}, f)
        with open(self.ord_path, "w") as f:
            json.dump({"orders": [{"id": "o1", "items": {"S1": -5}}]}, f)
        res = testkit.run([self.inv_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("invalid")
        res = testkit.run([bad_path, self.ord_path, self.man_path])
        self.assertEqual(res.returncode, 2)
