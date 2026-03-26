package httpapi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	clusterpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
)

// ---------- Cluster Handlers (Membership & Stats) ----------

func (h *Handler) handleNodeInfo(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, h.config)
}

func (h *Handler) handleJoin(w http.ResponseWriter, r *http.Request) {
	mode := h.settings.GetMode()
	if mode == "ap" {
		jsonOK(w, map[string]string{"status": "ignored_in_ap_mode"})
		return
	}
	var req struct {
		NodeID  string `json:"node_id"`
		Address string `json:"address"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return
	}
	if err := h.cluster.JoinCluster(req.NodeID, req.Address); err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleMembers(w http.ResponseWriter, r *http.Request) {
	membersResults, err := clusterpkg.BuildClusterMemberViews(h.config, h.settings, h.cluster, &h.nodeCache)
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, membersResults)
}

func (h *Handler) handleMember(w http.ResponseWriter, r *http.Request) {
	mode := h.settings.GetMode()
	if r.Method == http.MethodPost {
		var req struct {
			Addresses []string `json:"addresses"` // Support multiple addresses at once
		}
		json.NewDecoder(r.Body).Decode(&req)

		if len(req.Addresses) == 0 {
			httpError(w, http.StatusBadRequest, "addresses required")
			return
		}

		seeds := h.settings.GetSeeds()
		seedsMap := make(map[string]bool)
		for _, s := range seeds {
			seedsMap[s] = true
		}

		env := h.settings.GetEnvironment()
		// If we are adding nodes and currently standalone, transition to cluster environment FIRST
		if env == "standalone" && len(req.Addresses) > 0 {
			if err := h.settings.SetEnvironment("cluster"); err != nil {
				httpError(w, http.StatusInternalServerError, "failed to transition environment: "+err.Error())
				return
			}
			env = "cluster"
		}

		var lastErr error

		for _, addr := range req.Addresses {
			addr := h.normalizeAddr(addr)

			// 1. Fetch node info to get RaftAddr and NodeID
			client := http.Client{Timeout: 3 * time.Second}
			resp, err := client.Get(addr + "/v1/node/info")
			if err != nil {
				lastErr = fmt.Errorf("failed to fetch info from %s: %v", addr, err)
				continue
			}
			defer resp.Body.Close()

			var remoteCfg config.Config
			if err := json.NewDecoder(resp.Body).Decode(&remoteCfg); err != nil {
				lastErr = fmt.Errorf("failed to decode info from %s: %v", addr, err)
				continue
			}

			// 2. Add to seeds (always needed for AP and frontend display)
			if !seedsMap[addr] && addr != h.normalizeAddr(h.config.HTTPAddr) {
				seeds = append(seeds, addr)
				seedsMap[addr] = true
			}

			// 3. If in CP mode, join the Raft cluster
			if mode == "cp" && env == "cluster" {
				if remoteCfg.NodeID == "" || remoteCfg.RaftAddr == "" {
					lastErr = fmt.Errorf("node %s missing node_id or raft_addr", addr)
					continue
				}
				if err := h.cluster.JoinCluster(remoteCfg.NodeID, remoteCfg.RaftAddr); err != nil {
					lastErr = fmt.Errorf("failed to join %s to raft: %v", addr, err)
					// Don't continue, joining one raft node might redirect us to leader
					if err.Error() == "not leader" {
						h.handleLeaderRedirect(w, err)
						return
					}
				}
			}
		}

		// 5. Save updated seeds (this also triggers sync to peers)
		if err := h.settings.SetSeeds(seeds); err != nil {
			httpError(w, http.StatusInternalServerError, err.Error())
			return
		}

		if lastErr != nil {
			// Note: We return 200 with error details if partial failure, or 500 if we want strict
			// For simplicity let's return error if we couldn't add some nodes.
			// But since we saved seeds, maybe just return the last error.
			httpError(w, http.StatusInternalServerError, lastErr.Error())
			return
		}

		jsonOK(w, map[string]string{"status": "ok"})
		return
	}

	if r.Method == http.MethodDelete {
		addr := r.URL.Query().Get("address")
		nodeID := r.URL.Query().Get("node_id")
		if addr == "" {
			httpError(w, http.StatusBadRequest, "address required")
			return
		}

		env := h.settings.GetEnvironment()
		if env == "standalone" || mode == "ap" {
			seeds := h.settings.GetSeeds()
			newSeeds := []string{}
			for _, s := range seeds {
				if s != addr {
					newSeeds = append(newSeeds, s)
				}
			}
			if err := h.settings.SetSeeds(newSeeds); err != nil {
				httpError(w, http.StatusInternalServerError, err.Error())
				return
			}
			jsonOK(w, map[string]string{"status": "ok"})
			return
		}

		if mode == "cp" {
			if nodeID == "" {
				httpError(w, http.StatusBadRequest, "node_id required for CP mode removal")
				return
			}
			if err := h.cluster.RemoveMember(nodeID); err != nil {
				h.handleLeaderRedirect(w, err)
				return
			}
		}
		jsonOK(w, map[string]string{"status": "ok"})
		return
	}

	httpError(w, http.StatusMethodNotAllowed, "method not allowed")
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.cluster.GetStats()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}

	mode := h.settings.GetMode()
	env := h.settings.GetEnvironment()
	localAddr := h.normalizeAddr(h.config.HTTPAddr)
	isLeader := h.cluster.IsLeader()
	leaderAddr := h.cluster.LeaderAddr()

	role := "Peer"
	if env == "standalone" {
		role = "Standalone"
		leaderAddr = h.normalizeAddr(h.config.HTTPAddr)
		isLeader = true
	} else if mode == "ap" {
		role = "Peer"
		leaderAddr = h.normalizeAddr(h.config.HTTPAddr)
		isLeader = false
	} else {
		// CP mode
		if isLeader {
			role = "Leader"
		} else {
			role = "Follower"
		}
		if leaderAddr == "" {
			leaderAddr = h.normalizeAddr(h.config.HTTPAddr)
		}
	}

	nodeCount := 1
	healthyNodes := 1 // Local node is always healthy if we are running
	if env == "cluster" {
		if mode == "cp" {
			res, _ := h.cluster.GetMembers()
			if ms, ok := res.([]cp.ClusterMember); ok {
				nodeCount = len(ms)
				healthyNodes = 0
				for _, m := range ms {
					if m.Status == "Online" {
						healthyNodes++
					}
				}
			} else if ims, ok := res.([]interface{}); ok {
				nodeCount = len(ims)
				healthyNodes = 0
				for _, m := range ims {
					if mm, ok := m.(map[string]interface{}); ok {
						if mm["status"] == "Online" {
							healthyNodes++
						}
					} else if cm, ok := m.(cp.ClusterMember); ok {
						if cm.Status == "Online" {
							healthyNodes++
						}
					}
				}
			}
		} else {
			seeds := h.settings.GetSeeds()
			nodeCount = len(seeds) + 1
			// Quick parallel health check for AP seeds
			var wg sync.WaitGroup
			var mu sync.Mutex
			for _, s := range seeds {
				if s == "" || h.normalizeAddr(s) == localAddr {
					continue
				}
				wg.Add(1)
				go func(addr string) {
					defer wg.Done()
					client := http.Client{Timeout: 200 * time.Millisecond}
					resp, err := client.Get(addr + "/health")
					if err == nil {
						resp.Body.Close()
						mu.Lock()
						healthyNodes++
						mu.Unlock()
					}
				}(s)
			}
			wg.Wait()
		}
	}

	if nodeCount < 1 {
		nodeCount = 1
	}
	nodeHealthRate := float64(healthyNodes) / float64(nodeCount) * 100

	result := map[string]interface{}{
		"node_count":     nodeCount,
		"is_leader":      isLeader,
		"leader_addr":    leaderAddr,
		"mode":           mode,
		"environment":    env,
		"role":           role,
		"service_count":  stats.ServiceCount,
		"instance_count": stats.InstanceCount,
		"healthy_count":  stats.HealthyCount,
		"health_rate":    nodeHealthRate, // Return node health rate here for cluster dashboard
		"memory_usage":   stats.MemoryUsage,
	}
	jsonOK(w, result)
}

func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	events, err := h.cluster.ListEvents()
	if err != nil {
		httpError(w, http.StatusInternalServerError, err.Error())
		return
	}
	jsonOK(w, events)
}

func (h *Handler) handleUpdateStorage(w http.ResponseWriter, r *http.Request) {
	var req struct {
		EventRetention int      `json:"event_retention"`
		LogLevel       string   `json:"log_level"`
		LogRetention   int      `json:"log_retention"`
		EventTypes     []string `json:"event_types"`
		HBMaxFail      int      `json:"heartbeat_max_failures"`
		RemovalDelay   int      `json:"instance_removal_delay_seconds"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid body")
		return
	}

	if req.EventRetention > 0 {
		if err := h.settings.SetEventRetentionDays(req.EventRetention); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	}
	if req.LogLevel != "" {
		if err := h.settings.SetLogLevel(req.LogLevel); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	}
	if req.LogRetention > 0 {
		if err := h.settings.SetLogRetentionDays(req.LogRetention); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	}
	if req.EventTypes != nil {
		if err := h.settings.SetEventTypes(req.EventTypes); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	}
	if req.HBMaxFail > 0 {
		if err := h.settings.SetHeartbeatMaxFailures(req.HBMaxFail); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	}
	if req.RemovalDelay > 0 {
		if err := h.settings.SetInstanceRemovalDelaySeconds(req.RemovalDelay); err != nil {
			h.handleLeaderRedirect(w, err)
			return
		}
	}

	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleGetStorage(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]interface{}{
		"log_level":                      h.settings.GetLogLevel(),
		"event_retention":                h.settings.GetEventRetentionDays(),
		"log_retention":                  h.settings.GetLogRetentionDays(),
		"event_types":                    h.settings.GetEventTypes(),
		"heartbeat_max_failures":         h.settings.GetHeartbeatMaxFailures(),
		"instance_removal_delay_seconds": h.settings.GetInstanceRemovalDelaySeconds(),
	})
}

func (h *Handler) handleGetLogFiles(w http.ResponseWriter, r *http.Request) {
	var files []map[string]string
	for _, appender := range h.config.Log.Appenders {
		if appender.FileName != "" {
			files = append(files, map[string]string{
				"name": appender.Name,
				"file": appender.FileName,
			})
		}
	}
	// Fallback/Default if none configured
	if len(files) == 0 {
		files = append(files, map[string]string{
			"name": "InfoLog",
			"file": "logs/info.log",
		})
	}
	jsonOK(w, files)
}

func (h *Handler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	countStr := r.URL.Query().Get("count")
	count := 100
	if countStr != "" {
		fmt.Sscanf(countStr, "%d", &count)
	}

	fileName := r.URL.Query().Get("file")
	if fileName == "" {
		fileName = "logs/info.log" // Default
	}

	// Validate fileName is one of the appender's files to prevent arbitrary file reading
	isValid := false
	if fileName == "logs/info.log" {
		isValid = true
	} else {
		for _, appender := range h.config.Log.Appenders {
			if appender.FileName == fileName {
				isValid = true
				break
			}
		}
	}

	if !isValid {
		httpError(w, http.StatusForbidden, "Invalid log file")
		return
	}

	// Resolve path relative to DataDir if it doesn't exist directly
	logFile := fileName
	if _, err := os.Stat(logFile); os.IsNotExist(err) && h.config.DataDir != "" {
		logFile = filepath.Join(h.config.DataDir, fileName)
	}

	if _, err := os.Stat(logFile); os.IsNotExist(err) {
		jsonOK(w, []string{fmt.Sprintf("Log file [%s] not found", fileName)})
		return
	}

	file, err := os.Open(logFile)
	if err != nil {
		httpError(w, http.StatusInternalServerError, "Failed to open log file")
		return
	}
	defer file.Close()

	// tail -n count
	scanner := bufio.NewScanner(file)
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
		if len(lines) > count {
			lines = lines[len(lines)-count:]
		}
	}

	jsonOK(w, lines)
}

// Internal sync handlers
func (h *Handler) handleInternalSyncSeeds(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Seeds []string `json:"seeds"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	// Save locally only, do NOT re-broadcast (avoid infinite loop)
	h.settings.SaveSeedsLocal(req.Seeds)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncUsers(w http.ResponseWriter, r *http.Request) {
	var u auth.User
	json.NewDecoder(r.Body).Decode(&u)
	h.settings.SaveUserLocal(&u)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalDeleteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.DeleteUserLocal(req.Username)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncAPIKey(w http.ResponseWriter, r *http.Request) {
	var k auth.APIKey
	json.NewDecoder(r.Body).Decode(&k)
	h.settings.SaveAPIKeyLocal(&k)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.DeleteAPIKeyLocal(req.Key)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncSettings(w http.ResponseWriter, r *http.Request) {
	var req map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		httpError(w, http.StatusBadRequest, "invalid sync body")
		return
	}
	// Save directly to avoid re-broadcast (prevent infinite loop)
	for k, v := range req {
		h.settings.SaveSettingLocalV2(k, v)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
