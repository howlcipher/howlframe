//go:build ignore

package main

import (
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
	best, bestPrice := "", -1
	for _, row := range rows {
		price, err := strconv.Atoi(row[2])
		if err != nil {
			panic(err)
		}
		if price > bestPrice {
			best, bestPrice = row[0], price
		}
	}
	if err := os.WriteFile("out/result.txt", []byte(best+"\n"), 0o644); err != nil {
		panic(err)
	}
}
