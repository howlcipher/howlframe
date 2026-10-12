//go:build ignore

package main

import (
	"os"
	"sort"
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
	sort.SliceStable(rows, func(i, j int) bool { return rows[i][0] < rows[j][0] })
	out := make([]string, len(rows))
	for i, row := range rows {
		out[i] = strings.Join(row, "\t")
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
