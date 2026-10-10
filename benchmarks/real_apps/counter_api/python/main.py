#!/usr/bin/env python3
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

n = 0


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
        global n
        if self.path == "/count":
            self._json(200, {"n": str(n)})
        else:
            self.send_error(404)

    def do_POST(self):
        global n
        if self.path == "/inc":
            n += 1
            self._json(200, {"n": str(n)})
        elif self.path == "/reset":
            n = 0
            self._json(200, {"n": "0"})
        else:
            self.send_error(404)


if __name__ == "__main__":
    HTTPServer(("", 8103), Handler).serve_forever()
