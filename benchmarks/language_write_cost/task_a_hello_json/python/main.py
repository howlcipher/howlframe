import json
from http.server import BaseHTTPRequestHandler, HTTPServer


class Handler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/json":
            body = json.dumps({
                "status": "success",
                "message": "Hello from HowlFrame JSON endpoint!",
            }).encode()
            self.send_response(200)
            self.send_header("Content-Type", "application/json")
            self.end_headers()
            self.wfile.write(body)
        else:
            body = b"Hello, World! HowlFrame language is alive!"
            self.send_response(200)
            self.send_header("Content-Type", "text/plain")
            self.end_headers()
            self.wfile.write(body)


if __name__ == "__main__":
    HTTPServer(("", 8080), Handler).serve_forever()
