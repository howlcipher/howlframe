# echo_api

Minimal JSON echo HTTP API (`/health`, `/echo`, `/echo/{name}`).

```bash
go run ../../howlframe.go -compile-bc echo_api.howl
go run ../../howlframe.go -run-bc -allow-caps network echo_api.howl.bc.bin
```
