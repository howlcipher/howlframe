import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestLogisticsManifest(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.rules_path = os.path.join(self.temp_dir.name, "rules.json")
        self.pkg_path = os.path.join(self.temp_dir.name, "packages.csv")
        self.out_path = os.path.join(self.temp_dir.name, "manifest.json")

        self.rules = {
            "max_container_weight_kg": 500.0,
            "max_hazard_items_per_container": 1,
            "allowed_destinations": ["NORTH", "SOUTH", "EAST"]
        }
        with open(self.rules_path, "w") as f:
            json.dump(self.rules, f)

    def tearDown(self):
        self.temp_dir.cleanup()

    def _write_csv(self, rows):
        with open(self.pkg_path, "w") as f:
            f.write("package_id,destination,weight_kg,is_hazardous,priority\n")
            for r in rows:
                f.write(r + "\n")

    def test_single_container_packing(self):
        self._write_csv([
            "PKG-1,NORTH,150.0,false,1",
            "PKG-2,NORTH,200.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["total_containers"], 1)
        self.assertEqual(out["total_packed_packages"], 2)
        c = out["containers"][0]
        self.assertEqual(c["container_id"], "CONT-1")
        self.assertEqual(c["destination"], "NORTH")
        self.assertEqual(c["total_weight_kg"], 350.0)
        self.assertEqual(c["hazard_count"], 0)
        self.assertEqual(c["package_ids"], ["PKG-2", "PKG-1"]) # PKG-2 (200kg) packed before PKG-1 (150kg)

    def test_weight_capacity_overflow_creates_new_container(self):
        self._write_csv([
            "PKG-1,NORTH,300.0,false,1",
            "PKG-2,NORTH,300.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["total_containers"], 2)
        self.assertEqual(out["containers"][0]["package_ids"], ["PKG-1"])
        self.assertEqual(out["containers"][1]["package_ids"], ["PKG-2"])

    def test_different_destinations_separate_containers(self):
        self._write_csv([
            "PKG-1,NORTH,100.0,false,1",
            "PKG-2,SOUTH,100.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["total_containers"], 2)
        self.assertEqual(out["containers"][0]["destination"], "NORTH")
        self.assertEqual(out["containers"][1]["destination"], "SOUTH")

    def test_hazardous_limit_enforced(self):
        # max_hazard_items_per_container is 1
        self._write_csv([
            "PKG-1,NORTH,50.0,true,1",
            "PKG-2,NORTH,50.0,true,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["total_containers"], 2)
        self.assertEqual(out["containers"][0]["hazard_count"], 1)
        self.assertEqual(out["containers"][1]["hazard_count"], 1)

    def test_priority_sorting(self):
        # PKG-2 has priority 1, PKG-1 has priority 2
        self._write_csv([
            "PKG-1,NORTH,10.0,false,2",
            "PKG-2,NORTH,10.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["containers"][0]["package_ids"], ["PKG-2", "PKG-1"])

    def test_weight_descending_sorting_within_same_priority(self):
        self._write_csv([
            "PKG-A,NORTH,50.0,false,1",
            "PKG-B,NORTH,200.0,false,1",
            "PKG-C,NORTH,120.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["containers"][0]["package_ids"], ["PKG-B", "PKG-C", "PKG-A"])

    def test_rejected_invalid_destination(self):
        self._write_csv([
            "PKG-BAD,MOON,10.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(len(out["rejected_packages"]), 1)
        self.assertEqual(out["rejected_packages"][0]["package_id"], "PKG-BAD")
        self.assertEqual(out["rejected_packages"][0]["reason"], "invalid_destination")

    def test_rejected_overweight(self):
        # max_container_weight_kg is 500
        self._write_csv([
            "PKG-HEAVY,NORTH,550.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(len(out["rejected_packages"]), 1)
        self.assertEqual(out["rejected_packages"][0]["reason"], "overweight")

    def test_rejected_invalid_fields(self):
        self._write_csv([
            "PKG-INV1,NORTH,abc,false,1",
            "PKG-INV2,NORTH,50.0,maybe,1",
            "PKG-INV3,NORTH,50.0,false,9"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(len(out["rejected_packages"]), 3)
        for rej in out["rejected_packages"]:
            self.assertEqual(rej["reason"], "invalid_fields")

    def test_sorting_rejected_packages(self):
        self._write_csv([
            "PKG-Z,MOON,10.0,false,1",
            "PKG-A,MARS,10.0,false,1"
        ])
        res = testkit.run(["pack", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 0)
        with open(self.out_path) as f:
            out = json.load(f)
        self.assertEqual(out["rejected_packages"][0]["package_id"], "PKG-A")
        self.assertEqual(out["rejected_packages"][1]["package_id"], "PKG-Z")

    def test_missing_flags_exit_1(self):
        res = testkit.run(["pack", "--packages", self.pkg_path])
        self.assertEqual(res.returncode, 1)

    def test_invalid_subcommand_exit_1(self):
        res = testkit.run(["ship", "--packages", self.pkg_path, "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 1)

    def test_nonexistent_files_exit_2(self):
        res = testkit.run(["pack", "--packages", "/missing/pkg.csv", "--rules", self.rules_path, "--output", self.out_path])
        self.assertEqual(res.returncode, 2)


if __name__ == "__main__":
    unittest.main()
