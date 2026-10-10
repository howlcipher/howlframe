# Auth API

Minimal HTTP API with bearer-token gate and path/query echo.

Dogfoods `switch` on headers/methods inside `lambda` route handlers (header auth without middleware).

```bash
go run ../../howlframe.go -compile-bc auth_api.howl
go run ../../howlframe.go -run-bc -allow-caps network auth_api.howl.bc.bin
# GET /health, GET /secret (Authorization: Bearer secret), GET /echo/{name}?q=
```
