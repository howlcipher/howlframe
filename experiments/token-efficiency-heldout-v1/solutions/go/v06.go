//go:build ignore

package main

import (
	"os"
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
	var skus []string
	for _, row := range rows {
		if row[1] == "0" {
			skus = append(skus, row[0])
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(skus, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
