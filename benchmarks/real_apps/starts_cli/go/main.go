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
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: starts_cli <prefix> <file>")
		return 2
	}
	raw, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	if strings.HasPrefix(string(raw), args[0]) {
		fmt.Println("YES")
		return 0
	}
	fmt.Println("NO")
	return 1
}
