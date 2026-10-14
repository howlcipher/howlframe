# Consumer binding target validation

Frame baseline: 6c9696209e68aa38e9c3d7db768fca39eb573392.
Board consumer: 3b1e007f1b5127b0e761222bb6b9778f057ddd75.

The documented Board `make test` failed during frontend compilation because
`(let (rec (map_get x "delta")) ...)` was interpreted as record sugar by
`rejectUnsupportedTargetControlflow`. Eight Go/JS binding and parameter
cases reproduced the rejection before compiler edits.

The correction walks initializer and body expressions rather than treating
binding/parameter containers as calls. Dictionary keys and values remain
visited as expressions. This changes no supported syntax, capability,
authority or target coverage. Actual sugar in first/second initializers,
try_let initializers, dictionary values and function/lambda bodies remains
rejected before output. Targeted regression passes.

Board frontend/backend/import/demo compilation then succeeds. Its full backend
workflow still fails under current runtime contracts, including explicit
HTTP handler grant attenuation and legacy missing-value assumptions. These
are separate consumer compatibility defects, not fixed by this compiler PR.
Known-good consumer pins remain necessary until those workflows pass.

Independent review: Claude actual-diff review raised shape/coverage concerns.
The change was narrowed to documented forms, checked against checker/AST
contracts, and negative nested-expression regression added. Final review
and CI status must be checked on the PR; do not infer them from this journal.

A review follow-up executed generated Go using the binding name `range`.
It fails because `range` is a Go keyword; the target validator's success is
not proof that every identifier is accepted by the generated language.
This separate generator limitation is retained in mission evidence and is
not claimed fixed here. The regression's build assertion checks the target
validator boundary only. Board's actual generated JavaScript passes
`node --check` and all four public compilation steps.
