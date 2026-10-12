//go:build ignore

package main

import (
	"os"
	"strconv"
	"strings"
	"time"
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
	for _, line := range strings.Split(read("in/days.txt"), "\n") {
		d, err := strconv.Atoi(line)
		if err != nil {
			panic(err)
		}
		out = append(out, time.Date(2021, 1, d, 0, 0, 0, 0, time.UTC).Format("01-02"))
	}
	writeLines(out)
}
