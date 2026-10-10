package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, body map[string]string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(body)
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("/secret", func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") == "Bearer secret" {
			writeJSON(w, 200, map[string]string{"ok": "1", "scope": "secret"})
		} else {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
		}
	})
	mux.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			writeJSON(w, 405, map[string]string{"error": "method_not_allowed"})
			return
		}
		if r.Header.Get("Authorization") != "Bearer secret" {
			writeJSON(w, 401, map[string]string{"error": "unauthorized"})
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/echo/")
		writeJSON(w, 200, map[string]string{"name": name, "q": r.URL.Query().Get("q")})
	})
	_ = http.ListenAndServe(":8102", mux)
}
