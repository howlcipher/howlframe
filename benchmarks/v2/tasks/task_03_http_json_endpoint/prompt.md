---
support: supported
reason: "http_server, route, res, and res_json are supported in the standalone VM."
---

# Task 03: HTTP JSON Endpoint

Write an HTTP web server that listens on port 8080.
It should have two routes:
1. `GET /`: returns the plain text `Hello World`.
2. `GET /json`: returns a JSON response `{"message": "Hello JSON"}` with the `Content-Type: application/json` header.
