# Counter API

In-memory HTTP counter: `GET /count`, `POST /inc`, `POST /reset`.

Dogfoods `http_server` startup `store_open`/`sput`, plus `nz`/`switch` density in handlers.

```bash
go run ../../howlframe.go -compile-bc counter_api.howl
go run ../../howlframe.go -run-bc -allow-caps network,database counter_api.howl.bc.bin
```
