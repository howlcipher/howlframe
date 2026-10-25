import json
import os
import sys
import tempfile
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestEventStream(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_create_topic_201(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/topics", {"topic": "clicks", "max_events": 100})
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["topic"], "clicks")
            self.assertEqual(resp["max_events"], 100)
            self.assertEqual(resp["head_offset"], 0)

    def test_create_topic_duplicate_409(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "clicks"})
            status, _, body = s.request("POST", "/topics", {"topic": "clicks"})
            self.assertEqual(status, 409)

    def test_publish_event_monotonic_offsets(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "orders"})
            st1, _, b1 = s.request("POST", "/topics/orders/publish", {"event_type": "created", "payload": {}})
            st2, _, b2 = s.request("POST", "/topics/orders/publish", {"event_type": "paid", "payload": {}})
            self.assertEqual(st1, 201)
            self.assertEqual(st2, 201)
            self.assertEqual(json.loads(b1.decode())["offset"], 0)
            self.assertEqual(json.loads(b2.decode())["offset"], 1)

    def test_publish_missing_topic_404(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, _ = s.request("POST", "/topics/nonexistent/publish", {"event_type": "test"})
            self.assertEqual(status, 404)

    def test_consume_auto_commit_advances_cursor(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "logs"})
            s.request("POST", "/topics/logs/publish", {"event_type": "e1", "payload": {}})
            s.request("POST", "/topics/logs/publish", {"event_type": "e2", "payload": {}})

            # First consume (auto_commit = True by default)
            status, _, body = s.request("POST", "/subscriptions/grp1/consume", {"topic": "logs", "limit": 1})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(len(resp["events"]), 1)
            self.assertEqual(resp["events"][0]["offset"], 0)
            self.assertEqual(resp["next_offset"], 1)

            # Second consume starts at offset 1
            status2, _, body2 = s.request("POST", "/subscriptions/grp1/consume", {"topic": "logs", "limit": 1})
            self.assertEqual(status2, 200)
            resp2 = json.loads(body2.decode())
            self.assertEqual(len(resp2["events"]), 1)
            self.assertEqual(resp2["events"][0]["offset"], 1)
            self.assertEqual(resp2["next_offset"], 2)

    def test_consume_no_auto_commit_keeps_cursor(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "logs"})
            s.request("POST", "/topics/logs/publish", {"event_type": "e1"})

            # Consume without auto_commit
            s.request("POST", "/subscriptions/grp1/consume", {"topic": "logs", "auto_commit": False})
            # Consume again: should return same event 0
            status, _, body = s.request("POST", "/subscriptions/grp1/consume", {"topic": "logs"})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["events"][0]["offset"], 0)

    def test_consume_limit(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "bulk"})
            for i in range(5):
                s.request("POST", "/topics/bulk/publish", {"event_type": f"e{i}"})

            status, _, body = s.request("POST", "/subscriptions/g1/consume", {"topic": "bulk", "limit": 3})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(len(resp["events"]), 3)

    def test_seek_offset(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "seekable"})
            for i in range(5):
                s.request("POST", "/topics/seekable/publish", {"event_type": f"e{i}"})

            # Seek to offset 3
            status, _, body = s.request("POST", "/subscriptions/g1/seek", {"topic": "seekable", "offset": 3})
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["committed_offset"], 3)

            # Next consume starts at 3
            status_c, _, body_c = s.request("POST", "/subscriptions/g1/consume", {"topic": "seekable"})
            self.assertEqual(status_c, 200)
            resp_c = json.loads(body_c.decode())
            self.assertEqual(resp_c["events"][0]["offset"], 3)

    def test_retention_trims_oldest_events(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # max_events is 2
            s.request("POST", "/topics", {"topic": "ring", "max_events": 2})
            s.request("POST", "/topics/ring/publish", {"event_type": "e0"}) # offset 0
            s.request("POST", "/topics/ring/publish", {"event_type": "e1"}) # offset 1
            s.request("POST", "/topics/ring/publish", {"event_type": "e2"}) # offset 2, e0 purged

            status, _, body = s.request("GET", "/topics/ring")
            self.assertEqual(status, 200)
            meta = json.loads(body.decode())
            self.assertEqual(meta["event_count"], 2)
            self.assertEqual(meta["earliest_offset"], 1)
            self.assertEqual(meta["next_offset"], 3)

    def test_multiple_groups_independent_cursors(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "shared"})
            s.request("POST", "/topics/shared/publish", {"event_type": "e0"})
            s.request("POST", "/topics/shared/publish", {"event_type": "e1"})

            # Group A consumes 2
            s.request("POST", "/subscriptions/grpA/consume", {"topic": "shared", "limit": 2})
            # Group B consumes 1
            status_b, _, body_b = s.request("POST", "/subscriptions/grpB/consume", {"topic": "shared", "limit": 1})
            self.assertEqual(status_b, 200)
            self.assertEqual(json.loads(body_b.decode())["next_offset"], 1)

    def test_get_topic_metadata(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/topics", {"topic": "info", "max_events": 50})
            status, _, body = s.request("GET", "/topics/info")
            self.assertEqual(status, 200)
            meta = json.loads(body.decode())
            self.assertEqual(meta["topic"], "info")
            self.assertEqual(meta["event_count"], 0)
            self.assertIsNone(meta["earliest_offset"])
            self.assertEqual(meta["next_offset"], 0)

    def test_persistence_across_restart(self):
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("POST", "/topics", {"topic": "persisted"})
            s1.request("POST", "/topics/persisted/publish", {"event_type": "hello"})

        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/topics/persisted")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["next_offset"], 1)

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "POST", "path": "/topics", "body": {"topic": "rep"}}) + "\n")
            f.write(json.dumps({"method": "POST", "path": "/topics/rep/publish", "body": {"event_type": "e"}}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 201)


if __name__ == "__main__":
    unittest.main()
