//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"reflect"
	"sort"
)

func readJSON(path string, v any) {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		panic(err)
	}
}

func writeJSON(v any) {
	enc, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile("out/result.json", append(enc, '\n'), 0o644); err != nil {
		panic(err)
	}
}

func main() {
	var old, nw map[string]any
	readJSON("in/old.json", &old)
	readJSON("in/new.json", &nw)
	out := map[string][]string{"added": {}, "removed": {}, "changed": {}}
	for k, v := range nw {
		if ov, ok := old[k]; !ok {
			out["added"] = append(out["added"], k)
		} else if !reflect.DeepEqual(ov, v) {
			out["changed"] = append(out["changed"], k)
		}
	}
	for k := range old {
		if _, ok := nw[k]; !ok {
			out["removed"] = append(out["removed"], k)
		}
	}
	for _, v := range out {
		sort.Strings(v)
	}
	writeJSON(out)
}
