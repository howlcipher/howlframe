//go:build ignore

package main

import (
	"encoding/json"
	"os"
	"strconv"
	"strings"
)

func mustRead(path string) []byte {
	data, err := os.ReadFile(path)
	if err != nil {
		panic(err)
	}
	return data
}

func version(text string) [3]int {
	var result [3]int
	for i, part := range strings.Split(text, ".") {
		n, err := strconv.Atoi(part)
		if err != nil {
			panic(err)
		}
		result[i] = n
	}
	return result
}

func main() {
	var checks []map[string]interface{}
	if err := json.Unmarshal(mustRead("evidence/checks.json"), &checks); err != nil {
		panic(err)
	}
	versionText := mustRead("evidence/version.kv")
	mustRead("evidence/note.txt")
	selected := make([]map[string]interface{}, 0)
	allPassed := true
	for _, check := range checks {
		if check["name"] == "unit" {
			selected = append(selected, check)
		}
		if check["status"] != "PASS" {
			allPassed = false
		}
	}
	record := map[string]interface{}{"batch": "17", "selected": selected}
	versions := make(map[string]string)
	for _, line := range strings.Fields(string(versionText)) {
		key, value, _ := strings.Cut(line, "=")
		versions[key] = value
	}
	current, proposed := version(versions["current"]), version(versions["proposed"])
	newer := false
	for i := 0; i < 3; i++ {
		if proposed[i] != current[i] {
			newer = proposed[i] > current[i]
			break
		}
	}
	path := "out/release.json"
	if !allPassed || !newer {
		path = "out/approval_request.json"
		record = map[string]interface{}{"reason": "blocked", "summary": map[string]interface{}{}}
	}
	data, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		panic(err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0600); err != nil {
		panic(err)
	}
}
