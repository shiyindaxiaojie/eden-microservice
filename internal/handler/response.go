package handler

import (
	"encoding/json"
	"net/http"
	"strings"
)

// httpError writes a JSON error response.
func httpError(w http.ResponseWriter, code int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// jsonOK writes a JSON success response.
func jsonOK(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// normalizeAddr ensures the address has a scheme and host.
func (h *Handler) normalizeAddr(addr string) string {
	if addr == "" {
		return ""
	}
	res := addr
	if res[0] == ':' {
		res = "127.0.0.1" + res
	}
	if !strings.HasPrefix(res, "http") {
		res = "http://" + res
	}
	return res
}

// handleLeaderRedirect returns a redirect response pointing to the current Raft leader if applicable.
func (h *Handler) handleLeaderRedirect(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")
	
	errMsg := err.Error()
	// Check if this is a Raft redirect error
	if strings.Contains(errMsg, "not leader") || strings.Contains(errMsg, "redirect to") {
		leader := h.cluster.LeaderAddr()
		if leader != "" {
			w.Header().Set("Location", h.normalizeAddr(leader))
			w.WriteHeader(http.StatusTemporaryRedirect)
			json.NewEncoder(w).Encode(map[string]string{
				"error":  "not leader",
				"leader": leader,
			})
			return
		}
	}

	w.WriteHeader(http.StatusInternalServerError)
	json.NewEncoder(w).Encode(map[string]string{
		"error": errMsg,
	})
}
