package handler

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
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
	mode := h.settings.GetMode()
	env := h.settings.GetEnvironment()
	localAddr := h.normalizeAddr(h.config.HTTPAddr)
	seeds := h.settings.GetSeeds()

	// 1. Get Raft members first if in CP mode to determine Leader/Follower
	raftMembersMap := make(map[string]map[string]string)
	if mode == "cp" && env == "cluster" {
		if raftMembers, err := h.cluster.GetMembers(); err == nil && raftMembers != nil {
			// In Go, []Structure cannot be cast to []interface{} directly.
			// Let's handle both possible return types from CPNode.Members()
			var membersList []cp.ClusterMember
			if ms, ok := raftMembers.([]cp.ClusterMember); ok {
				membersList = ms
			} else if ims, ok := raftMembers.([]interface{}); ok {
				// Fallback if it was somehow returned as []interface{} (e.g. from JSON)
				for _, item := range ims {
					if m, ok := item.(cp.ClusterMember); ok {
						membersList = append(membersList, m)
					}
				}
			}

			for _, rm := range membersList {
				// We depend on json serialization output from CPNode.Members()
				b, _ := json.Marshal(rm)
				var m map[string]string
				json.Unmarshal(b, &m)
				if m["id"] != "" {
					raftMembersMap[m["id"]] = m
				}
			}
		}
	}

	members := make([]map[string]interface{}, 0)

	// Helper to normalize host:port string to include an IP if it's [::] or :port
	normalizeDisplayAddr := func(addr string) string {
		if addr == "" {
			return ""
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			// Not a host:port, maybe just port?
			if strings.HasPrefix(addr, ":") {
				return "127.0.0.1" + addr
			}
			return addr
		}
		if host == "" || host == "::" || host == "0.0.0.0" {
			return "127.0.0.1:" + port
		}
		return addr
	}

	// Helper to fetch node info
	fetchNodeInfo := func(addr string, isLocal bool, raftID string) map[string]interface{} {
		info := map[string]interface{}{
			"address":  addr,
			"status":   "Offline",
			"role":     "Peer",
			"is_local": isLocal,
		}
		if env == "standalone" {
			info["role"] = "Standalone"
		}

		if isLocal {
			info["id"] = h.config.NodeID
			info["status"] = "Online"
			info["http_addr"] = normalizeDisplayAddr(h.config.HTTPAddr)
			info["grpc_addr"] = normalizeDisplayAddr(h.config.GRPCAddr)
			info["raft_addr"] = normalizeDisplayAddr(h.config.RaftAddr)
			info["quic_addr"] = normalizeDisplayAddr(h.config.QUICAddr)
		} else {
			client := http.Client{Timeout: 500 * time.Millisecond}
			resp, err := client.Get(addr + "/v1/node/info")
			if err == nil {
				defer resp.Body.Close()
				var remoteCfg config.Config
				if json.NewDecoder(resp.Body).Decode(&remoteCfg) == nil {
					info["id"] = remoteCfg.NodeID
					info["status"] = "Online"
					info["http_addr"] = normalizeDisplayAddr(remoteCfg.HTTPAddr)
					info["grpc_addr"] = normalizeDisplayAddr(remoteCfg.GRPCAddr)
					info["raft_addr"] = normalizeDisplayAddr(remoteCfg.RaftAddr)
					info["quic_addr"] = normalizeDisplayAddr(remoteCfg.QUICAddr)
					// Cache it for offline display
					h.nodeCache.Store(addr, remoteCfg)
				}
			} else {
				// Try cache
				if val, ok := h.nodeCache.Load(addr); ok {
					if cached, ok := val.(config.Config); ok {
						info["id"] = cached.NodeID
						info["http_addr"] = normalizeDisplayAddr(cached.HTTPAddr)
						info["grpc_addr"] = normalizeDisplayAddr(cached.GRPCAddr)
						info["raft_addr"] = normalizeDisplayAddr(cached.RaftAddr)
						info["quic_addr"] = normalizeDisplayAddr(cached.QUICAddr)
					}
				}
			}
			// Fallback to heuristic ID if still unknown
			if id, _ := info["id"].(string); id == "" {
				if raftID != "" {
					info["id"] = raftID
				} else {
					info["id"] = fmt.Sprintf("peer-node-(%s)", addr)
				}
			}
		}

		// Overlay Raft role if CP mode
		if mode == "cp" && env == "cluster" {
			nodeID, _ := info["id"].(string)
			if rm, exists := raftMembersMap[nodeID]; exists {
				info["role"] = rm["role"]
				// If offline but we have raft port from cluster config, show it
				if info["status"] == "Offline" && rm["address"] != "" {
					info["raft_addr"] = normalizeDisplayAddr(rm["address"])
				}
			}
		}

		return info
	}

	// 2. Correlation Heuristic: Match seeds to Raft members by host + port order
	seedToRaftID := make(map[string]string)
	if mode == "cp" && env == "cluster" && len(seeds) > 0 {
		// Group seeds by host (excluding local)
		hostSeeds := make(map[string][]string)
		for _, s := range seeds {
			cleaned := strings.TrimPrefix(strings.TrimPrefix(s, "http://"), "https://")
			if h.normalizeAddr(s) == localAddr { continue }
			host, _, _ := net.SplitHostPort(cleaned)
			if host == "" { host = cleaned }
			hostSeeds[host] = append(hostSeeds[host], s)
		}
		// Group Raft members by host (excluding local)
		hostRaft := make(map[string][]string)
		raftAddrToID := make(map[string]string)
		localRaftAddr := h.config.RaftAddr
		for id, rm := range raftMembersMap {
			raftAddrToID[rm["address"]] = id
			if rm["address"] == localRaftAddr { continue }
			host, _, _ := net.SplitHostPort(rm["address"])
			if host != "" {
				hostRaft[host] = append(hostRaft[host], rm["address"])
			}
		}
		// Sort both by port and match
		for host, sList := range hostSeeds {
			rList := hostRaft[host]
			if len(sList) > 0 && len(rList) > 0 {
				// Sort by port numbers (extracting from addr)
				sortFunc := func(list []string) {
					sort.Slice(list, func(i, j int) bool {
						_, p1, _ := net.SplitHostPort(list[i])
						if p1 == "" { // Handle HTTP strings like http://...:8500
							parts := strings.Split(list[i], ":")
							p1 = parts[len(parts)-1]
						}
						_, p2, _ := net.SplitHostPort(list[j])
						if p2 == "" {
							parts := strings.Split(list[j], ":")
							p2 = parts[len(parts)-1]
						}
						return p1 < p2
					})
				}
				sortFunc(sList)
				sortFunc(rList)
				// If lengths match, we can match by order (common in local tests)
				if len(sList) == len(rList) {
					for i := 0; i < len(sList); i++ {
						seedToRaftID[sList[i]] = raftAddrToID[rList[i]]
					}
				}
			}
		}
	}

	// 3. Assemble final list
	members = append(members, fetchNodeInfo(localAddr, true, ""))

	if env != "standalone" {
		for _, seed := range seeds {
			if seed != "" && seed != localAddr {
				members = append(members, fetchNodeInfo(seed, false, seedToRaftID[seed]))
			}
		}
	}

	jsonOK(w, members)
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
				if s == "" || h.normalizeAddr(s) == localAddr { continue }
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

	if nodeCount < 1 { nodeCount = 1 }
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
	var u model.User
	json.NewDecoder(r.Body).Decode(&u)
	h.settings.AddUser(&u)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalDeleteUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.DeleteUser(req.Username)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncAPIKey(w http.ResponseWriter, r *http.Request) {
	var k model.APIKey
	json.NewDecoder(r.Body).Decode(&k)
	h.settings.AddAPIKey(&k)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalDeleteAPIKey(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Key string `json:"key"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	h.settings.DeleteAPIKey(req.Key)
	jsonOK(w, map[string]string{"status": "ok"})
}

func (h *Handler) handleInternalSyncSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Mode        string `json:"mode,omitempty"`
		Environment string `json:"environment,omitempty"`
		LogLevel    string `json:"log_level,omitempty"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	// Save directly to avoid re-broadcast (prevent infinite loop)
	if req.Mode != "" {
		h.settings.SaveSettingLocal("mode", req.Mode)
	}
	if req.Environment != "" {
		h.settings.SaveSettingLocal("environment", req.Environment)
	}
	if req.LogLevel != "" {
		h.settings.SaveSettingLocal("log_level", req.LogLevel)
	}
	jsonOK(w, map[string]string{"status": "ok"})
}
