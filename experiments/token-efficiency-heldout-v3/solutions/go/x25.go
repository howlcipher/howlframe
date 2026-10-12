//go:build ignore

package main

import (
	"fmt"
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

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var out []string
	for _, n := range strings.Split(read("in/nums.txt"), "\n") {
		d := n
		for len(d) > 1 {
			s := 0
			for _, c := range d {
				s += int(c - '0')
			}
			d = fmt.Sprint(s)
		}
		out = append(out, n+" -> "+d)
	}
	writeLines(out)
}
