//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/data.csv")
	if err != nil {
		panic(err)
	}
	lines := strings.Split(string(data), "\n")
	header := strings.Split(lines[0], ",")
	rows := []map[string]string{}
	for _, line := range lines[1:] {
		row := map[string]string{}
		for i, value := range strings.Split(line, ",") {
			row[header[i]] = value
		}
		rows = append(rows, row)
	}
	out, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
