//go:build ignore

package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	data, err := os.ReadFile("in/ages.txt")
	if err != nil {
		panic(err)
	}
	counts := map[int]int{}
	for _, x := range strings.Split(strings.TrimSpace(string(data)), ",") {
		v, err := strconv.Atoi(x)
		if err != nil {
			panic(err)
		}
		counts[v/10]++
	}
	var buckets []int
	for b := range counts {
		buckets = append(buckets, b)
	}
	sort.Ints(buckets)
	var out []string
	for _, b := range buckets {
		out = append(out, fmt.Sprintf("%d-%d: %d", b*10, b*10+9, counts[b]))
	}
	if err := os.WriteFile("out/result.txt", []byte(strings.Join(out, "\n")+"\n"), 0o644); err != nil {
		panic(err)
	}
}
