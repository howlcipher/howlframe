//go:build ignore

package main

import (
	"fmt"
	"os"
	"sort"
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
	var spans [][2]int
	for _, line := range strings.Split(read("in/spans.txt"), "\n") {
		var a, b int
		if _, err := fmt.Sscanf(line, "%d-%d", &a, &b); err != nil {
			panic(err)
		}
		spans = append(spans, [2]int{a, b})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i][0] < spans[j][0] })
	var merged [][2]int
	for _, s := range spans {
		if n := len(merged); n > 0 && s[0] <= merged[n-1][1] {
			merged[n-1][1] = max(merged[n-1][1], s[1])
		} else {
			merged = append(merged, s)
		}
	}
	var out []string
	for _, m := range merged {
		out = append(out, fmt.Sprintf("%d-%d", m[0], m[1]))
	}
	writeLines(out)
}
