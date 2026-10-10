# icontains_cli

Case-insensitive substring search using `to_lower` + `str_contains`.

```bash
go run ../../howlframe.go -compile-bc icontains_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem icontains_cli.howl.bc.bin beta sample.txt
```
