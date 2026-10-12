//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/path.txt")
	if err != nil {
		panic(err)
	}
	path := strings.TrimSpace(string(data))
	if err := os.WriteFile("out/result.txt", []byte(path[strings.LastIndex(path, ".")+1:]+"\n"), 0o644); err != nil {
		panic(err)
	}
}
