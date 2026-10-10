# Notes CLI

Persistent notes CRUD over a file-backed store (`NOTES_STORE`, default `file://notes.store`).

Dogfoods runtime `store_open` URI expressions, `sget`/`sput`, `switch`/`inc` sugar, and cross-process persistence.

```bash
go run ../../howlframe.go -compile-bc notes_cli.howl
NOTES_STORE=file:///tmp/notes.store \
  go run ../../howlframe.go -run-bc -allow-caps database,filesystem,environment \
  notes_cli.howl.bc.bin add "hello" list get 1
```
