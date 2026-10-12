//go:build ignore

package main

import (
	"os"
	"strings"
)

func read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func main() {
	words := strings.Split(read("in/words.txt"), "\n")
	p := words[0]
	for _, w := range words {
		for !strings.HasPrefix(w, p) {
			p = p[:len(p)-1]
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(p+"\n"), 0o644); err != nil {
		panic(err)
	}
}
