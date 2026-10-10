#!/usr/bin/env python3
import json
from http.server import BaseHTTPRequestHandler, HTTPServer
from urllib.parse import parse_qs, urlparse


class Handler(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def _json(self, code, body):
        data = json.dumps(body).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(data)

    def do_GET(self):
        parsed = urlparse(self.path)
        path = parsed.path
        auth = self.headers.get("Authorization", "")
        if path == "/health":
            self._json(200, {"status": "ok"})
        elif path == "/secret":
            if auth == "Bearer secret":
                self._json(200, {"ok": "1", "scope": "secret"})
            else:
                self._json(401, {"error": "unauthorized"})
        elif path.startswith("/echo/"):
            name = path[len("/echo/") :]
            q = parse_qs(parsed.query).get("q", [""])[0]
            if auth != "Bearer secret":
                self._json(401, {"error": "unauthorized"})
            else:
                self._json(200, {"name": name, "q": q})
        else:
            self.send_error(404)

    def do_POST(self):
        self._json(405, {"error": "method_not_allowed"})


if __name__ == "__main__":
    HTTPServer(("", 8102), Handler).serve_forever()
