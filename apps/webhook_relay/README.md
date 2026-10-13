# Webhook Relay

Inbound HTTP relay that forwards GET/POST to `UPSTREAM_URL` and returns the upstream status + body.

Dogfoods `http_request` (status+body dict, optional request body), `env`, and `res`.

```bash
export UPSTREAM_URL=http://127.0.0.1:8099/
go run ../../howlframe.go -compile-bc webhook_relay.howl
go run ../../howlframe.go -run-bc -allow-caps network,environment,network:fetch=127.0.0.1:8099 webhook_relay.howl.bc.bin
```

The `/relay` route is granted exactly one upstream, `network:fetch=127.0.0.1:8099` (#165). To relay elsewhere, change that route grant and pass the same grant to `-allow-caps`.
