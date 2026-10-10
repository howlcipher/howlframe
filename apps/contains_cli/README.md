# contains_cli

Substring search in a file (`str_contains` density sugar). Prints `FOUND` or `MISSING`.

```bash
go run ../../howlframe.go -compile-bc contains_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem contains_cli.howl.bc.bin needle sample.txt
```
