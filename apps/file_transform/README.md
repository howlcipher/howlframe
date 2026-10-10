# File Transform

CLI that reads a CSV file, emits JSONL (`c0`/`c1` columns), and writes the result.

Dogfoods `read_file`, `write_file`, `lines`, `str_split`, `encode_json`, and filesystem capability denial.

```bash
go run ../../howlframe.go -compile-bc file_transform.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem file_transform.howl.bc.bin in.csv out.jsonl
```
