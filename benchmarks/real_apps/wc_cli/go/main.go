package main

import (
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: wc_cli <file>")
		return 2
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	rows := strings.Split(string(raw), "\n")
	lc := len(rows)
	wc := 0
	for _, row := range rows {
		for _, w := range strings.Split(row, " ") {
			if w != "" {
				wc++
			}
		}
	}
	fmt.Printf("lines=%d\n", lc)
	fmt.Printf("words=%d\n", wc)
	fmt.Printf("bytes=%d\n", len(raw))
	return 0
}
