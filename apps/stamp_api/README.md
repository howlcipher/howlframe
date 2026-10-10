# Stamp API

HTTP clock: `/health`, `/now` (`time_now`), `/echo/{name}`.

```bash
go run ../../howlframe.go -compile-bc stamp_api.howl
go run ../../howlframe.go -run-bc -allow-caps network stamp_api.howl.bc.bin
```
