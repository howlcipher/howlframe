//go:build race

package hfir

// raceEnabled is true when tests run under the race detector, which slows
// execution several times over.
const raceEnabled = true
