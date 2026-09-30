# Dict key sort parity

## Why this slice

`map_keys` already returns a sorted key list on the interpreter, the bytecode
VM, the Go backend, and JavaScript. Ordinary keys agree. The comparators do
not.

The VM orders keys with Go's string comparison (`sort.Strings`). The Go
backend emits that same call. JavaScript emitted `Object.keys(_d).sort()`
with no comparator, which compares UTF-16 code units. For well-formed text,
UTF-8 byte order is code-point order. UTF-16 code-unit order is not, once a
key is outside the Basic Multilingual Plane: a supplementary character is a
surrogate pair whose first unit is in U+D800–U+DBFF, so it sorts before a
BMP key such as U+F000. In UTF-8 those bytes start with F0, which sorts
after U+F000 (EF 80 80).

## Contract

`map_keys` order is UTF-8 byte order. That is Go `sort.Strings` and Go
string `<`. It is not locale collation, and it is not JavaScript's default
UTF-16 code-unit sort.

JavaScript encodes each code point to UTF-8 and compares those bytes
(`howlFrameCompareUTF8`). A lone surrogate is encoded as that code point's
three-byte UTF-8 sequence, which is the same byte order Go uses for a
string that contains it. `TextEncoder` is not used: it would replace a lone
surrogate with U+FFFD.

The two fixtures in `tests/fixtures/` are the shared program.

| Fixture | Keys | Byte order and UTF-16 |
| --- | --- | --- |
| `map_keys_sort_bmp.howl` | `alpha`, `beta`, `gamma`, U+00E9 | same list |
| `map_keys_sort_nonbmp.howl` | U+0061, U+00E9, U+F000, U+1F000, U+1F600 | different lists |

Byte order of the non-BMP fixture is `a`, `é`, U+F000, U+1F000, U+1F600.
UTF-16 code-unit order is `a`, `é`, U+1F000, U+1F600, U+F000. The
interpreter, the bytecode VM, Go, and JavaScript all print the byte-order
list. Both runs use an empty capability grant.

An empty dictionary is still an empty list. A non-dictionary is still
`TYPE_ERROR`. `map_keys` still grants nothing.

## What this does not do

No new opcode. No change to the `map_keys` form. No locale-aware collation.
No capability change. No Factory. No linker, HFIR module linking, or VM
module opcode.

`store_keys` was not modified. On the VM it already compares keys with Go
string `<`, which is this same byte order. The Go and JavaScript backends
do not emit `store_keys`, so there was no cross-backend drift to widen
into this row.

Wasm SSA collections stay on #73. That backend does not emit `map_keys`.
If a later Wasm `map_keys` is added under #73, it must use this UTF-8 byte
order, not UTF-16 code-unit order.

## Evidence

`internal/vm/map_keys_order_test.go` sorts the fixture keys with Go string
`<` and with UTF-16 code units. The BMP pair agrees. The non-BMP pair does
not. The interpreter, `RunBytecode`, generated Go, and generated JavaScript
each print the byte-order line for both fixtures. The JavaScript emitter
rejects a bare `.sort()` call. The existing empty-dict and non-dict
`TYPE_ERROR` tests are unchanged.
