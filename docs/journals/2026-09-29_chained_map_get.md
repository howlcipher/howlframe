# Chained map_get

## Why this slice

A path read had to bind every intermediate dictionary. Interpreter
`evalMapGet`, the Go checker, and `MAP_GET` all required the dict operand to
be a symbol. `(map_get (map_get row "user") "name")` was illegal, so
applications piled up `let`s for a pure read.

The smallest fix is to evaluate that operand. A symbol still names a
variable. Any other expression, including another `map_get`, is the dict.
Nothing new is granted.

## Contract

`(map_get dict key)` reads one key. `dict` may be a symbol or an expression.
A missing key is `""` on the interpreter, the bytecode VM, Go, and
JavaScript. That is the #103 sentinel. A present value is returned as stored,
so a present `""` stays `""`.

`map_get` of that `""`, or of any other non-dict, fails closed with
`TYPE_ERROR: map_get expected dict`. It does not return another absence
value. A missing leaf is `""`. A missing intermediate prints as `""` and the
next `map_get` is `TYPE_ERROR`.

`OpMapGet` keeps an empty capability field, the same as `OpMapKeys`.
`ForConstruct("map_get")` stays empty. A path read runs under an empty grant.

Named bytecode still pops only the key and loads the variable. An expression
dict is compiled onto the stack and popped as well. The opcode does not grow
an operand or a capability. `map_set` and `map_delete` still require a symbol,
because they mutate.

Go keeps `howlFrameMapGet` for a named dict, so a `map[string]string` read
stays a string. A nested read uses `howlFrameMapGetValue`, which accepts
`map[string]string` and `map[string]any` and panics with `TYPE_ERROR`
otherwise. A dict literal whose values are strings stays `map[string]string`.
A dict that contains a nested dict is `map[string]any`, and a nested dict of
strings stays `map[string]string`.

JavaScript keeps `(dict[key] ?? "")` for a named dict. An expression dict
rejects null, arrays, and non-objects before that coercion, so the `""`
sentinel is not indexed into another miss.

## What this does not do

No new capability. No JSONPath dialect. No mutation along the path. No
bytecode modules (#106). No `html_escape` (#105). No `TYPE_ERROR` work for
`map_set`, `append`, or a named JavaScript `map_get` (#108). No `map_keys`
sort parity (#109). No Factory, HowlPlane, HowlBoard, or authority work. The
experimental HFIR bytecode lowerer still names a variable; production
bytecode is `bytecode.CompileToBytecode`.

## Evidence

`internal/vm/map_get_path_test.go` runs the same programs on the interpreter
and the bytecode VM with an empty grant. The hit program prints a two-level
leaf, a three-level leaf, and a missing leaf as `""`, and the bytecode
contains both a named `MAP_GET` and a value `MAP_GET`. The missing-intermediate
program prints the #103 sentinel and then fails with `TYPE_ERROR` and
`got string`. A string intermediate fails the same way and prints nothing.
Capability metadata for `OpMapGet` matches `OpMapKeys`, and neither form is
`CAPABILITY_DENIED` under an empty grant.

`internal/backend/gogen/map_get_path_test.go` runs the generated Go for the
hit program and checks the crash record for the two `TYPE_ERROR` programs.
`internal/backend/javascript/map_get_path_test.go` runs the generated script
under Node the same way.
