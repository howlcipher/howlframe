//go:build ignore

package main

import (
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/n.txt")
	if err != nil {
		panic(err)
	}
	n, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		panic(err)
	}
	var out []string
	for p := 2; p <= n; p++ {
		prime := true
		for d := 2; d*d <= p; d++ {
			if p%d == 0 {
				prime = false
				break
			}
		}
		if prime {
			out = append(out, strconv.Itoa(p))
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, " ")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
