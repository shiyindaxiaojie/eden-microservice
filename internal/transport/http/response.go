package httpapi

import (
	"encoding/json"
	"net/http"
	"strings"

	clusterpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
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
	// If it's a raw port like ":8500", prefix with 127.0.0.1
	if strings.HasPrefix(res, ":") {
		res = "127.0.0.1" + res
	}
	// If it has broad-listening host [::] or 0.0.0.0, replace with 127.0.0.1
	res = strings.Replace(res, "[::]", "127.0.0.1", 1)
	res = strings.Replace(res, "0.0.0.0", "127.0.0.1", 1)

	if !strings.HasPrefix(res, "http") {
		res = "http://" + res
	}
	return res
}

// writeLeaderRedirect returns a redirect response pointing to the current Raft leader if applicable.
func (h *Handler) writeLeaderRedirect(w http.ResponseWriter, err error) {
	w.Header().Set("Content-Type", "application/json")

	errMsg := err.Error()
	// Check if this is a Raft redirect error
	if strings.Contains(errMsg, "not leader") || strings.Contains(errMsg, "redirect to") {
		leader := h.leaderHTTPAddr()
		if leader != "" {
			w.Header().Set("Location", leader)
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

func (h *Handler) leaderHTTPAddr() string {
	if h == nil || h.cluster == nil {
		return ""
	}

	leaderRaftAddr := strings.TrimSpace(h.cluster.LeaderAddr())
	if leaderRaftAddr == "" {
		return h.normalizeAddr(h.config.HTTPAddr)
	}

	members, err := clusterpkg.BuildClusterMemberViews(h.config, h.settings, h.cluster, &h.nodeCache)
	if err == nil {
		for _, member := range members {
			if member == nil {
				continue
			}
			if member.Role == "Leader" && member.HTTPAddr != "" {
				return h.normalizeAddr(member.HTTPAddr)
			}
			if member.RaftAddr != "" && member.RaftAddr == leaderRaftAddr && member.HTTPAddr != "" {
				return h.normalizeAddr(member.HTTPAddr)
			}
			if member.Address != "" && member.Address == leaderRaftAddr && member.HTTPAddr != "" {
				return h.normalizeAddr(member.HTTPAddr)
			}
		}
	}

	return h.normalizeAddr(leaderRaftAddr)
}
