"""Deterministic mock of the Anthropic Messages API and the OpenAI Responses API.

It exists so the harness, proxy and cap enforcement can be validated with **no paid model call**. Each scenario
is a JSON file:

    {
      "steps": [{"shell": "echo hi > out.txt"}, {"shell": "task-complete"}, {"text": "done"}],
      "usage": {"input_tokens": 1000, "cache_creation_input_tokens": 0, "cache_read_input_tokens": 0,
                "output_tokens": 200},
      "served_model": null
    }

`step_sets` may replace `steps`: a list of {"match": text, "steps": [...]}; the first set whose text occurs in the
request body is used, so one mock can script different arms of a co-scheduled group.

For a conversation request (one that offers tools), the step is chosen by how many tool results the
conversation already holds, so a CLI that runs each command and reports back walks through the script. A
request without tools (a CLI side call, such as a title) gets a short text answer. Every request is appended to
the record file as one JSON line (path, model, tool names, stream flag, body size) for later inspection.
"""

import argparse
import json
import sys
import threading
import time
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer

_lock = threading.Lock()


def _anthropic_step_index(body):
    n = 0
    for msg in body.get("messages", []):
        content = msg.get("content")
        if isinstance(content, list):
            n += sum(1 for block in content if isinstance(block, dict) and block.get("type") == "tool_result")
    return n


def _openai_step_index(body):
    items = body.get("input")
    if not isinstance(items, list):
        return 0
    return sum(1 for it in items if isinstance(it, dict) and str(it.get("type", "")).endswith("_call_output"))


def _openai_tools(body):
    """Tool definitions from `tools` and from Codex's `additional_tools` input items, namespaces flattened."""
    found = []

    def walk(items):
        for t in items or []:
            if not isinstance(t, dict):
                continue
            if t.get("type") == "namespace":
                walk(t.get("tools"))
            else:
                found.append(t)

    walk(body.get("tools"))
    for it in body.get("input") or []:
        if isinstance(it, dict) and it.get("type") == "additional_tools":
            walk(it.get("tools"))
    return found


def _steps_for(scenario, raw):
    text = raw.decode("utf-8", "replace") if isinstance(raw, bytes) else raw
    for entry in scenario.get("step_sets") or []:
        if entry["match"] in text:
            return entry["steps"]
    return scenario.get("steps") or [{"text": "ok"}]


def _shell_tool(tools, preferred):
    names = []
    for t in tools or []:
        name = t.get("name") or (t.get("function") or {}).get("name") or t.get("type")
        if name:
            names.append(name)
    for want in preferred:
        if want in names:
            return want
    return names[0] if names else None


class MockState:
    def __init__(self, scenario, record_path, dump_dir=None):
        self.scenario = scenario
        self.record_path = record_path
        self.dump_dir = dump_dir
        self.counter = 0

    def next_id(self, prefix):
        with _lock:
            self.counter += 1
            return f"{prefix}_mock{self.counter:06d}"

    def record(self, entry):
        if not self.record_path:
            return
        with _lock:
            with open(self.record_path, "a") as f:
                f.write(json.dumps(entry, sort_keys=True) + "\n")


class Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    state = None

    def log_message(self, fmt, *args):
        pass

    def _read_body(self):
        length = int(self.headers.get("Content-Length") or 0)
        raw = self.rfile.read(length) if length else b""
        try:
            return raw, json.loads(raw or b"{}")
        except ValueError:
            return raw, {}

    def _send_json(self, code, obj):
        data = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def _start_sse(self):
        self.send_response(200)
        self.send_header("Content-Type", "text/event-stream")
        self.send_header("Cache-Control", "no-cache")
        self.send_header("Connection", "close")
        self.close_connection = True
        self.end_headers()

    def _sse(self, event, data):
        self.wfile.write(f"event: {event}\ndata: {json.dumps(data)}\n\n".encode())
        self.wfile.flush()

    def do_GET(self):
        self.state.record({"method": "GET", "path": self.path})
        if self.path.startswith("/v1/models"):
            model = self.state.scenario.get("served_model") or "mock-model"
            self._send_json(200, {"data": [{"id": model, "type": "model"}], "object": "list"})
            return
        self._send_json(404, {"error": {"type": "not_found", "message": "mock: no such route"}})

    def do_HEAD(self):
        self.send_response(200)
        self.send_header("Content-Length", "0")
        self.end_headers()

    def do_POST(self):
        raw, body = self._read_body()
        path = self.path.split("?", 1)[0]
        tools = body.get("tools") or []
        self.state.record({
            "method": "POST", "path": self.path, "model": body.get("model"), "stream": bool(body.get("stream")),
            "tools": [t.get("name") or t.get("type") for t in (tools or _openai_tools(body)) if isinstance(t, dict)],
            "body_bytes": len(raw), "max_tokens": body.get("max_tokens") or body.get("max_output_tokens"),
            "headers": {k.lower(): v for k, v in self.headers.items()
                        if k.lower() in ("anthropic-version", "anthropic-beta", "user-agent", "openai-beta")},
            "auth_present": bool(self.headers.get("x-api-key") or self.headers.get("authorization")),
            "auth_is_real_marker": (self.headers.get("x-api-key") == "REAL-KEY-MARKER"
                                    or self.headers.get("authorization") == "Bearer REAL-KEY-MARKER"),
        })
        if self.state.dump_dir:
            with open(f"{self.state.dump_dir}/{self.state.next_id('req')}.json", "wb") as f:
                f.write(raw)
        delay = float(self.state.scenario.get("delay_seconds", 0))
        if delay:
            time.sleep(delay)
        if path == "/v1/messages/count_tokens":
            self._send_json(200, {"input_tokens": max(1, len(raw) // 4)})
        elif path == "/v1/messages":
            self._anthropic(body, raw)
        elif path == "/v1/responses":
            self._openai(body, raw)
        else:
            self._send_json(404, {"error": {"type": "not_found", "message": "mock: no such route"}})

    # Anthropic Messages API -------------------------------------------------------------------------------
    def _anthropic(self, body, raw=b""):
        sc = self.state.scenario
        model = sc.get("served_model") or body.get("model") or "mock-model"
        usage = dict(sc.get("usage") or {})
        start_usage = {
            "input_tokens": usage.get("input_tokens", 100),
            "cache_creation_input_tokens": usage.get("cache_creation_input_tokens", 0),
            "cache_read_input_tokens": usage.get("cache_read_input_tokens", 0),
            "output_tokens": 1,
        }
        out_tokens = usage.get("output_tokens", 10)
        steps = _steps_for(sc, raw)
        tools = body.get("tools") or []
        if tools:
            idx = _anthropic_step_index(body)
            step = steps[idx] if idx < len(steps) else steps[-1]
        else:
            step = {"text": "Mock title"}
        tool_name = _shell_tool(tools, ("Bash",)) if "shell" in step else None
        msg_id = self.state.next_id("msg")
        if "shell" in step and tool_name:
            block = {"type": "tool_use", "id": self.state.next_id("toolu"), "name": tool_name, "input": {}}
            payload = json.dumps({"command": step["shell"], "description": "mock step"})
            stop = "tool_use"
        else:
            block = {"type": "text", "text": ""}
            payload = step.get("text", "ok")
            stop = "end_turn"
        if not body.get("stream"):
            content = dict(block)
            if block["type"] == "tool_use":
                content["input"] = json.loads(payload)
            else:
                content["text"] = payload
            final_usage = dict(start_usage, output_tokens=out_tokens)
            self._send_json(200, {"id": msg_id, "type": "message", "role": "assistant", "model": model,
                                  "content": [content], "stop_reason": stop, "stop_sequence": None,
                                  "usage": final_usage})
            return
        self._start_sse()
        self._sse("message_start", {"type": "message_start", "message": {
            "id": msg_id, "type": "message", "role": "assistant", "model": model, "content": [],
            "stop_reason": None, "stop_sequence": None, "usage": start_usage}})
        self._sse("content_block_start", {"type": "content_block_start", "index": 0, "content_block": block})
        if block["type"] == "tool_use":
            delta = {"type": "input_json_delta", "partial_json": payload}
        else:
            delta = {"type": "text_delta", "text": payload}
        self._sse("content_block_delta", {"type": "content_block_delta", "index": 0, "delta": delta})
        self._sse("content_block_stop", {"type": "content_block_stop", "index": 0})
        self._sse("message_delta", {"type": "message_delta", "delta": {"stop_reason": stop, "stop_sequence": None},
                                    "usage": {"output_tokens": out_tokens}})
        self._sse("message_stop", {"type": "message_stop"})

    # OpenAI Responses API ---------------------------------------------------------------------------------
    def _openai(self, body, raw=b""):
        sc = self.state.scenario
        model = sc.get("served_model") or body.get("model") or "mock-model"
        usage = dict(sc.get("usage") or {})
        steps = _steps_for(sc, raw)
        tools = _openai_tools(body)
        idx = _openai_step_index(body)
        step = (steps[idx] if idx < len(steps) else steps[-1]) if tools else {"text": "Mock title"}
        resp_id = self.state.next_id("resp")
        tool_name = _shell_tool(tools, ("exec", "exec_command", "shell", "local_shell")) if "shell" in step \
            else None
        if "shell" in step and tool_name == "exec":
            # Codex code mode: the only model-facing tool runs JavaScript that calls the nested shell tool.
            js = (f"const r = await tools.exec_command({{cmd: {json.dumps(step['shell'])}}});\n"
                  "text(typeof r === 'string' ? r : JSON.stringify(r));")
            item = {"type": "custom_tool_call", "id": self.state.next_id("ctc"), "call_id": self.state.next_id("call"),
                    "name": "exec", "input": js, "status": "completed"}
        elif "shell" in step and tool_name:
            if tool_name == "exec_command":
                args = {"cmd": step["shell"]}
            else:
                args = {"command": ["bash", "-lc", step["shell"]]}
            item = {"type": "function_call", "id": self.state.next_id("fc"), "call_id": self.state.next_id("call"),
                    "name": tool_name, "arguments": json.dumps(args), "status": "completed"}
        else:
            item = {"type": "message", "id": self.state.next_id("msg"), "role": "assistant", "status": "completed",
                    "content": [{"type": "output_text", "text": step.get("text", "ok"), "annotations": []}]}
        in_tok = usage.get("input_tokens", 100)
        response = {
            "id": resp_id, "object": "response", "created_at": int(time.time()), "status": "completed", "model": model,
            "output": [item],
            "usage": {"input_tokens": in_tok,
                      "input_tokens_details": {"cached_tokens": usage.get("cached_tokens", 0)},
                      "output_tokens": usage.get("output_tokens", 10),
                      "output_tokens_details": {"reasoning_tokens": usage.get("reasoning_tokens", 0)},
                      "total_tokens": in_tok + usage.get("output_tokens", 10)},
        }
        if not body.get("stream"):
            self._send_json(200, response)
            return
        self._start_sse()
        created = dict(response, status="in_progress", output=[], usage=None)
        seq = 0

        def ev(kind, data):
            nonlocal seq
            data = dict(data, type=kind, sequence_number=seq)
            seq += 1
            self._sse(kind, data)

        ev("response.created", {"response": created})
        ev("response.in_progress", {"response": created})
        ev("response.output_item.added", {"output_index": 0, "item": dict(item, status="in_progress")})
        if item["type"] == "message":
            ev("response.content_part.added", {"item_id": item["id"], "output_index": 0, "content_index": 0,
                                               "part": {"type": "output_text", "text": "", "annotations": []}})
            text = item["content"][0]["text"]
            ev("response.output_text.delta", {"item_id": item["id"], "output_index": 0, "content_index": 0,
                                              "delta": text})
            ev("response.output_text.done", {"item_id": item["id"], "output_index": 0, "content_index": 0,
                                             "text": text})
            ev("response.content_part.done", {"item_id": item["id"], "output_index": 0, "content_index": 0,
                                              "part": item["content"][0]})
        elif item["type"] == "custom_tool_call":
            ev("response.custom_tool_call_input.delta", {"item_id": item["id"], "output_index": 0,
                                                         "delta": item["input"]})
            ev("response.custom_tool_call_input.done", {"item_id": item["id"], "output_index": 0,
                                                        "input": item["input"]})
        else:
            ev("response.function_call_arguments.delta", {"item_id": item["id"], "output_index": 0,
                                                          "delta": item["arguments"]})
            ev("response.function_call_arguments.done", {"item_id": item["id"], "output_index": 0,
                                                         "arguments": item["arguments"]})
        ev("response.output_item.done", {"output_index": 0, "item": item})
        ev("response.completed", {"response": response})


def make_server(scenario, record_path=None, host="127.0.0.1", port=0, dump_dir=None):
    handler = type("BoundHandler", (Handler,), {"state": MockState(scenario, record_path, dump_dir)})
    return ThreadingHTTPServer((host, port), handler)


def main(argv=None):
    p = argparse.ArgumentParser(description=__doc__.splitlines()[0])
    p.add_argument("--scenario", required=True)
    p.add_argument("--record")
    p.add_argument("--port", type=int, default=0)
    p.add_argument("--port-file")
    p.add_argument("--dump-dir", help="also save each raw request body here (debugging only)")
    args = p.parse_args(argv)
    with open(args.scenario) as f:
        scenario = json.load(f)
    server = make_server(scenario, args.record, port=args.port, dump_dir=args.dump_dir)
    if args.port_file:
        with open(args.port_file, "w") as f:
            f.write(str(server.server_address[1]))
    server.serve_forever()
    return 0


if __name__ == "__main__":
    sys.exit(main())
