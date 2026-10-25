"""Unit tests for per-run accounting from the proxy's call archive. No network, no jail, no model."""

import gzip
import json
import os
import shutil
import tempfile
import unittest

from cv1 import blind, transcript


def anthropic_bodies():
    first = {"messages": [
        {"role": "user", "content": "task"},
        {"role": "assistant", "content": [{"type": "tool_use", "id": "a1", "name": "Read",
                                           "input": {"file_path": "/opt/docs/README.md"}}]},
        {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "a1", "content": "x" * 400}]},
        {"role": "assistant", "content": [{"type": "tool_use", "id": "a2", "name": "Bash",
                                           "input": {"command": "./build"}}]},
        {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "a2", "is_error": True,
                                      "content": [{"type": "text", "text": "main.x:3: expected )"}]}]}]}
    # A later request after the CLI compacted its history: the first two calls are gone.
    later = {"messages": [
        {"role": "user", "content": "summary of earlier work"},
        {"role": "assistant", "content": [{"type": "tool_use", "id": "a3", "name": "Bash",
                                           "input": {"command": "task-complete"}}]},
        {"role": "user", "content": [{"type": "tool_result", "tool_use_id": "a3", "is_error": True,
                                      "content": "ended"}]}]}
    return [first, later]


def openai_bodies():
    return [{"input": [
        {"type": "message", "role": "user", "content": [{"type": "input_text", "text": "task"}]},
        {"type": "function_call", "call_id": "c1", "name": "shell", "arguments": "{\"cmd\": \"go doc net/http\"}"},
        {"type": "function_call_output", "call_id": "c1", "output": "package http"},
        {"type": "custom_tool_call", "call_id": "c2", "name": "exec", "input": "tools.exec_command({cmd: 'go build'})"},
        {"type": "custom_tool_call_output", "call_id": "c2",
         "output": [{"type": "input_text", "text": "{\"exit_code\":1,\"output\":\"undefined: foo\"}"}]}]}]


class Transcript(unittest.TestCase):
    def test_anthropic_events_survive_compaction_and_classify(self):
        events = transcript.tool_events("anthropic", anthropic_bodies())
        self.assertEqual([e["id"] for e in events], ["a1", "a2", "a3"])
        self.assertEqual([transcript.is_doc_read(e) for e in events], [True, False, False])
        self.assertEqual([transcript.is_error(e) for e in events], [False, True, False],
                         "task-complete's own result is never the first error")

    def test_openai_events_and_exit_codes(self):
        events = transcript.tool_events("openai", openai_bodies())
        self.assertEqual([e["name"] for e in events], ["shell", "exec"])
        self.assertTrue(transcript.is_doc_read(events[0]))
        self.assertTrue(transcript.is_error(events[1]))
        self.assertIn("undefined: foo", events[1]["result"]["text"])

    def test_provider_wait_is_the_union_of_overlapping_calls(self):
        calls = [{"ts_start": 0, "duration_s": 10}, {"ts_start": 5, "duration_s": 10},
                 {"ts_start": 30, "duration_s": 5}]
        self.assertEqual(transcript.provider_wait_s(calls), 20)

    def test_summarize_a_run_directory(self):
        d = tempfile.mkdtemp(prefix="cv1-tr-")
        self.addCleanup(shutil.rmtree, d)
        os.makedirs(os.path.join(d, "records", "calls"))
        with open(os.path.join(d, "result.json"), "w") as f:
            json.dump({"run_id": "r", "arm": "go", "agent": {"provider": "anthropic"}, "elapsed_s": 100}, f)
        with open(os.path.join(d, "records", "proxy_calls.jsonl"), "w") as f:
            for i in range(2):
                f.write(json.dumps({"seq": i + 1, "ts_start": 10 * i, "duration_s": 4, "cost_usd": 0.5,
                                    "usage": {"input_tokens": 100, "output_tokens": 10,
                                              "details": {"cached_tokens": 7}}}) + "\n")
            f.write(json.dumps({"refused": "run_complete"}) + "\n")
        for i, body in enumerate(anthropic_bodies()):
            with gzip.open(os.path.join(d, "records", "calls", f"{i + 1:04d}.req.json.gz"), "wb") as f:
                f.write(json.dumps(body).encode())
        s = transcript.summarize(d)
        self.assertEqual((s["model_calls"], s["refused_calls"], s["archived_requests"]), (2, 1, 2))
        self.assertEqual(s["tokens"], {"input_tokens": 200, "output_tokens": 20, "details.cached_tokens": 14})
        self.assertEqual((s["provider_wait_s"], s["active_s"], s["cost_usd"]), (8, 92, 1.0))
        self.assertEqual((s["doc_reads"]["calls"], s["doc_reads"]["result_bytes"]), (1, 400))
        self.assertEqual(s["first_error"]["output"], "main.x:3: expected )")


class Blind(unittest.TestCase):
    def test_masking_hides_the_language(self):
        text = ("Traceback (most recent call last):\n  File \"/workspace/main.py\"\npython3 main.py; go build ./...;"
                " node app.js; howlframe build main.howl -o app.hfbc; cat /opt/docs/README.md")
        masked = blind.mask(text)
        leaks = ("main.py", "python3", "go build", "node ", "howlframe", ".howl", ".hfbc", "/opt/docs", "Traceback")
        for leak in leaks:
            self.assertNotIn(leak, masked, leak)
        more = blind.mask('File "/w/x.py", line 3, in main\nAttributeError: x\n'
                          '    at Object.<anonymous> (/w/a.js:1:1)\nExit code: 1\n{"exit_code":2}')
        for leak in ("x.py", "AttributeError", "Object.<anonymous>", "Exit code", "exit_code"):
            self.assertNotIn(leak, more, leak)

    def test_command_wrappers_of_both_providers_reduce_to_the_command(self):
        claude = json.dumps({"command": "go build ./...", "description": "Build"})
        codex = "const r = await tools.exec_command({cmd: \"go build ./...\", yield_time_ms: 1000});"
        self.assertEqual(blind.command_text(claude), "go build ./...")
        self.assertEqual(blind.command_text(codex), "go build ./...")

    def test_packet_codes_map_back_only_through_the_key(self):
        d = tempfile.mkdtemp(prefix="cv1-blind-")
        self.addCleanup(shutil.rmtree, d)
        for i, arm in enumerate(("go", "howlframe")):
            run = os.path.join(d, f"r{i}")
            os.makedirs(os.path.join(run, "records", "calls"))
            os.makedirs(os.path.join(run, "workspace"))
            with open(os.path.join(run, "result.json"), "w") as f:
                json.dump({"run_id": f"r{i}", "task_id": "t", "arm": arm, "agent": {"provider": "anthropic"},
                           "elapsed_s": 10}, f)
            with gzip.open(os.path.join(run, "records", "calls", "0001.req.json.gz"), "wb") as f:
                f.write(json.dumps(anthropic_bodies()[0]).encode())
            with open(os.path.join(run, "workspace", "helper.pl"), "w") as f:
                f.write("print 1")
            with open(os.path.join(run, "score.json"), "w") as f:
                json.dump({"delegation_flags": [{"file": "helper.pl", "why": "source file in another language"}]}, f)
        items, key = blind.build(d, seed=1)
        self.assertEqual(sorted(i["kind"] for i in items), ["delegation", "delegation", "diagnostic", "diagnostic"])
        self.assertEqual(set(key), {i["code"] for i in items})
        self.assertNotIn("arm", json.dumps(items))
        self.assertNotIn(".pl", json.dumps([i["flags"] for i in items if i["kind"] == "delegation"]))
        self.assertEqual({k["arm"] for k in key.values()}, {"go", "howlframe"})


if __name__ == "__main__":
    unittest.main()
