# Absence idiom

## Why this slice

A missing dict field and a missing store record are different events. Callers
tell them apart only when every backend uses the same sentinel for each one.

The bytecode VM (`OpMapGet`) and the interpreter (`evalMapGet`) already return
`""` when a `map_get` key is absent. `OpStoreGet` returns nil when a record is
absent, so a missing record is not an empty dict. The JavaScript backend
already emits `(dict[key] ?? "")`. The Go backend emitted raw indexing. A
`map[string]string` miss is `""`. A `map[string]any` miss is nil, which is the
same sentinel as a missing store record.

## Contract

`(map_get dict key)` on a missing key is `""`.

A present value is returned as stored. A present `""` stays `""`. A present
nil stays nil. `is_nil` of that present empty string is false.

`(store_get handle key)` on a missing record is nil. The result is not `""`
and it is not an empty dict. `is_nil` of that result is true, which is how a
caller separates a blank field from a missing record.

`map_get` declares no capability. `store_get` stays `database`. `store_keys`
stays `database`, and a `file://` store still also requires `filesystem`.

The interpreter and the bytecode VM already followed this contract. Dicts
there are `map[string]any`. A miss uses comma-ok and yields `""`. A hit
returns the stored value, including a non-string value.

The Go backend now emits `howlFrameMapGet`. A missing key is `""` for both
`map[string]string` and `map[string]any`. A hit keeps the map's value type, so
a string-map read stays a string. A present nil stays nil.

The JavaScript backend keeps `(dict[key] ?? "")`. A missing key is `""`. A
present `""` stays `""`. A present JSON null is also `""` under `??`. The VM
and the Go helper return nil for a present null. This slice leaves that
JavaScript expression as it is.

## What this does not do

No new capability. No change to which opcode requires which grant. No change
to `store_keys`. No chained `map_get` (#104). No `html_escape` (#105). No
`TYPE_ERROR` completeness for the remaining dict and list ops (#108). No sort
parity (#109). No Factory, HowlPlane, HowlBoard, or authority work.

## Evidence

`internal/vm/absence_test.go` runs the same `map_get` program on the
interpreter and the bytecode VM with an empty grant, covering a string-valued
dict and a `map[string]any` dict (a number and a nested dict). A miss prints
as `""`, and `is_nil` of that miss is false. The store program, under
`database` only, shows `is_nil` false for a present record whose field is
`""` and true for a missing record, and the missing record prints as `<nil>`.
Opcode checks require the same values, require `map_get` to stay capability-free,
and require `store_get` and `store_keys` to stay `database`.

`internal/backend/gogen/absence_test.go` runs the helper copied out of
generated Go against `map[string]string` and `map[string]any`, then runs the
generated string-map program.

`internal/backend/javascript/absence_test.go` runs the generated script under
Node for a string dict and a `JSON.parse` object.
