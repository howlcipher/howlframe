import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestLedgerReconcile(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.internal_path = os.path.join(self.temp_dir.name, "internal.json")
        self.external_path = os.path.join(self.temp_dir.name, "external.csv")
        self.out_path = os.path.join(self.temp_dir.name, "recon.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_files(self, internal_data, external_lines):
        with open(self.internal_path, "w") as f:
            json.dump(internal_data, f)
        with open(self.external_path, "w") as f:
            f.write("ref_id,account_id,amount,currency,posted_at\n")
            for line in external_lines:
                f.write(line + "\n")

    def test_exact_matches(self):
        self._write_files(
            [{"txn_id": "TX-1", "account_id": "ACC-A", "amount": 100.50, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}],
            ["TX-1,ACC-A,100.50,USD,2026-10-01T10:05:00Z"]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["matched_count"], 1)
        self.assertEqual(out["net_settled_amount"], 100.50)
        self.assertEqual(len(out["discrepancies"]["amount_mismatch"]), 0)

    def test_net_settled_amount_calculation(self):
        self._write_files(
            [
                {"txn_id": "TX-1", "account_id": "ACC-A", "amount": 100.25, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"},
                {"txn_id": "TX-2", "account_id": "ACC-A", "amount": 200.50, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}
            ],
            [
                "TX-1,ACC-A,100.25,USD,2026-10-01T10:05:00Z",
                "TX-2,ACC-A,200.50,USD,2026-10-01T10:05:00Z"
            ]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["matched_count"], 2)
        self.assertEqual(out["net_settled_amount"], 300.75)

    def test_amount_mismatch(self):
        self._write_files(
            [{"txn_id": "TX-1", "account_id": "ACC-A", "amount": 100.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}],
            ["TX-1,ACC-A,95.50,USD,2026-10-01T10:05:00Z"]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["matched_count"], 0)
        self.assertEqual(out["net_settled_amount"], 0.0)
        diffs = out["discrepancies"]["amount_mismatch"]
        self.assertEqual(len(diffs), 1)
        self.assertEqual(diffs[0]["id"], "TX-1")
        self.assertEqual(diffs[0]["difference"], 4.50)

    def test_metadata_mismatch_account(self):
        self._write_files(
            [{"txn_id": "TX-1", "account_id": "ACC-A", "amount": 50.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}],
            ["TX-1,ACC-B,50.0,USD,2026-10-01T10:05:00Z"]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        meta = out["discrepancies"]["metadata_mismatch"]
        self.assertEqual(len(meta), 1)
        self.assertEqual(meta[0]["internal_account"], "ACC-A")
        self.assertEqual(meta[0]["external_account"], "ACC-B")

    def test_metadata_mismatch_currency(self):
        self._write_files(
            [{"txn_id": "TX-1", "account_id": "ACC-A", "amount": 50.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}],
            ["TX-1,ACC-A,50.0,EUR,2026-10-01T10:05:00Z"]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        meta = out["discrepancies"]["metadata_mismatch"]
        self.assertEqual(len(meta), 1)
        self.assertEqual(meta[0]["internal_currency"], "USD")
        self.assertEqual(meta[0]["external_currency"], "EUR")

    def test_unmatched_internal(self):
        self._write_files(
            [{"txn_id": "TX-INTERNAL-ONLY", "account_id": "ACC-A", "amount": 25.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}],
            []
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        unmatched = out["discrepancies"]["unmatched_internal"]
        self.assertEqual(len(unmatched), 1)
        self.assertEqual(unmatched[0]["id"], "TX-INTERNAL-ONLY")

    def test_unmatched_external(self):
        self._write_files(
            [],
            ["TX-EXT-ONLY,ACC-B,75.0,USD,2026-10-01T10:00:00Z"]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        unmatched = out["discrepancies"]["unmatched_external"]
        self.assertEqual(len(unmatched), 1)
        self.assertEqual(unmatched[0]["id"], "TX-EXT-ONLY")

    def test_internal_duplicates(self):
        self._write_files(
            [
                {"txn_id": "DUP-1", "account_id": "ACC-A", "amount": 10.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"},
                {"txn_id": "DUP-1", "account_id": "ACC-A", "amount": 10.0, "currency": "USD", "timestamp": "2026-10-01T10:01:00Z"}
            ],
            ["DUP-1,ACC-A,10.0,USD,2026-10-01T10:00:00Z"]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        dups = out["discrepancies"]["duplicates"]
        self.assertEqual(len(dups), 1)
        self.assertEqual(dups[0]["source"], "internal")
        self.assertEqual(dups[0]["id"], "DUP-1")

    def test_external_duplicates(self):
        self._write_files(
            [{"txn_id": "DUP-2", "account_id": "ACC-A", "amount": 10.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}],
            [
                "DUP-2,ACC-A,10.0,USD,2026-10-01T10:00:00Z",
                "DUP-2,ACC-A,10.0,USD,2026-10-01T10:05:00Z"
            ]
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        dups = out["discrepancies"]["duplicates"]
        self.assertEqual(len(dups), 1)
        self.assertEqual(dups[0]["source"], "external")
        self.assertEqual(dups[0]["id"], "DUP-2")

    def test_discrepancies_sorting(self):
        self._write_files(
            [
                {"txn_id": "TX-Z", "account_id": "ACC-A", "amount": 10.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"},
                {"txn_id": "TX-A", "account_id": "ACC-A", "amount": 10.0, "currency": "USD", "timestamp": "2026-10-01T10:00:00Z"}
            ],
            []
        )
        res = testkit.run(["reconcile", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        unmatched = out["discrepancies"]["unmatched_internal"]
        self.assertEqual(unmatched[0]["id"], "TX-A")
        self.assertEqual(unmatched[1]["id"], "TX-Z")

    def test_missing_cli_flags_exit_1(self):
        res = testkit.run(["reconcile", "--internal", self.internal_path])
        self.assertEqual(res.returncode, 1)

    def test_unrecognized_subcommand_exit_1(self):
        res = testkit.run(["match", "--internal", self.internal_path, "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_missing_file_exit_2(self):
        res = testkit.run(["reconcile", "--internal", "/missing/internal.json", "--external", self.external_path, "--out", self.out_path])
        self.assertEqual(res.returncode, 2)


if __name__ == "__main__":
    unittest.main()
