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

func writeLines(lines []string) {
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var out []string
	cur := ""
	for _, line := range strings.Split(read("in/script.txt"), "\n") {
		if s, ok := strings.CutSuffix(line, "\\"); ok {
			cur += s
		} else {
			out = append(out, cur+line)
			cur = ""
		}
	}
	writeLines(out)
}
