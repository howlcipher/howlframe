# Fetch CLI

CLI HTTP client: print upstream `status=` and body via `http_request`.

```bash
go run ../../howlframe.go -compile-bc fetch_cli.howl
go run ../../howlframe.go -run-bc -allow-caps network:fetch=example.com fetch_cli.howl.bc.bin http://example.com
```

Outbound requests need a host-scoped `network:fetch=<host>[:<port>]` grant; the coarse `network` grant does not fetch (#165).
