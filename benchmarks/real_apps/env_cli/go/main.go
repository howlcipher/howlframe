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
		fmt.Println("Usage: env_cli <name>")
		return 2
	}
	v := os.Getenv(args[0])
	if v == "" {
		fmt.Println("MISSING")
		return 1
	}
	fmt.Println(v)
	return 0
}
