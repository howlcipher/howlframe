//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/events.txt")
	if err != nil {
		panic(err)
	}
	var out []string
	for _, line := range strings.Split(string(data), "\n") {
		if len(out) == 0 || out[len(out)-1] != line {
			out = append(out, line)
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
