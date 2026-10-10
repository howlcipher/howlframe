package main

import (
	"encoding/json"
	"net/http"
	"strings"
)

func writeJSON(w http.ResponseWriter, code int, obj map[string]string) {
	b, _ := json.Marshal(obj)
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	w.Write(b)
}

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"status": "ok"})
	})
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"method": r.Method, "q": r.URL.Query().Get("q")})
	})
	http.HandleFunc("/echo/", func(w http.ResponseWriter, r *http.Request) {
		name := strings.TrimPrefix(r.URL.Path, "/echo/")
		writeJSON(w, 200, map[string]string{"name": name, "method": r.Method})
	})
	http.ListenAndServe("127.0.0.1:8106", nil)
}
