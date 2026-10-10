#!/usr/bin/env python3
import json
import os
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
        elif self.path == "/ready":
            if os.environ.get("REQUIRED_ENV", "") != "":
                self._json(200, {"status": "ready"})
            else:
                self._json(503, {"status": "not ready", "reason": "missing REQUIRED_ENV"})
        elif self.path == "/version":
            self._json(200, {"version": "1.0.0", "app": "status_api"})
        elif self.path == "/config":
            self._json(200, {"public_config": os.environ.get("PUBLIC_CONFIG", "")})
        else:
            self.send_error(404)


if __name__ == "__main__":
    HTTPServer(("", 8090), Handler).serve_forever()
