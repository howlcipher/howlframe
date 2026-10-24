"""Helpers for language-neutral black-box tests: run `./run`, or start it as a server and talk HTTP to it."""

import json
import os
import socket
import subprocess
import time
import urllib.error
import urllib.request


def app():
    return os.environ["CV1_APP"]


def workspace():
    return os.environ.get("CV1_WORKSPACE") or os.path.dirname(app())


def run(args=(), stdin=None, timeout=30, cwd=None, env=None):
    """Run ./run with arguments; returns CompletedProcess with text stdout and stderr."""
    return subprocess.run([app(), *args], input=stdin, capture_output=True, text=True, timeout=timeout,
                          cwd=cwd or workspace(), env=env)


def free_port():
    with socket.socket() as s:
        s.bind(("127.0.0.1", 0))
        return s.getsockname()[1]


class Server:
    """Start ./run as a server (the port is passed in PORT and as `--port N`) and wait until it accepts."""

    def __init__(self, args=(), port=None, startup_timeout=20, cwd=None, extra_env=None):
        self.port = port or free_port()
        env = dict(os.environ, PORT=str(self.port), **(extra_env or {}))
        self.proc = subprocess.Popen([app(), *args, "--port", str(self.port)], cwd=cwd or workspace(), env=env,
                                     stdout=subprocess.PIPE, stderr=subprocess.PIPE, text=True)
        deadline = time.time() + startup_timeout
        while time.time() < deadline:
            if self.proc.poll() is not None:
                raise RuntimeError(f"server exited with {self.proc.returncode}: {self.proc.stderr.read()[-2000:]}")
            try:
                with socket.create_connection(("127.0.0.1", self.port), timeout=0.2):
                    return
            except OSError:
                time.sleep(0.05)
        self.stop()
        raise RuntimeError("server did not accept connections in time")

    def request(self, method, path, body=None, headers=None, timeout=10):
        data = json.dumps(body).encode() if body is not None and not isinstance(body, bytes) else body
        req = urllib.request.Request(f"http://127.0.0.1:{self.port}{path}", data=data, method=method,
                                     headers=dict({"Content-Type": "application/json"} if data else {},
                                                  **(headers or {})))
        try:
            with urllib.request.urlopen(req, timeout=timeout) as r:
                return r.status, dict(r.headers), r.read()
        except urllib.error.HTTPError as e:
            return e.code, dict(e.headers), e.read()

    def stop(self):
        if self.proc.poll() is None:
            self.proc.terminate()
            try:
                self.proc.wait(5)
            except subprocess.TimeoutExpired:
                self.proc.kill()
                self.proc.wait()

    def __enter__(self):
        return self

    def __exit__(self, *exc):
        self.stop()
