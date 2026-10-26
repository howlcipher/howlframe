"""Metering, cap-enforcing reverse proxy between a jailed agent CLI and its model provider.

One `RunProxy` serves one run on its own Unix socket (the jail reaches it through `forwarder.py`). All runs of
an experiment share one `Budget`. For every request the proxy, in order:

1. accepts only the provider's model-call route, an uncompressed JSON body and the pinned model;
2. refuses server-side tools (web search, web fetch, code execution, MCP connectors), background mode and
   premium speed tiers, so every arm has the same local-only tool surface and list-price billing;
3. reserves the call's worst-case cost against the run ceiling and the experiment cap, or refuses;
4. replaces the dummy credential with the real key (which never enters the jail), forwards the call and
   streams the response back unchanged while reading usage, served model and tool calls from it;
5. charges the actual cost (the reserved worst case if usage is missing), records the call in the ledger and
   checks the served model and the input-token bound.

Once the agent has run `task-complete`, every further call is refused unpaid. A refused request gets an HTTP 400
in the provider's error shape, and the `on_event` callback lets the runner end the run at once, whatever the CLI's
retry behaviour.
"""

import gzip
import hashlib
import http.client
import json
import os
import socket
import socketserver
import threading
import time
from http.server import BaseHTTPRequestHandler
from urllib.parse import urlsplit

from . import pricing
from .budget import BudgetRefused
from .sse import SSEParser, observer_for

ROUTES = {
    "anthropic": {"/v1/messages": "bill", "/v1/messages/count_tokens": "free"},
    "openai": {"/v1/responses": "bill"},
}
OPENAI_ALLOWED_TOOL_TYPES = {"function", "custom", "namespace", "local_shell"}
HOP_BY_HOP = {"connection", "keep-alive", "proxy-authenticate", "proxy-authorization", "te", "trailer",
              "transfer-encoding", "upgrade", "host", "content-length", "accept-encoding"}
CREDENTIAL_HEADERS = {"x-api-key", "authorization"}
OPAQUE_KEYS = {"encrypted_content", "signature"}
SETTLE_WAIT_S = 30.0


class PolicyRefused(Exception):
    def __init__(self, reason, detail=""):
        super().__init__(f"{reason}: {detail}" if detail else reason)
        self.reason = reason
        self.detail = detail


def _walk_openai_tools(body):
    found = []

    def walk(items):
        for t in items or []:
            if isinstance(t, dict):
                found.append(t)
                if t.get("type") == "namespace":
                    walk(t.get("tools"))

    walk(body.get("tools"))
    for it in body.get("input") or []:
        if isinstance(it, dict) and it.get("type") == "additional_tools":
            walk(it.get("tools"))
    return found


def opaque_bytes(obj):
    """Bytes held in opaque fields (encrypted reasoning, signatures, redacted thinking)."""
    total = 0
    stack = [obj]
    while stack:
        cur = stack.pop()
        if isinstance(cur, dict):
            for k, v in cur.items():
                redacted = k == "data" and cur.get("type") == "redacted_thinking"
                if isinstance(v, str) and (k in OPAQUE_KEYS or redacted):
                    total += len(v)
                elif isinstance(v, (dict, list)):
                    stack.append(v)
        elif isinstance(cur, list):
            stack.extend(cur)
    return total


def check_request_policy(provider, pinned_model, body):
    """Raise PolicyRefused unless the request is a plain call to the pinned model with local tools only."""
    if body.get("model") != pinned_model:
        raise PolicyRefused("wrong_model", f"{body.get('model')!r} is not the pinned {pinned_model!r}")
    if provider == "anthropic":
        for t in body.get("tools") or []:
            if isinstance(t, dict) and t.get("type") not in (None, "custom"):
                raise PolicyRefused("server_tool", str(t.get("type")))
        if body.get("mcp_servers"):
            raise PolicyRefused("server_tool", "mcp_servers")
        if body.get("speed") not in (None, "standard"):
            raise PolicyRefused("speed_tier", str(body.get("speed")))
        if body.get("inference_geo") not in (None, "", "global"):
            raise PolicyRefused("inference_geo", str(body.get("inference_geo")))
    else:
        for t in _walk_openai_tools(body):
            if t.get("type") not in OPENAI_ALLOWED_TOOL_TYPES:
                raise PolicyRefused("server_tool", str(t.get("type")))
        if body.get("background"):
            raise PolicyRefused("background_mode")
        if body.get("service_tier") not in (None, "auto", "default"):
            raise PolicyRefused("speed_tier", str(body.get("service_tier")))


class _UnixHTTPServer(socketserver.ThreadingMixIn, socketserver.UnixStreamServer):
    daemon_threads = True
    allow_reuse_address = True


class RunProxy:
    def __init__(self, run_id, provider, pinned_model, budget, prices, upstream_url, api_key, socket_path,
                 record_dir, allowed_served_models=None, on_event=None, upstream_timeout=900, complete_marker=None):
        self.run_id = run_id
        self.provider = provider
        self.pinned_model = pinned_model
        self.budget = budget
        self.prices = prices
        self.entry = pricing.model_entry(prices, pinned_model)
        u = urlsplit(upstream_url)
        self.upstream_scheme = u.scheme
        self.upstream_host = u.hostname
        self.upstream_port = u.port or (443 if u.scheme == "https" else 80)
        self.upstream_base = u.path.rstrip("/")
        self.api_key = api_key
        self.socket_path = socket_path
        self.record_dir = record_dir
        self.allowed_served = set(allowed_served_models or [pinned_model])
        self.on_event = on_event or (lambda kind, info: None)
        self.upstream_timeout = upstream_timeout
        self.complete_marker = complete_marker
        self.blocked = None
        self.first_request_saved = False
        self.calls = 0
        self.archived = 0
        self._lock = threading.Lock()
        os.makedirs(record_dir, exist_ok=True)
        if os.path.exists(socket_path):
            os.unlink(socket_path)
        handler = type("RunHandler", (_Handler,), {"proxy": self})
        self.server = _UnixHTTPServer(socket_path, handler)
        os.chmod(socket_path, 0o666)
        self.thread = threading.Thread(target=self.server.serve_forever, daemon=True)

    def start(self):
        self.thread.start()
        return self

    def close(self):
        self.server.shutdown()
        self.server.server_close()
        try:
            os.unlink(self.socket_path)
        except FileNotFoundError:
            pass

    def block(self, reason):
        with self._lock:
            if not self.blocked:
                self.blocked = reason

    def _log(self, entry):
        with self._lock:
            with open(os.path.join(self.record_dir, "proxy_calls.jsonl"), "a") as f:
                f.write(json.dumps(entry, sort_keys=True) + "\n")

    def open_archive(self, raw):
        """Archive one forwarded call: calls/NNNN.req.json.gz (the exact body) and NNNN.resp.gz (the response bytes
        as relayed). Token and documentation-reading accounting, active time and first-error extraction are
        computed from these after the run (cv1/transcript.py); they cannot be recovered any other way."""
        with self._lock:
            self.archived += 1
            seq = self.archived
        d = os.path.join(self.record_dir, "calls")
        os.makedirs(d, exist_ok=True)
        base = os.path.join(d, f"{seq:04d}")
        with gzip.open(base + ".req.json.gz", "wb") as f:
            f.write(raw)
        return seq, gzip.open(base + ".resp.gz", "wb")

    def save_first_request(self, raw, body):
        with self._lock:
            if self.first_request_saved:
                return
            self.first_request_saved = True
        with open(os.path.join(self.record_dir, "first_request.json"), "wb") as f:
            f.write(raw)
        # The agent's fixed surface: system prompt, developer messages and tool definitions. The task prompt and
        # environment description are user messages and are excluded, so arms of one provider must match.
        # Per-session item IDs are random, so they are dropped before hashing.
        developer = [{k: v for k, v in m.items() if k != "id"} for m in body.get("input") or []
                     if isinstance(m, dict) and m.get("role") == "developer" and m.get("type") != "additional_tools"]
        surface = {"system": body.get("system") or body.get("instructions"), "developer": developer,
                   "tools": body.get("tools") or _walk_openai_tools(body)}
        digest = hashlib.sha256(json.dumps(surface, sort_keys=True).encode()).hexdigest()
        with open(os.path.join(self.record_dir, "first_request_surface.sha256"), "w") as f:
            f.write(digest + "\n")

    def upstream_connection(self):
        cls = http.client.HTTPSConnection if self.upstream_scheme == "https" else http.client.HTTPConnection
        return cls(self.upstream_host, self.upstream_port, timeout=self.upstream_timeout)


class _Handler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    proxy = None

    def log_message(self, fmt, *args):
        pass

    def address_string(self):
        return "jail"

    # Responses to the agent ---------------------------------------------------------------------------------
    def _refuse(self, reason, detail="", status=400):
        p = self.proxy
        msg = f"cv1 proxy refused this request ({reason}). {detail}".strip()
        if p.provider == "anthropic":
            obj = {"type": "error", "error": {"type": "invalid_request_error", "message": msg}}
        else:
            obj = {"error": {"type": "invalid_request_error", "code": "cv1_" + reason, "message": msg}}
        # Record and notify before answering, so the runner has stopped the run by the time the CLI sees the error.
        p._log({"ts": time.time(), "run": p.run_id, "path": self.path, "refused": reason, "detail": detail})
        p.budget.note(p.run_id, {"kind": "refuse", "reason": reason, "detail": detail})
        p.on_event("refused", {"reason": reason, "detail": detail})
        data = json.dumps(obj).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.send_header("Connection", "close")
        self.close_connection = True
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        self._refuse("route", f"GET {self.path} is not a model call")

    def do_PUT(self):
        self._refuse("route", f"PUT {self.path}")

    def do_DELETE(self):
        self._refuse("route", f"DELETE {self.path}")

    def do_POST(self):
        p = self.proxy
        path = self.path.split("?", 1)[0]
        mode = ROUTES[p.provider].get(path)
        if mode is None:
            self._refuse("route", f"POST {path} is not an allowed route")
            return
        if p.blocked:
            self._refuse("run_blocked", p.blocked)
            return
        if p.complete_marker and os.path.exists(p.complete_marker):
            # The agent ran task-complete; nothing after it counts (protocol §5), so nothing after it is paid for.
            self._refuse("run_complete", "task-complete was already run")
            return
        if self.headers.get("Transfer-Encoding") or not self.headers.get("Content-Length"):
            self._refuse("body", "a Content-Length body is required")
            return
        if self.headers.get("Content-Encoding", "identity") not in ("identity", ""):
            self._refuse("body", "compressed request bodies cannot be inspected")
            return
        raw = self.rfile.read(int(self.headers["Content-Length"]))
        try:
            body = json.loads(raw)
        except ValueError:
            self._refuse("body", "request body is not JSON")
            return
        try:
            check_request_policy(p.provider, p.pinned_model, body)
        except PolicyRefused as e:
            self._refuse(e.reason, e.detail)
            return
        if mode == "free":
            self._forward(raw, body, token=None, worst=0.0)
            return
        p.save_first_request(raw, body)
        max_out = body.get("max_tokens") or body.get("max_output_tokens")
        opaque = opaque_bytes(body)
        worst = pricing.worst_call_cost(p.prices, p.pinned_model, len(raw), max_out, opaque)
        try:
            token = p.budget.reserve(p.run_id, worst, settle_wait_s=SETTLE_WAIT_S)
        except BudgetRefused as e:
            self._refuse(e.reason, e.detail)
            return
        self._forward(raw, body, token, worst, opaque)

    # Forwarding --------------------------------------------------------------------------------------------
    def _forward(self, raw, body, token, worst, opaque=0):
        p = self.proxy
        headers = {}
        for k, v in self.headers.items():
            lk = k.lower()
            if lk in HOP_BY_HOP or lk in CREDENTIAL_HEADERS:
                continue
            headers[k] = v
        headers["Accept-Encoding"] = "identity"
        headers["Content-Length"] = str(len(raw))
        if p.provider == "anthropic":
            headers["x-api-key"] = p.api_key
        else:
            headers["Authorization"] = f"Bearer {p.api_key}"
        observer = observer_for(p.provider)
        seq, archive = p.open_archive(raw)
        started = time.time()
        ttfb = None
        status = None
        resp_headers = {}
        sent_head = False
        conn = None
        try:
            conn = p.upstream_connection()
            conn.request("POST", p.upstream_base + self.path, body=raw, headers=headers)
            resp = conn.getresponse()
            ttfb = time.time() - started
            status = resp.status
            resp_headers = {k.lower(): v for k, v in resp.getheaders()}
            self.send_response(resp.status)
            for k, v in resp.getheaders():
                if k.lower() not in HOP_BY_HOP:
                    self.send_header(k, v)
            self.send_header("Connection", "close")
            self.close_connection = True
            self.end_headers()
            sent_head = True
            streaming = "text/event-stream" in resp_headers.get("content-type", "")
            parser = SSEParser() if streaming else None
            whole = []
            while True:
                chunk = resp.read1(65536) if hasattr(resp, "read1") else resp.read(65536)
                if not chunk:
                    break
                archive.write(chunk)
                self.wfile.write(chunk)
                self.wfile.flush()
                if parser:
                    for _, data in parser.feed(chunk):
                        observer.event(data)
                else:
                    whole.append(chunk)
            if not streaming:
                try:
                    observer.whole(json.loads(b"".join(whole) or b"{}"))
                except ValueError:
                    observer.obs.error = "unparseable response"
        except Exception as e:  # any failure still settles below, at the worst case if usage is incomplete
            observer.obs.error = f"{type(e).__name__}: {e}"
            observer.obs.complete = False
            if not sent_head:
                try:
                    self.send_error(502, "upstream unavailable")
                except OSError:
                    pass
        finally:
            if conn is not None:
                conn.close()
            archive.close()
            self._settle(body, token, worst, opaque, observer.obs, status, started, ttfb, resp_headers, seq)

    def _settle(self, body, token, worst, opaque, obs, status, started, ttfb, resp_headers, seq=None):
        p = self.proxy
        usage = obs.usage
        if token is None:
            cost = 0.0
            basis = "free route"
        elif usage and status == 200 and obs.complete:
            try:
                cost = pricing.call_cost(p.prices, p.pinned_model, usage)
                basis = "usage"
            except Exception as e:  # a malformed usage block must still settle the reservation
                cost = worst
                basis = f"unpriceable usage ({type(e).__name__}); charged the reserved worst case"
        elif status is not None and status >= 400 and not usage:
            cost = 0.0
            basis = f"HTTP {status}, not billed"
        else:
            # A cut stream or a dropped client leaves only partial usage; the provider may bill more than it
            # reported, so the call is charged at its reserved worst case.
            cost = worst
            basis = "usage missing or incomplete; charged the reserved worst case"
        record = {
            "seq": seq, "ts_start": started, "duration_s": round(time.time() - started, 3),
            "ttfb_s": round(ttfb, 3) if ttfb is not None else None, "status": status, "path": self.path,
            "requested_model": body.get("model"), "served_model": obs.served_model, "usage": usage,
            "cost_basis": basis, "tool_names": obs.tool_names, "complete": obs.complete, "error": obs.error,
            "upstream_request_id": resp_headers.get("request-id") or resp_headers.get("x-request-id"),
            "opaque_bytes": opaque,
        }
        p.calls += 1
        if token is not None:
            exceeded = p.budget.settle(token, cost, obs.tool_calls, record)
            if exceeded:
                p.on_event("bound_exceeded", record)
            if basis == "usage":  # only a usage block that priced cleanly is checked against the bound
                reported = pricing.input_tokens_reported(p.entry, usage)
                bound = pricing.input_token_bound(p.entry, int(self.headers.get("Content-Length") or 0), opaque)
                if reported > bound:
                    p.budget.stop(f"input tokens {reported} exceeded the byte bound {bound}")
                    p.on_event("bound_exceeded", record)
            if obs.served_model and status == 200 and obs.served_model not in p.allowed_served:
                p.block(f"served model {obs.served_model!r} is not in the allowed set")
                p.budget.note(p.run_id, {"kind": "served_model_mismatch", "served": obs.served_model})
                p.on_event("served_model_mismatch", record)
        p._log(dict(record, run=p.run_id, cost_usd=round(cost, 6), worst_usd=round(worst, 6)))
        p.on_event("call", {"cost_usd": cost, "status": status, "tool_calls": obs.tool_calls})


def read_api_key(path):
    """Read a provider key from a file readable only by the owner, or fail."""
    st = os.stat(path)
    if st.st_mode & 0o077:
        raise PermissionError(f"{path} must not be readable by group or others (chmod 600)")
    with open(path) as f:
        key = f.read().strip()
    if not key:
        raise ValueError(f"{path} is empty")
    return key


def wait_for_socket(path, timeout=5.0):
    deadline = time.time() + timeout
    while time.time() < deadline:
        s = socket.socket(socket.AF_UNIX, socket.SOCK_STREAM)
        try:
            s.connect(path)
            return True
        except OSError:
            time.sleep(0.05)
        finally:
            s.close()
    return False
