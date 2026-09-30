# Capability surface honesty

## Why this slice

`map_keys` and `store_keys` both enumerate keys. They do not share a grant.
`OpMapKeys` has an empty capability field. `OpStoreKeys` is `database`, and
the VM also demands `filesystem` when the store is file-backed. A later
"make enumeration easier" patch can blur that line. #79 and #94 stay Done.
This row writes the split down and locks it with tests.

## Contract

`map_keys` is pure. Dictionary operations such as `map_get` and `map_keys`
grant nothing. `MAP_KEYS` runs under an empty grant.

`store_keys` stays `database`. A `memory://` store needs that grant alone.
Omitting `database` denies `STORE_KEYS`.

A `file://` store additionally requires `filesystem`. `database` alone denies
`file://` `STORE_KEYS`, including when the handle is already open, so the
denial is the enumeration check and not only `STORE_OPEN`.

The generated opcode table records the opcode field: empty for `MAP_KEYS`,
`database` for `STORE_KEYS`. The URI grant is hand-written in
`docs/reference/bytecode_capability_notes.md` and in the README capability
section. The generator appends a pointer under the table. The table cells
are still generated.

## What this does not do

No new capability. No change to which opcode requires which grant. No
envelope or authority bypass. No Factory. No linker, HFIR module linking, or
VM module opcode (#106 stays the do-not-build-yet spike). No sort-parity
work (#109). #79 and #94 stay Done.

## Evidence

`internal/vm/map_keys_test.go` runs `map_keys` on the interpreter and the
bytecode VM with a nil grant, and `TestMapKeysDeclaresNoCapability` fails if
`OpMapKeys` or `ForConstruct("map_keys")` grows a capability.

`TestBytecodeStoreKeysRequiresDatabaseCapability` fails if `OpStoreKeys` or
`ForConstruct("store_keys")` loses `database`, and if a memory-store
enumeration succeeds with `filesystem` alone.

`TestFileStoreKeysDeniedWithDatabaseAlone` opens a `file://` store with
`database` and `filesystem`, then enumerates with `database` alone. The
result is `CAPABILITY_DENIED` for `filesystem`, and the persisted file is
unchanged. The same file enumerates when both grants are present.

Opcode fields in `internal/bytecode/opcode.go` are unchanged.
