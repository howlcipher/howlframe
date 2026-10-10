package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"sync"
)

func main() {
	var mu sync.Mutex
	n := 0
	writeJSON := func(w http.ResponseWriter, code int, body map[string]string) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(code)
		_ = json.NewEncoder(w).Encode(body)
	}
	http.HandleFunc("/count", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		cur := n
		mu.Unlock()
		writeJSON(w, 200, map[string]string{"n": strconv.Itoa(cur)})
	})
	http.HandleFunc("/inc", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
			return
		}
		mu.Lock()
		n++
		cur := n
		mu.Unlock()
		writeJSON(w, 200, map[string]string{"n": strconv.Itoa(cur)})
	})
	http.HandleFunc("/reset", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
			return
		}
		mu.Lock()
		n = 0
		mu.Unlock()
		writeJSON(w, 200, map[string]string{"n": "0"})
	})
	_ = http.ListenAndServe(":8103", nil)
}
