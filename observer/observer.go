package observer

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"plugin"
	"sync"
	"time"
)

// Trace logs the start and end of a function block
func Trace(funcName string, vars map[string]any) func() {
	varsJSON, _ := json.Marshal(vars)
	entryMsg := fmt.Sprintf("{\"event\":\"enter\", \"func\":%q, \"vars\":%s}\n", funcName, varsJSON)

	f, err := os.OpenFile("telemetry.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err == nil {
		f.WriteString(entryMsg)
	}

	return func() {
		exitMsg := fmt.Sprintf("{\"event\":\"exit\", \"func\":%q}\n", funcName)
		if f != nil {
			f.WriteString(exitMsg)
			f.Close()
		}
	}
}

var optimizeCache sync.Map
var optimizing sync.Map

func GetOptimizedPlugin(metric string) (func(), bool) {
	if p, ok := optimizeCache.Load(metric); ok {
		return p.(func()), true
	}
	return nil, false
}

func OptimizeGoImplementation(metric string, originalCode string) {
	if _, loading := optimizing.LoadOrStore(metric, true); loading {
		return
	}
	defer optimizing.Delete(metric)

	prompt := fmt.Sprintf("You are an expert Go developer. Rewrite the following Go code to be significantly more optimized. Return ONLY the raw Go code starting with 'package main' and containing an exported 'func Execute()'. Include all necessary imports. Do not include markdown formatting.\n\nCode to optimize:\n%s", originalCode)

	reqBody, _ := json.Marshal(map[string]any{
		"model":  "llama3",
		"prompt": prompt,
		"stream": false,
	})
	resp, err := http.Post("http://localhost:11434/api/generate", "application/json", bytes.NewReader(reqBody))
	if err != nil {
		return
	}
	defer resp.Body.Close()

	var res struct {
		Response string `json:"response"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return
	}

	pluginFilename := fmt.Sprintf("plugin_%s_%d.go", metric, time.Now().UnixNano())
	soFilename := fmt.Sprintf("plugin_%s_%d.so", metric, time.Now().UnixNano())

	if err := os.WriteFile(pluginFilename, []byte(res.Response), 0644); err != nil {
		return
	}
	defer os.Remove(pluginFilename)

	cmd := exec.Command("go", "build", "-buildmode=plugin", "-o", soFilename, pluginFilename)
	if err := cmd.Run(); err != nil {
		return
	}
	defer os.Remove(soFilename)

	p, err := plugin.Open(soFilename)
	if err != nil {
		return
	}

	sym, err := p.Lookup("Execute")
	if err != nil {
		return
	}

	if fn, ok := sym.(func()); ok {
		optimizeCache.Store(metric, fn)
	}
}
