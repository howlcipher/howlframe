package main

import (
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: json_get <file> <key>")
		return 2
	}
	b, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	var obj map[string]any
	if err := json.Unmarshal(b, &obj); err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	v, ok := obj[args[1]]
	if !ok {
		fmt.Println("NOT FOUND")
		return 1
	}
	fmt.Println(v)
	return 0
}
