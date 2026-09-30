package hfir

// LoweredABIV1 is the version of the lowered-HFIR backend contract.
// The prose contract is docs/reference/lowered_hfir_abi_v1.md.
const LoweredABIV1 = "lowered-hfir-abi/v1"

// WasmInfeasibleKinds is the complete v1 host-effect rejection set for
// target "wasm". isFeasible rejects these kinds and no others. Growing
// Wasm collections or new host imports is outside this revision.
var WasmInfeasibleKinds = []string{
	"exec",
	"spawn_agent",
	"http_server_start",
}
