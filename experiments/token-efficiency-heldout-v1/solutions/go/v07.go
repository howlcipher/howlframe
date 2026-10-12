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
	total := 0
	for _, row := range rows {
		qty, err := strconv.Atoi(row[1])
		if err != nil {
			panic(err)
		}
		price, err := strconv.Atoi(row[2])
		if err != nil {
			panic(err)
		}
		total += qty * price
	}
	if err := os.WriteFile("out/result.txt", []byte(strconv.Itoa(total)+"\n"), 0o644); err != nil {
		panic(err)
	}
}
