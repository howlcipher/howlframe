package wasm

import (
	"fmt"
	"strings"
)

// ValidateWAT performs the local structural validation available without a
// WAT toolchain. It deliberately does not claim to type-check instructions;
// that remains the job of wat2wasm/wasm-tools when installed.
func ValidateWAT(source string) error {
	trimmed := strings.TrimSpace(source)
	if !strings.HasPrefix(trimmed, "(module") {
		return fmt.Errorf("WAT must start with a module")
	}
	if !strings.Contains(trimmed, "(func") {
		return fmt.Errorf("WAT module must contain a function")
	}

	depth := 0
	inString := false
	escaped := false
	for index := 0; index < len(source); index++ {
		ch := source[index]
		if inString {
			if escaped {
				escaped = false
				continue
			}
			if ch == '\\' {
				escaped = true
			} else if ch == '"' {
				inString = false
			}
			continue
		}
		if ch == '"' {
			inString = true
			continue
		}
		if ch == ';' && index+1 < len(source) && source[index+1] == ';' {
			for index < len(source) && source[index] != '\n' {
				index++
			}
			continue
		}
		switch ch {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return fmt.Errorf("unexpected closing parenthesis at byte %d", index)
			}
		}
	}
	if inString {
		return fmt.Errorf("unterminated string")
	}
	if depth != 0 {
		return fmt.Errorf("unbalanced parentheses: depth %d", depth)
	}
	return nil
}
