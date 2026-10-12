//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/text.txt")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.ReplaceAll(string(data), "foo", "bar")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
