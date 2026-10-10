# Template CLI

Replace `{{key}}` in a file with a value (split/join; n-ary `or` + `str` density).

```bash
go run ../../howlframe.go -compile-bc template_cli.howl
go run ../../howlframe.go -run-bc -allow-caps filesystem template_cli.howl.bc.bin tpl.txt name Ada
```
