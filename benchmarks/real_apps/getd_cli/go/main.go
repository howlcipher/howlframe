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
		fmt.Println("Usage: getd_cli <file> <key> [default]")
		return 2
	}
	def := "MISSING"
	if len(args) > 2 && args[2] != "" {
		def = args[2]
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	if v, ok := obj[args[1]]; ok {
		fmt.Println(v)
		return 0
	}
	fmt.Println(def)
	return 0
}
