# has_line_cli

Exact line membership in a file (`list_contains` density sugar). Distinct from substring `contains_cli`.

```bash
go run ../../howlframe.go -compile-bc has_line_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem has_line_cli.howl.bc.bin beta sample.txt
```
