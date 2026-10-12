//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/data.csv")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	col := -1
	for i, name := range strings.Split(lines[0], ",") {
		if name == "email" {
			col = i
		}
	}
	var emails []string
	for _, row := range lines[1:] {
		emails = append(emails, strings.Split(row, ",")[col])
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(emails, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
