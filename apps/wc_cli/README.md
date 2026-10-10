# WC CLI

Line/word/byte counts for a file.

Dogfoods widened `list_len` (list/string/bytes), `lines`, `str`, and filesystem capability denial.

```bash
go run ../../howlframe.go -compile-bc wc_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem wc_cli.howl.bc.bin sample.txt
```
