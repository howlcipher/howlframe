package main

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

func writeJSON(w http.ResponseWriter, code int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	http.HandleFunc("/now", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"unix": strconv.FormatInt(time.Now().Unix(), 10)})
	})
	http.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/echo/")
		writeJSON(w, 200, map[string]string{
			"name": name,
			"unix": strconv.FormatInt(time.Now().Unix(), 10),
		})
	})
	_ = http.ListenAndServe(":8104", nil)
}
