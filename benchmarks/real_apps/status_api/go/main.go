package main

import (
	"encoding/json"
	"net/http"
	"os"
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
	http.HandleFunc("/ready", func(w http.ResponseWriter, r *http.Request) {
		if os.Getenv("REQUIRED_ENV") != "" {
			writeJSON(w, 200, map[string]string{"status": "ready"})
		} else {
			writeJSON(w, 503, map[string]string{"status": "not ready", "reason": "missing REQUIRED_ENV"})
		}
	})
	http.HandleFunc("/version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"version": "1.0.0", "app": "status_api"})
	})
	http.HandleFunc("/config", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, 200, map[string]string{"public_config": os.Getenv("PUBLIC_CONFIG")})
	})
	_ = http.ListenAndServe(":8090", nil)
}
