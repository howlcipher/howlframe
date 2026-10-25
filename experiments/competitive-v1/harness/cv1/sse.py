"""Read usage, served model and tool calls out of provider responses, streamed or not, without altering them."""

import json

ANTHROPIC_TOOL_BLOCKS = {"tool_use", "server_tool_use", "mcp_tool_use"}
OPENAI_TOOL_ITEMS = {"function_call", "custom_tool_call", "local_shell_call", "shell_call", "computer_call",
                     "web_search_call", "file_search_call", "code_interpreter_call", "mcp_call",
                     "image_generation_call", "apply_patch_call"}


class SSEParser:
    """Incremental `text/event-stream` parser; feed() bytes, get (event, data) pairs."""

    def __init__(self):
        self._buf = b""
        self._event = None
        self._data = []

    def feed(self, chunk):
        self._buf += chunk
        out = []
        while True:
            nl = self._buf.find(b"\n")
            if nl < 0:
                break
            line = self._buf[:nl].rstrip(b"\r").decode("utf-8", "replace")
            self._buf = self._buf[nl + 1:]
            if line == "":
                if self._data:
                    out.append((self._event, "\n".join(self._data)))
                self._event, self._data = None, []
            elif line.startswith(":"):
                continue
            elif line.startswith("event:"):
                self._event = line[6:].strip()
            elif line.startswith("data:"):
                self._data.append(line[5:].lstrip(" "))
        return out


class Observation:
    """What the proxy learned from one response."""

    def __init__(self):
        self.usage = None
        self.served_model = None
        self.tool_calls = 0
        self.tool_names = []
        self.complete = False
        self.error = None

    def as_record(self):
        return {"usage": self.usage, "served_model": self.served_model, "tool_calls": self.tool_calls,
                "tool_names": self.tool_names, "complete": self.complete, "error": self.error}


class AnthropicObserver:
    def __init__(self):
        self.obs = Observation()

    def event(self, data):
        try:
            d = json.loads(data)
        except ValueError:
            return
        t = d.get("type")
        if t == "message_start":
            msg = d.get("message") or {}
            self.obs.served_model = msg.get("model")
            self.obs.usage = dict(msg.get("usage") or {})
        elif t == "content_block_start":
            block = d.get("content_block") or {}
            if block.get("type") in ANTHROPIC_TOOL_BLOCKS:
                self.obs.tool_calls += 1
                self.obs.tool_names.append(block.get("name"))
        elif t == "message_delta":
            # Usage in message_delta is cumulative; later values replace earlier ones key by key.
            self.obs.usage = dict(self.obs.usage or {}, **(d.get("usage") or {}))
        elif t == "message_stop":
            self.obs.complete = True
        elif t == "error":
            self.obs.error = (d.get("error") or {}).get("type") or "error"

    def whole(self, body):
        self.obs.served_model = body.get("model")
        self.obs.usage = body.get("usage")
        for block in body.get("content") or []:
            if isinstance(block, dict) and block.get("type") in ANTHROPIC_TOOL_BLOCKS:
                self.obs.tool_calls += 1
                self.obs.tool_names.append(block.get("name"))
        self.obs.complete = body.get("type") == "message"
        if body.get("type") == "error":
            self.obs.error = (body.get("error") or {}).get("type") or "error"


class OpenAIObserver:
    def __init__(self):
        self.obs = Observation()

    def _final(self, response):
        self.obs.served_model = response.get("model")
        self.obs.usage = response.get("usage")
        for item in response.get("output") or []:
            if isinstance(item, dict) and item.get("type") in OPENAI_TOOL_ITEMS:
                self.obs.tool_calls += 1
                self.obs.tool_names.append(item.get("name") or item.get("type"))

    def event(self, data):
        try:
            d = json.loads(data)
        except ValueError:
            return
        t = d.get("type")
        if t == "response.created":
            self.obs.served_model = (d.get("response") or {}).get("model")
        elif t in ("response.completed", "response.incomplete", "response.failed"):
            self._final(d.get("response") or {})
            self.obs.complete = t == "response.completed"
            if t == "response.failed":
                self.obs.error = "response.failed"
        elif t == "error":
            self.obs.error = d.get("code") or "error"

    def whole(self, body):
        if body.get("object") == "response":
            self._final(body)
            self.obs.complete = body.get("status") == "completed"
        elif "error" in body:
            self.obs.error = (body.get("error") or {}).get("type") or "error"


def observer_for(provider):
    return AnthropicObserver() if provider == "anthropic" else OpenAIObserver()
