//go:build ignore

package main

import (
	"os"
	"strings"
)

func check(p string) string {
	switch {
	case len(p) < 8:
		return "too short"
	case !strings.ContainsAny(p, "0123456789"):
		return "no digit"
	case !strings.ContainsAny(p, "ABCDEFGHIJKLMNOPQRSTUVWXYZ"):
		return "no upper"
	}
	return "ok"
}

func main() {
	data, err := os.ReadFile("in/passwords.txt")
	if err != nil {
		panic(err)
	}
	var out []string
	for _, p := range strings.Split(string(data), "\n") {
		out = append(out, check(p))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
