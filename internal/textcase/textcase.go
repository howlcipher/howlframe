// Package textcase is HowlFrame's case mapping for to_lower and to_upper:
// Go's strings.ToLower and ToUpper, but over a case table the package owns
// (Unicode UnicodeVersion, the version of Go 1.25, which releases are built
// with) instead of the toolchain's unicode.CaseRanges. Go 1.27 moved to
// Unicode 17.0.0, which maps 110 more code points, so a program's output
// depended on the toolchain that built the VM (bugs.md #69). The
// WebAssembly backend's std/unicode.howl is generated from the same table.
package textcase

import (
	"strings"
	"unicode"
)

// UnicodeVersion is the Unicode version of CaseRanges.
const UnicodeVersion = "15.0.0"

// ToLower is strings.ToLower over CaseRanges: each rune lowered, invalid
// UTF-8 as U+FFFD.
func ToLower(s string) string { return strings.Map(LowerRune, s) }

// ToUpper is strings.ToUpper over CaseRanges.
func ToUpper(s string) string { return strings.Map(UpperRune, s) }

// LowerRune is unicode.ToLower over CaseRanges.
func LowerRune(r rune) rune {
	if r <= unicode.MaxASCII {
		if 'A' <= r && r <= 'Z' {
			r += 'a' - 'A'
		}
		return r
	}
	return to(unicode.LowerCase, r)
}

// UpperRune is unicode.ToUpper over CaseRanges.
func UpperRune(r rune) rune {
	if r <= unicode.MaxASCII {
		if 'a' <= r && r <= 'z' {
			r -= 'a' - 'A'
		}
		return r
	}
	return to(unicode.UpperCase, r)
}

// to is Go's unicode.to: the mapping of r to the given case by binary
// search of CaseRanges, or r when no range holds it.
func to(c int, r rune) rune {
	lo, hi := 0, len(CaseRanges)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		cr := CaseRanges[m]
		if rune(cr.Lo) <= r && r <= rune(cr.Hi) {
			delta := cr.Delta[c]
			if delta > unicode.MaxRune {
				// An alternating upper-lower range: upper case at even
				// offsets from Lo, lower case at odd ones.
				return rune(cr.Lo) + ((r-rune(cr.Lo))&^1 | rune(c&1))
			}
			return r + delta
		}
		if r < rune(cr.Lo) {
			hi = m
		} else {
			lo = m + 1
		}
	}
	return r
}
