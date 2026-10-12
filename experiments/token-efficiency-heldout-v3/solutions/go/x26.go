//go:build ignore

package main

import (
	"fmt"
	"os"
	"strconv"
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
	var names []string
	var counts []int
	total := 0
	for _, line := range strings.Split(read("in/votes.txt"), "\n") {
		name, c, _ := strings.Cut(line, " ")
		n, err := strconv.Atoi(c)
		if err != nil {
			panic(err)
		}
		names, counts, total = append(names, name), append(counts, n), total+n
	}
	var out []string
	for i, name := range names {
		out = append(out, fmt.Sprintf("%s %d%%", name, counts[i]*100/total))
	}
	writeLines(out)
}
