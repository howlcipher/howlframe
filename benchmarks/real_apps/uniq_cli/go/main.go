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
		fmt.Println("Usage: uniq_cli <file>")
		return 2
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	seen := map[string]bool{}
	var order []string
	for _, row := range strings.Split(string(raw), "\n") {
		if row == "" || seen[row] {
			continue
		}
		seen[row] = true
		order = append(order, row)
	}
	for _, row := range order {
		fmt.Println(row)
	}
	return 0
}
