import json
import os
import sys
import tempfile
import time
import unittest

sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), "../../../kit")))
import testkit


class TestJobQueue(unittest.TestCase):
    def setUp(self):
        self.temp_dir = tempfile.TemporaryDirectory()
        self.store_path = os.path.join(self.temp_dir.name, "store.json")

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_enqueue_job_201(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, body = s.request("POST", "/jobs", {"topic": "mail", "payload": {"foo": "bar"}})
            self.assertEqual(status, 201)
            resp = json.loads(body.decode())
            self.assertEqual(resp["job_id"], "job-1")
            self.assertEqual(resp["status"], "pending")
            self.assertEqual(resp["priority"], 0)

    def test_lease_empty_queue_204(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            status, _, _ = s.request("POST", "/jobs/lease", {"topic": "empty", "worker_id": "w1"})
            self.assertEqual(status, 204)

    def test_lease_available_job_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "t1", "payload": {}})
            status, _, body = s.request("POST", "/jobs/lease", {"topic": "t1", "worker_id": "w1"})
            self.assertEqual(status, 200)
            job = json.loads(body.decode())
            self.assertEqual(job["status"], "leased")
            self.assertEqual(job["leased_by"], "w1")
            self.assertEqual(job["attempts"], 1)

    def test_priority_scheduling(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "prio", "payload": {}, "priority": 5})
            s.request("POST", "/jobs", {"topic": "prio", "payload": {}, "priority": 100})
            status, _, body = s.request("POST", "/jobs/lease", {"topic": "prio", "worker_id": "w1"})
            self.assertEqual(status, 200)
            job = json.loads(body.decode())
            self.assertEqual(job["priority"], 100)
            self.assertEqual(job["job_id"], "job-2")

    def test_fifo_tie_breaking_for_same_priority(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "fifo", "payload": {"n": 1}, "priority": 10})
            s.request("POST", "/jobs", {"topic": "fifo", "payload": {"n": 2}, "priority": 10})
            status, _, body = s.request("POST", "/jobs/lease", {"topic": "fifo", "worker_id": "w1"})
            self.assertEqual(status, 200)
            job = json.loads(body.decode())
            self.assertEqual(job["payload"]["n"], 1)

    def test_ack_success_200(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "ack", "payload": {}})
            s.request("POST", "/jobs/lease", {"topic": "ack", "worker_id": "w1"})
            status, _, body = s.request("POST", "/jobs/job-1/ack", {"worker_id": "w1"})
            self.assertEqual(status, 200)
            status_get, _, body_get = s.request("GET", "/jobs/job-1")
            self.assertEqual(status_get, 200)
            self.assertEqual(json.loads(body_get.decode())["status"], "completed")

    def test_ack_wrong_worker_409(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "ack_err", "payload": {}})
            s.request("POST", "/jobs/lease", {"topic": "ack_err", "worker_id": "w1"})
            status, _, body = s.request("POST", "/jobs/job-1/ack", {"worker_id": "w_impostor"})
            self.assertEqual(status, 409)

    def test_fail_retries_remaining(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "retry", "payload": {}, "max_retries": 3})
            s.request("POST", "/jobs/lease", {"topic": "retry", "worker_id": "w1"}) # attempt 1
            status, _, body = s.request("POST", "/jobs/job-1/fail", {"worker_id": "w1"})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["status"], "pending")
            self.assertEqual(resp["attempts"], 1)

    def test_fail_dead_letter(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "dlq", "payload": {}, "max_retries": 1})
            s.request("POST", "/jobs/lease", {"topic": "dlq", "worker_id": "w1"}) # attempt 1 of 1
            status, _, body = s.request("POST", "/jobs/job-1/fail", {"worker_id": "w1"})
            self.assertEqual(status, 200)
            resp = json.loads(body.decode())
            self.assertEqual(resp["status"], "dead_letter")

    def test_lease_expiration(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            # 1 second lease
            s.request("POST", "/jobs", {"topic": "exp", "payload": {}, "lease_seconds": 1})
            s.request("POST", "/jobs/lease", {"topic": "exp", "worker_id": "w1"})
            time.sleep(1.2)
            # Worker 2 should be able to lease it because lease expired
            status, _, body = s.request("POST", "/jobs/lease", {"topic": "exp", "worker_id": "w2"})
            self.assertEqual(status, 200)
            job = json.loads(body.decode())
            self.assertEqual(job["leased_by"], "w2")
            self.assertEqual(job["attempts"], 2)

    def test_get_stats(self):
        with testkit.Server(args=["--store", self.store_path]) as s:
            s.request("POST", "/jobs", {"topic": "st", "payload": {}}) # pending
            s.request("POST", "/jobs", {"topic": "st", "payload": {}})
            s.request("POST", "/jobs/lease", {"topic": "st", "worker_id": "w1"}) # leased
            status, _, body = s.request("GET", "/stats")
            self.assertEqual(status, 200)
            stats = json.loads(body.decode())
            self.assertEqual(stats["pending"], 1)
            self.assertEqual(stats["leased"], 1)
            self.assertEqual(stats["completed"], 0)
            self.assertEqual(stats["dead_letter"], 0)

    def test_persistence_across_restarts(self):
        with testkit.Server(args=["--store", self.store_path]) as s1:
            s1.request("POST", "/jobs", {"topic": "persist", "payload": {"foo": "bar"}})

        with testkit.Server(args=["--store", self.store_path]) as s2:
            status, _, body = s2.request("GET", "/jobs/job-1")
            self.assertEqual(status, 200)
            self.assertEqual(json.loads(body.decode())["topic"], "persist")

    def test_batch_replay_mode(self):
        reqs_path = os.path.join(self.temp_dir.name, "reqs.jsonl")
        with open(reqs_path, "w") as f:
            f.write(json.dumps({"method": "POST", "path": "/jobs", "body": {"topic": "r"}}) + "\n")
            f.write(json.dumps({"method": "POST", "path": "/jobs/lease", "body": {"topic": "r", "worker_id": "w"}}) + "\n")

        res = testkit.run(["replay", "--store", self.store_path, reqs_path])
        self.assertEqual(res.returncode, 0)
        lines = [json.loads(l) for l in res.stdout.strip().split("\n") if l.strip()]
        self.assertEqual(len(lines), 2)
        self.assertEqual(lines[0]["status"], 201)
        self.assertEqual(lines[1]["status"], 200)


if __name__ == "__main__":
    unittest.main()
