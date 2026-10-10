# uniq_cli

Print unique non-empty lines in first-seen order (`list_contains` + `not`).

```bash
go run ../../howlframe.go -compile-bc uniq_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem uniq_cli.howl.bc.bin sample.txt
```
