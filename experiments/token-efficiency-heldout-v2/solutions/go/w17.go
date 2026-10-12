//go:build ignore

package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/raw.txt")
	if err != nil {
		panic(err)
	}
	raw := strings.TrimSpace(string(data))
	var b strings.Builder
	for i := 0; i < len(raw); {
		j := i
		for j < len(raw) && raw[j] == raw[i] {
			j++
		}
		fmt.Fprintf(&b, "%c%d", raw[i], j-i)
		i = j
	}
	if err := os.WriteFile("out/result.txt", []byte(b.String()+"\n"), 0o644); err != nil {
		panic(err)
	}
}
