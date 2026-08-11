package main

import (
	"sort"
	"testing"
)

type Item struct {
	Name  string
	Score int
}

func ProcessScores(items []Item) []string {
	var filtered []Item
	for _, item := range items {
		if item.Score >= 50 {
			filtered = append(filtered, item)
		}
	}
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Score > filtered[j].Score
	})
	var result []string
	for _, item := range filtered {
		result = append(result, item.Name)
	}
	return result
}
func TestProcessScores(t *testing.T) {
	input := []Item{{"Alice", 40}, {"Bob", 80}, {"Charlie", 90}}
	res := ProcessScores(input)
	if len(res) != 2 || res[0] != "Charlie" || res[1] != "Bob" {
		t.Errorf("unexpected %v", res)
	}
}
