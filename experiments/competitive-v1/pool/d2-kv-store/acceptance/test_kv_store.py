import json
import os
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestKvStore(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_put_new_key_201(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("PUT", "/keys/msg", {"value": "hello world"})
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["key"], "msg")
            self.assertEqual(resp["value"], "hello world")
            self.assertEqual(resp["version"], 1)

    def test_get_existing_key_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/user:1", {"value": {"name": "Alice"}})
            status, _, body = s.request("GET", "/keys/user:1")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["value"], {"name": "Alice"})
            self.assertEqual(resp["version"], 1)

    def test_put_update_key_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/counter", {"value": 1})
            status, _, body = s.request("PUT", "/keys/counter", {"value": 2})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["version"], 2)
            self.assertEqual(resp["value"], 2)

    def test_put_expected_version_zero_conflict_409(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/exists", {"value": "first"})
            # expected_version=0 means must not exist
            status, _, body = s.request("PUT", "/keys/exists", {"value": "second", "expected_version": 0})
            self.assertEqual(status, 409)
            resp = json.loads(body.decode())
            self.assertEqual(resp["error"], "version_conflict")
            self.assertEqual(resp["current_version"], 1)

    def test_put_expected_version_match_success(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/item", {"value": "v1"}) # version 1
            status, _, body = s.request("PUT", "/keys/item", {"value": "v2", "expected_version": 1})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["version"], 2)

    def test_put_expected_version_mismatch_conflict_409(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/item", {"value": "v1"}) # version 1
            status, _, body = s.request("PUT", "/keys/item", {"value": "v2", "expected_version": 99})
            self.assertEqual(status, 409)

    def test_ttl_expiration_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # 1 second TTL
            s.request("PUT", "/keys/ephemeral", {"value": "quick", "ttl_seconds": 1})
            time.sleep(1.2)
            status, _, body = s.request("GET", "/keys/ephemeral")
            self.assertEqual(status, 404)

    def test_delete_key_204(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/to_delete", {"value": 123})
            status, _, _ = s.request("DELETE", "/keys/to_delete")
            self.assertEqual(status, 204)
            get_status, _, _ = s.request("GET", "/keys/to_delete")
            self.assertEqual(get_status, 404)

    def test_delete_nonexistent_key_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, _ = s.request("DELETE", "/keys/missing_key")
            self.assertEqual(status, 404)

    def test_prefix_query_filtering(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/app:config", {"value": 1})
            s.request("PUT", "/keys/app:secret", {"value": 2})
            s.request("PUT", "/keys/user:alice", {"value": 3})

            status, _, body = s.request("GET", "/keys?prefix=app:")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["keys"], ["app:config", "app:secret"])

    def test_prefix_query_limit(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("PUT", "/keys/k1", {"value": 1})
            s.request("PUT", "/keys/k2", {"value": 2})
            s.request("PUT", "/keys/k3", {"value": 3})

            status, _, body = s.request("GET", "/keys?limit=2")
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(len(resp["keys"]), 2)
            self.assertEqual(resp["keys"], ["k1", "k2"])

    def test_persistence_across_restarts(self):
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("PUT", "/keys/persist_me", {"value": "safe"})

        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/keys/persist_me")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["value"], "safe")

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "PUT", "path": "/keys/r1", "body": {"value": 10}}) + "\n")
            f.write(json.dumps({"method": "GET", "path": "/keys/r1"}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 200)
        self.assertEqual(lines[1]["body"]["value"], 10)


if __name__ == "__main__":
    unittest.main()
