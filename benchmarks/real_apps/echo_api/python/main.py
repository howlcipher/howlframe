#!/usr/bin/env python3
from http.server import BaseHTTPRequestHandler, HTTPServer
import json
from urllib.parse import parse_qs, urlparse


class H(BaseHTTPRequestHandler):
    def log_message(self, fmt, *args):
        return

    def _json(self, code, obj):
        body = json.dumps(obj, separators=(",", ":")).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self):
        u = urlparse(self.path)
        if u.path == "/health":
            self._json(200, {"status": "ok"})
            return
        if u.path == "/echo":
            q = parse_qs(u.query).get("q", [""])[0]
            self._json(200, {"method": "GET", "q": q})
            return
        if u.path.startswith("/echo/"):
            name = u.path[len("/echo/") :]
            self._json(200, {"name": name, "method": "GET"})
            return
        self._json(404, {"error": "not_found"})


if __name__ == "__main__":
    HTTPServer(("127.0.0.1", 8106), H).serve_forever()
