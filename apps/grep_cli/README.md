# Grep CLI

Regex line matcher: print matching lines and a `hits=` summary.

Dogfoods `read_file`, `lines`, `regex_match`, and filesystem capability denial.

```bash
go run ../../howlframe.go -compile-bc grep_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem grep_cli.howl.bc.bin foo sample.txt
```
