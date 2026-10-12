//go:build ignore

package main

import (
	"os"
	"slices"
	"strings"
)

func read(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	key := func(w string) string {
		r := []rune(w)
		slices.Sort(r)
		return string(r)
	}
	var out []string
	for _, line := range strings.Split(read("in/pairs.txt"), "\n") {
		a, b, _ := strings.Cut(line, " ")
		if key(a) == key(b) {
			out = append(out, "yes")
		} else {
			out = append(out, "no")
		}
	}
	writeLines(out)
}
