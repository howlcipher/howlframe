# Fail-closed TYPE_ERROR on dict and list ops

## Why this slice

`map_keys` already rejects a non-dict with `TYPE_ERROR`. The other public
dict and list operations did not. A JavaScript `map_get` on a named value
was `obj[key] ?? ""`, so a string or a list produced a character, an element,
or `""` instead of a type error. `map_set` wrote through arrays. `delete`
and `.length` succeeded on the wrong receiver. `append` threw a host
`TypeError` from `.push`.

Go `list_len` was `len`, which counts a string or a map. `list_get` ignored
a bad index (`Atoi` error discarded) and did not compile for a non-slice.
`append`, `map_set`, and `map_delete` on a dynamic `any` value were host
compile errors. Named `map_get` of that `any` value did not compile either.
Bug #47 and bug #53 had already closed other opcodes. This set was still open.

## Contract

A wrong receiver is `TYPE_ERROR` on the interpreter, the bytecode VM, Go,
and JavaScript. The message names the operation and the expected kind
(`dict` or `list`).

| Op | Wrong receiver |
| --- | --- |
| `map_get` | non-dict |
| `map_set` | non-dict |
| `map_delete` | non-dict |
| `map_keys` | non-dict (already closed; unchanged) |
| `append` | non-list |
| `list_get` | non-list, or an index that is not a whole number |
| `list_len` | non-list |

A missing `map_get` key is still `""` (#103). A present `""` stays `""`.
`list_get` of an in-range element returns that element. An out-of-range
index is still `""`.

None of these ops declare a capability. `ForConstruct` stays empty,
matching `map_keys`.

Concrete Go maps stay on `howlFrameMapGet`, `dict[key] =`, and `delete`.
A concrete `[]string` stays on `append`. A dynamic value (a `map_get` out
of a mixed record, whose Go type is `any`) goes through a helper that
panics `TYPE_ERROR` when the runtime value is the wrong kind.
`list_get` and `list_len` always use that helper, including for `[]string`,
so `len` of a string or a map is gone.

JavaScript uses one helper for `map_get`, `map_set`, `map_delete`,
`append`, `list_get`, and `list_len`. Dicts are plain objects. Lists are
arrays. `map_get` still ends in `?? ""`.

The interpreter already rejected a bad `append`, `map_set`, `map_delete`,
and `list_get`. Those messages now say `TYPE_ERROR`, matching the VM.
The VM opcodes were already fail-closed. This slice does not change them
except by locking the contract in tests.

## What this does not do

No new collection opcode. No new capability. No change to the #103 absence
sentinel. No `map_keys` sort change (#109). No Wasm SSA collection work
(#73): that backend's `list_get` and `map_get` stay on the static layout
and are not part of this lockstep. No #106 modules spike. No #107
capability-honesty docs pass. No Factory, HowlPlane, or authority work.

## Evidence

`internal/vm/collection_type_test.go` runs the miss, the in-range read, the
out-of-range `""`, and the mutations on the interpreter and the bytecode
VM, then one non-dict and one non-list case per op that was still soft.
`map_keys` on a string is still `TYPE_ERROR`. An empty grant still runs
the pure program.

`internal/backend/gogen/collection_type_test.go` and
`internal/backend/javascript/collection_type_test.go` run the same
programs. A string `list_len` no longer prints the string's length. A
dynamic `map_get` of a list or a string no longer returns a value.
