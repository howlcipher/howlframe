# KV CLI

A small stateful CLI around HowlFrame's native store system.
Demonstrates deterministic in-memory state manipulation, sharing state across multiple operations within the same process.

## Usage

```bash
go run ../../howlframe.go -compile-bc kv_cli.howl

# Pass a sequence of operations as arguments:
go run ../../howlframe.go -run-bc -allow-caps database kv_cli.howl.bc.bin set theme dark get theme delete theme increment visits
```

## Operations
* `set <key> <value>`: Store value.
* `get <key>`: Read value (prints NOT FOUND if absent).
* `delete <key>`: Remove value.
* `increment <key>`: Read an integer-like counter, initialize to 1 if absent, increment it, and return new value.
