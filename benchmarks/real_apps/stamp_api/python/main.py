#!/usr/bin/env python3
import json
import time
from http.server import BaseHTTPRequestHandler, HTTPServer


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
        if self.path == "/health":
            self._json(200, {"status": "ok"})
        elif self.path == "/now":
            self._json(200, {"unix": str(int(time.time()))})
        elif self.path.startswith("/echo/"):
            name = self.path[len("/echo/") :]
            self._json(200, {"name": name, "unix": str(int(time.time()))})
        else:
            self.send_error(404)

    def do_POST(self):
        self._json(405, {"error": "method_not_allowed"})


if __name__ == "__main__":
    HTTPServer(("", 8104), Handler).serve_forever()
