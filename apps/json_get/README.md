# JSON Get

Print a top-level JSON object value by key (`NOT FOUND` when missing or empty).

Dogfoods `parse_json` from file bytes and dense CLI error handling.

```bash
go run ../../howlframe.go -compile-bc json_get.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem json_get.howl.bc.bin sample.json name
```
