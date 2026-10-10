package main

import (
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 1 || args[0] == "" {
		fmt.Println("Usage: mkdir_cli <path>")
		return 2
	}
	if err := os.MkdirAll(args[0], 0755); err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	fmt.Println("OK")
	return 0
}
