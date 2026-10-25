// Package health provides the standard /health handler every service exposes.
package health

import (
	"encoding/json"
	"net/http"
)

// Handler returns {"status":"ok","version":"..."}.
func Handler(version string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]string{"status": "ok", "version": version})
	})
}
