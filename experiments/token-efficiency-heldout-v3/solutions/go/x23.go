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
	if err := os.WriteFile("out/result.csv", []byte(strings.Join(lines, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	dept := map[string]string{}
	for _, line := range strings.Split(read("in/dept.csv"), "\n")[1:] {
		id, d, _ := strings.Cut(line, ",")
		dept[id] = d
	}
	out := []string{"id,name,dept"}
	for _, line := range strings.Split(read("in/emp.csv"), "\n")[1:] {
		id, _, _ := strings.Cut(line, ",")
		out = append(out, line+","+dept[id])
	}
	writeLines(out)
}
