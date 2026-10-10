package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: cut_cli <index> <file>")
		return 2
	}
	idx, err := strconv.Atoi(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	raw, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	for _, row := range strings.Split(string(raw), "\n") {
		if row == "" {
			continue
		}
		cols := strings.Split(row, ",")
		if len(cols) > idx {
			fmt.Println(cols[idx])
		}
	}
	return 0
}
