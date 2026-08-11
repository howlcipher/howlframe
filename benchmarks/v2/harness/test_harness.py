import unittest
import os
import tempfile
import subprocess
import json
import run_track_a
import aggregate

class TestHarness(unittest.TestCase):
    def setUp(self):
        subprocess.run(["go", "build", "-o", "howlframe", "."], cwd="../../../", check=True)
        os.environ["PATH"] = os.path.abspath("../../../") + os.pathsep + os.environ.get("PATH", "")
        self.temp_dir = tempfile.TemporaryDirectory()
        self.results_dir = os.path.join(self.temp_dir.name, "results")
        os.makedirs(self.results_dir)

    def tearDown(self):
        self.temp_dir.cleanup()

    def test_schema_validation(self):
        # Just check that schema parses
        schema_path = os.path.abspath(os.path.join(os.path.dirname(__file__), "..", "schema.json"))
        with open(schema_path) as f:
            schema = json.load(f)
        self.assertIn("properties", schema)

    def test_known_good_passes(self):
        ref = os.path.abspath("../reference/task_01_typed_function/python/test_app.py")
        out = os.path.join(self.results_dir, "t1.json")
        subprocess.run(["python3", "run_track_a.py", "--task", "task_01_typed_function", "--language", "python", "--sources", ref, "--model", "test-model", "--out", out], check=True)
        with open(out) as f:
            data = json.load(f)
        self.assertTrue(data["metrics"]["first_pass_success"])
        self.assertEqual(data["metrics"]["repair_attempts"], 0)

    def test_broken_candidate_fails(self):
        broken = os.path.join(self.temp_dir.name, "broken.py")
        with open(broken, "w") as f: f.write("def add(a, b): return a - b\nimport unittest\nclass T(unittest.TestCase):\n def test_a(self): self.assertEqual(add(2,3), 5)\nif __name__=='__main__': unittest.main()")
        out = os.path.join(self.results_dir, "t2.json")
        subprocess.run(["python3", "run_track_a.py", "--task", "task_01_typed_function", "--language", "python", "--sources", broken, "--model", "test-model", "--out", out], check=True)
        with open(out) as f:
            data = json.load(f)
        self.assertFalse(data["metrics"]["first_pass_success"])
        self.assertFalse(data["metrics"]["eventual_success"])

    def test_repair_progression(self):
        broken = os.path.join(self.temp_dir.name, "broken.py")
        with open(broken, "w") as f: f.write("syntax error")
        ref = os.path.abspath("../reference/task_01_typed_function/python/test_app.py")
        out = os.path.join(self.results_dir, "t3.json")
        subprocess.run(["python3", "run_track_a.py", "--task", "task_01_typed_function", "--language", "python", "--sources", broken, ref, "--model", "test-model", "--out", out], check=True)
        with open(out) as f:
            data = json.load(f)
        self.assertFalse(data["metrics"]["first_pass_success"])
        self.assertTrue(data["metrics"]["eventual_success"])
        self.assertEqual(data["metrics"]["repair_attempts"], 1)

    def test_unsupported_task(self):
        out = os.path.join(self.results_dir, "t4.json")
        subprocess.run(["python3", "run_track_a.py", "--task", "task_08_capability_restricted", "--language", "python", "--sources", "foo", "--model", "test-model", "--out", out], check=True)
        with open(out) as f:
            data = json.load(f)
        self.assertEqual(data["notes"], "unsupported")
        self.assertEqual(data["metrics"]["verification_result"], "error")

    def test_aggregation(self):
        times = [10.0, 20.0, 30.0, 40.0, 50.0, 60.0, 70.0, 80.0, 90.0, 100.0]
        self.assertEqual(aggregate.median(times), 55.0)
        self.assertEqual(aggregate.p90(times), 100.0)
        self.assertEqual(aggregate.p90([1,2,3]), "n/a")
        
    def test_capability_denial(self):
        ref = os.path.abspath("../reference/task_08_capability_restricted/howlframe/app.howl")
        out = os.path.join(self.results_dir, "t5.json")
        subprocess.run(["python3", "run_track_a.py", "--task", "task_08_capability_restricted", "--language", "howlframe", "--sources", ref, "--model", "test-model", "--out", out], check=True)
        with open(out) as f:
            data = json.load(f)
        self.assertTrue(data["metrics"]["first_pass_success"])
        self.assertEqual(data["metrics"]["capability_denial_behavior"], "Gracefully caught Access Denied")

    def test_all_references(self):
        ref_dir = os.path.abspath("../reference")
        for task in os.listdir(ref_dir):
            task_dir = os.path.join(ref_dir, task)
            if not os.path.isdir(task_dir): continue
            for lang in os.listdir(task_dir):
                lang_dir = os.path.join(task_dir, lang)
                if not os.path.isdir(lang_dir): continue
                out = os.path.join(self.results_dir, f"ref_{task}_{lang}.json")
                src = lang_dir
                if lang == "howlframe": src = os.path.join(lang_dir, "app.howl")
                if task == "task_03_http_json_endpoint" and lang == "howlframe": src = os.path.join(lang_dir, "server.howl")
                if task == "task_04_multi_module_cli" and lang == "howlframe": src = lang_dir
                if not os.path.exists(src): continue
                
                subprocess.run(["python3", "run_track_a.py", "--task", task, "--language", lang, "--sources", src, "--model", "test-model", "--out", out], check=True)
                with open(out) as f:
                    data = json.load(f)
                self.assertTrue(data["metrics"]["first_pass_success"], f"Reference failed: {task} {lang}")

if __name__ == '__main__':
    unittest.main()
