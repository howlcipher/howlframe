package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	if len(args) < 2 || args[0] == "" || args[1] == "" {
		fmt.Println("Usage: file_transform <in.csv> <out.jsonl>")
		return 2
	}
	raw, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	n := 0
	var buf strings.Builder
	for _, row := range strings.Split(string(raw), "\n") {
		if row == "" {
			continue
		}
		cols := strings.Split(row, ",")
		if len(cols) >= 2 {
			n++
			obj := map[string]string{"c0": cols[0], "c1": cols[1]}
			b, _ := json.Marshal(obj)
			buf.Write(b)
			buf.WriteByte('\n')
		}
	}
	if err := os.WriteFile(args[1], []byte(buf.String()), 0644); err != nil {
		fmt.Printf("error=%v\n", err)
		return 2
	}
	fmt.Printf("rows=%d\n", n)
	fmt.Printf("out=%s\n", args[1])
	return 0
}
