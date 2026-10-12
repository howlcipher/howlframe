//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/stock.tsv")
	if err != nil {
		panic(err)
	}
	var rows [][]string
	for _, line := range strings.Split(string(data), "\n")[1:] {
		rows = append(rows, strings.Split(line, "\t"))
	}
	stock := map[string]int{}
	for _, row := range rows {
		qty, err := strconv.Atoi(row[1])
		if err != nil {
			panic(err)
		}
		stock[row[0]] = qty
	}
	out, err := json.MarshalIndent(stock, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(out, '\n'), 0o644); err != nil {
		panic(err)
	}
}
