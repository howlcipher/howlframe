//go:build ignore

package main

import (
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/board.txt")
	if err != nil {
		panic(err)
	}
	b := strings.ReplaceAll(string(data), "\n", "")
	result := "draw"
	if strings.Contains(b, ".") {
		result = "open"
	}
	for _, l := range [][3]int{{0, 1, 2}, {3, 4, 5}, {6, 7, 8}, {0, 3, 6}, {1, 4, 7}, {2, 5, 8}, {0, 4, 8}, {2, 4, 6}} {
		if b[l[0]] != '.' && b[l[0]] == b[l[1]] && b[l[1]] == b[l[2]] {
			result = string(b[l[0]])
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(result+"\n"), 0o644); err != nil {
		panic(err)
	}
}
