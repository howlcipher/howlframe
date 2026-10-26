import json
import os
import tempfile
import unittest
import testkit

class TestInvoiceTaxCalculator(unittest.TestCase):
    def setUp(self):
        self.tmpdir = tempfile.TemporaryDirectory()
        self.cfg_path = os.path.join(self.tmpdir.name, "tax_config.json")
        self.inv_path = os.path.join(self.tmpdir.name, "invoice.json")
        self.rec_path = os.path.join(self.tmpdir.name, "receipt.json")

        self.default_config = {
            "categories": {
                "standard": 0.10,
                "reduced": 0.05,
                "exempt": 0.00
            },
            "luxury_surcharge": {
                "threshold_cents": 10000,
                "rate": 0.05
            }
        }
        with open(self.cfg_path, "w") as f:
            json.dump(self.default_config, f)

    def tearDown(self):
        self.tmpdir.cleanup()

    def test_single_item_standard_rate(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "i1", "description": "Widget", "unit_price_cents": 2000, "quantity": 2}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["gross_subtotal_cents"], 4000)
        self.assertEqual(data["discount_cents"], 0)
        self.assertEqual(data["net_subtotal_cents"], 4000)
        self.assertEqual(data["total_base_tax_cents"], 400)
        self.assertEqual(data["total_surcharge_cents"], 0)
        self.assertEqual(data["grand_total_cents"], 4400)

    def test_multi_category_rates(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "i-std", "category": "standard", "unit_price_cents": 1000, "quantity": 1},
                    {"id": "i-red", "category": "reduced", "unit_price_cents": 1000, "quantity": 1},
                    {"id": "i-ex", "category": "exempt", "unit_price_cents": 1000, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["items"][0]["base_tax_cents"], 100) # 10% of 1000
        self.assertEqual(data["items"][1]["base_tax_cents"], 50)  # 5% of 1000
        self.assertEqual(data["items"][2]["base_tax_cents"], 0)   # 0% of 1000
        self.assertEqual(data["total_base_tax_cents"], 150)

    def test_luxury_surcharge_applied_above_threshold(self):
        # Unit price: 15000, threshold: 10000. Above threshold = 5000.
        # Surcharge = 5000 * 0.05 = 250 cents.
        # Base tax = 15000 * 0.10 = 1500 cents.
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "lux", "category": "standard", "unit_price_cents": 15000, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["items"][0]["surcharge_cents"], 250)
        self.assertEqual(data["items"][0]["base_tax_cents"], 1500)
        self.assertEqual(data["items"][0]["total_tax_cents"], 1750)
        self.assertEqual(data["total_surcharge_cents"], 250)

    def test_exempt_item_not_surcharged(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "lux-ex", "category": "exempt", "unit_price_cents": 20000, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["items"][0]["surcharge_cents"], 0)
        self.assertEqual(data["items"][0]["total_tax_cents"], 0)

    def test_discount_proportional_distribution_non_exempt(self):
        # item A (standard): 3000 gross
        # item B (standard): 1000 gross
        # item C (exempt): 1000 gross
        # discount: 400
        # Non-exempt eligible total = 4000.
        # Item A gets 400 * (3000 / 4000) = 300 discount.
        # Item B gets 400 * (1000 / 4000) = 100 discount.
        # Item C gets 0 discount.
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "a", "category": "standard", "unit_price_cents": 3000, "quantity": 1},
                    {"id": "b", "category": "standard", "unit_price_cents": 1000, "quantity": 1},
                    {"id": "c", "category": "exempt", "unit_price_cents": 1000, "quantity": 1}
                ],
                "discount_cents": 400
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["items"][0]["discount_cents"], 300)
        self.assertEqual(data["items"][0]["net_cents"], 2700)
        self.assertEqual(data["items"][1]["discount_cents"], 100)
        self.assertEqual(data["items"][1]["net_cents"], 900)
        self.assertEqual(data["items"][2]["discount_cents"], 0)
        self.assertEqual(data["items"][2]["net_cents"], 1000)

    def test_bankers_rounding_half_to_even(self):
        # 10% of 25 = 2.5 -> rounds to even integer 2!
        # 10% of 35 = 3.5 -> rounds to even integer 4!
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "i-even", "category": "standard", "unit_price_cents": 25, "quantity": 1},
                    {"id": "i-odd", "category": "standard", "unit_price_cents": 35, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["items"][0]["base_tax_cents"], 2)
        self.assertEqual(data["items"][1]["base_tax_cents"], 4)

    def test_backwards_compatibility_flat_tax_rate(self):
        with open(self.cfg_path, "w") as f:
            json.dump({"flat_tax_rate": 0.08}, f)
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "item-legacy", "unit_price_cents": 1000, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["total_base_tax_cents"], 80)

    def test_discount_all_exempt_items(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "e1", "category": "exempt", "unit_price_cents": 1000, "quantity": 1},
                    {"id": "e2", "category": "exempt", "unit_price_cents": 1000, "quantity": 1}
                ],
                "discount_cents": 200
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 0)
        with open(self.rec_path) as f:
            data = json.load(f)
        self.assertEqual(data["items"][0]["discount_cents"], 100)
        self.assertEqual(data["items"][1]["discount_cents"], 100)

    def test_unknown_category_exits_2(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "e1", "category": "non_existent_category", "unit_price_cents": 1000, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 2)

    def test_negative_unit_price_exits_2(self):
        with open(self.inv_path, "w") as f:
            json.dump({
                "items": [
                    {"id": "e1", "unit_price_cents": -50, "quantity": 1}
                ]
            }, f)
        res = testkit.run([self.cfg_path, self.inv_path, self.rec_path])
        self.assertEqual(res.returncode, 2)

    def test_missing_args(self):
        res = testkit.run([])
        self.assertEqual(res.returncode, 2)

    def test_malformed_json(self):
        bad_path = os.path.join(self.tmpdir.name, "bad.json")
        with open(bad_path, "w") as f:
            f.write("{invalid")
        res = testkit.run([self.cfg_path, bad_path, self.rec_path])
        self.assertEqual(res.returncode, 2)
