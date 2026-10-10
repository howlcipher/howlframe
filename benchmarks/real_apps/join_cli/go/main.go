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
	if len(args) < 2 || args[1] == "" {
		fmt.Println("Usage: join_cli <sep> <file>")
		return 2
	}
	raw, err := os.ReadFile(args[1])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	fmt.Println(strings.Join(strings.Split(string(raw), "\n"), args[0]))
	return 0
}
