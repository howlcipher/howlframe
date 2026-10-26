import json
import os
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestCacheInvalidator(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_post_cache_entry_201(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/cache", {
                "key": "item-1",
                "value": {"title": "Hat"},
                "tags": ["apparel", "summer"]
            })
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "item-1")
            self.assertEqual(resp["tags"], ["apparel", "summer"])

    def test_post_cache_invalid_payload_400(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, _ = s.request("POST", "/cache", {"key": ""})
            self.assertEqual(status, 400)

    def test_get_cache_entry_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "p1", "value": "val1", "tags": ["t1"]})
            status, _, body = s.request("GET", "/cache/p1")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["value"], "val1")

    def test_get_nonexistent_cache_entry_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, _ = s.request("GET", "/cache/missing")
            self.assertEqual(status, 404)

    def test_ttl_expiration_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "short", "value": 1, "tags": [], "ttl_seconds": 1})
            time.sleep(1.2)
            status, _, _ = s.request("GET", "/cache/short")
            self.assertEqual(status, 404)

    def test_purge_single_tag(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "k1", "value": 1, "tags": ["shoes", "nike"]})
            s.request("POST", "/cache", {"key": "k2", "value": 2, "tags": ["shoes", "adidas"]})
            s.request("POST", "/cache", {"key": "k3", "value": 3, "tags": ["shirts"]})

            # Purge "nike"
            status, _, body = s.request("POST", "/purge", {"tags": ["nike"]})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["purged_count"], 1)
            self.assertEqual(resp["purged_keys"], ["k1"])

            # k1 is now 404
            st1, _, _ = s.request("GET", "/cache/k1")
            self.assertEqual(st1, 404)
            # k2 and k3 still active
            st2, _, _ = s.request("GET", "/cache/k2")
            self.assertEqual(st2, 200)

    def test_purge_multiple_tags_union(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "a", "value": 1, "tags": ["tagA"]})
            s.request("POST", "/cache", {"key": "b", "value": 2, "tags": ["tagB"]})
            s.request("POST", "/cache", {"key": "c", "value": 3, "tags": ["tagC"]})

            status, _, body = s.request("POST", "/purge", {"tags": ["tagA", "tagB"]})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["purged_count"], 2)
            self.assertEqual(resp["purged_keys"], ["a", "b"])

    def test_purge_invalid_payload_400(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, _ = s.request("POST", "/purge", {"tags": []}) # empty array
            self.assertEqual(status, 400)

    def test_delete_key_204(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "del_me", "value": 10, "tags": ["t"]})
            status, _, _ = s.request("DELETE", "/cache/del_me")
            self.assertEqual(status, 204)
            st_get, _, _ = s.request("GET", "/cache/del_me")
            self.assertEqual(st_get, 404)

    def test_get_keys_by_tag(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "z-key", "value": 1, "tags": ["special"]})
            s.request("POST", "/cache", {"key": "a-key", "value": 2, "tags": ["special"]})

            status, _, body = s.request("GET", "/tags/special")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["tag"], "special")
            self.assertEqual(resp["keys"], ["a-key", "z-key"])

    def test_stats_metrics(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/cache", {"key": "m1", "value": 1, "tags": ["t1", "t2"]})
            s.request("POST", "/cache", {"key": "m2", "value": 2, "tags": ["t2", "t3"]})
            s.request("POST", "/purge", {"tags": ["t1"]}) # purges m1

            status, _, body = s.request("GET", "/stats")
            self.assertEqual(status, 200)
            stats = json.loads(body.decode())
            self.assertEqual(stats["active_entries"], 1) # only m2
            self.assertEqual(stats["total_purged"], 1)
            self.assertEqual(stats["active_tags"], 2) # t2 and t3

    def test_persistence_across_restart(self):
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("POST", "/cache", {"key": "persist", "value": "val", "tags": ["p"]})

        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/cache/persist")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["value"], "val")

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "POST", "path": "/cache", "body": {"key": "rk", "value": 1, "tags": ["t"]}}) + "\n")
            f.write(json.dumps({"method": "GET", "path": "/cache/rk"}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 200)


if __name__ == "__main__":
    unittest.main()
