//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/msg.txt")
	if err != nil {
		panic(err)
	}
	out := strings.Map(func(c rune) rune {
		if c >= 'a' && c <= 'z' {
			return (c-'a'+3)%26 + 'a'
		}
		return c
	}, strings.TrimSpace(string(data)))
	if err := os.WriteFile("out/result.txt", []byte(out+"\n"), 0o644); err != nil {
		panic(err)
	}
}
