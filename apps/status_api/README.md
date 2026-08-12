# Status API

A straightforward HTTP service built in HowlFrame to verify network capabilities, environment inspection, and deterministic HTTP request handling.

## Usage

```bash
go run ../../howlframe.go -compile-bc status_api.howl
REQUIRED_ENV=1 PUBLIC_CONFIG=test go run ../../howlframe.go -run-bc -allow-caps network,environment status_api.howl.bc.bin
```

## Endpoints

- `GET /health`
- `GET /ready` (Requires `REQUIRED_ENV` environment variable)
- `GET /version`
- `GET /config` (Echoes `PUBLIC_CONFIG` environment variable)
