package cluster

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/shiyindaxiaojie/eden-registry/internal/config"
)

// ClusterMemberView is the normalized member payload shared by HTTP and gRPC endpoints.
type ClusterMemberView struct {
	ID       string `json:"id"`
	Address  string `json:"address"`
	Status   string `json:"status"`
	Role     string `json:"role"`
	IsLocal  bool   `json:"is_local"`
	HTTPAddr string `json:"http_addr,omitempty"`
	GRPCAddr string `json:"grpc_addr,omitempty"`
	QUICAddr string `json:"quic_addr,omitempty"`
	RaftAddr string `json:"raft_addr,omitempty"`
}

// BuildClusterMemberViews assembles the cluster member list exposed to clients.
type SettingsReader interface {
	GetMode() string
	GetEnvironment() string
	GetSeeds() []string
}

func BuildClusterMemberViews(cfg *config.Config, settings SettingsReader, cluster Membership, nodeCache *sync.Map) ([]*ClusterMemberView, error) {
	mode := settings.GetMode()
	env := settings.GetEnvironment()
	localAddr := normalizeHTTPAddr(cfg.Server.HTTP)
	seeds := settings.GetSeeds()

	raftMembersMap := make(map[string]map[string]string)
	if mode == "cp" && env == "cluster" {
		if raftMembers, err := cluster.GetMembers(); err == nil && raftMembers != nil {
			var membersList []ClusterMember
			if ms, ok := raftMembers.([]ClusterMember); ok {
				membersList = ms
			} else if ims, ok := raftMembers.([]interface{}); ok {
				for _, item := range ims {
					if m, ok := item.(ClusterMember); ok {
						membersList = append(membersList, m)
					}
				}
			}

			for _, rm := range membersList {
				b, _ := json.Marshal(rm)
				var m map[string]string
				_ = json.Unmarshal(b, &m)
				if m["id"] != "" {
					raftMembersMap[m["id"]] = m
				}
			}
		}
	}

	fetchNodeInfo := func(addr string, isLocal bool, raftID string) *ClusterMemberView {
		info := &ClusterMemberView{
			Address: addr,
			Status:  "Offline",
			Role:    "Peer",
			IsLocal: isLocal,
		}
		if env == "standalone" {
			info.Role = "Standalone"
		}

		if isLocal {
			info.ID = cfg.NodeID
			info.Status = "Online"
			info.HTTPAddr = normalizeDisplayAddr(cfg.Server.HTTP)
			info.GRPCAddr = normalizeDisplayAddr(cfg.Server.GRPC)
			info.RaftAddr = normalizeDisplayAddr(cfg.Server.Raft)
			info.QUICAddr = normalizeDisplayAddr(cfg.Server.QUIC)
		} else {
			client := http.Client{Timeout: 500 * time.Millisecond}
			resp, err := client.Get(addr + "/v1/node/info")
			if err == nil {
				defer resp.Body.Close()
				var remoteCfg config.Config
				if json.NewDecoder(resp.Body).Decode(&remoteCfg) == nil {
					info.ID = remoteCfg.NodeID
					info.Status = "Online"
					info.HTTPAddr = normalizeDisplayAddr(remoteCfg.Server.HTTP)
					info.GRPCAddr = normalizeDisplayAddr(remoteCfg.Server.GRPC)
					info.RaftAddr = normalizeDisplayAddr(remoteCfg.Server.Raft)
					info.QUICAddr = normalizeDisplayAddr(remoteCfg.Server.QUIC)
					nodeCache.Store(addr, remoteCfg)
				}
			} else if val, ok := nodeCache.Load(addr); ok {
				if cached, ok := val.(config.Config); ok {
					info.ID = cached.NodeID
					info.HTTPAddr = normalizeDisplayAddr(cached.Server.HTTP)
					info.GRPCAddr = normalizeDisplayAddr(cached.Server.GRPC)
					info.RaftAddr = normalizeDisplayAddr(cached.Server.Raft)
					info.QUICAddr = normalizeDisplayAddr(cached.Server.QUIC)
				}
			}

			if info.ID == "" {
				if raftID != "" {
					info.ID = raftID
				} else {
					info.ID = fmt.Sprintf("peer-node-(%s)", addr)
				}
			}
		}

		if mode == "cp" && env == "cluster" {
			if rm, exists := raftMembersMap[info.ID]; exists {
				info.Role = rm["role"]
				if info.Status == "Offline" && rm["address"] != "" {
					info.RaftAddr = normalizeDisplayAddr(rm["address"])
				}
			}
		}

		return info
	}

	seedToRaftID := make(map[string]string)
	if mode == "cp" && env == "cluster" && len(seeds) > 0 {
		hostSeeds := make(map[string][]string)
		for _, s := range seeds {
			cleaned := strings.TrimPrefix(strings.TrimPrefix(s, "http://"), "https://")
			if normalizeHTTPAddr(s) == localAddr {
				continue
			}
			host, _, _ := net.SplitHostPort(cleaned)
			if host == "" {
				host = cleaned
			}
			hostSeeds[host] = append(hostSeeds[host], s)
		}

		hostRaft := make(map[string][]string)
		raftAddrToID := make(map[string]string)
		localRaftAddr := cfg.Server.Raft
		for id, rm := range raftMembersMap {
			raftAddrToID[rm["address"]] = id
			if rm["address"] == localRaftAddr {
				continue
			}
			host, _, _ := net.SplitHostPort(rm["address"])
			if host != "" {
				hostRaft[host] = append(hostRaft[host], rm["address"])
			}
		}

		sortByPort := func(list []string) {
			sort.Slice(list, func(i, j int) bool {
				_, p1, _ := net.SplitHostPort(list[i])
				if p1 == "" {
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

		for host, sList := range hostSeeds {
			rList := hostRaft[host]
			if len(sList) == 0 || len(rList) == 0 {
				continue
			}
			sortByPort(sList)
			sortByPort(rList)
			if len(sList) == len(rList) {
				for i := 0; i < len(sList); i++ {
					seedToRaftID[sList[i]] = raftAddrToID[rList[i]]
				}
			}
		}
	}

	membersResults := make([]*ClusterMemberView, 0)
	membersResults = append(membersResults, fetchNodeInfo(localAddr, true, ""))

	if env != "standalone" {
		var mu sync.Mutex
		var wg sync.WaitGroup
		for _, seed := range seeds {
			if seed == "" || normalizeHTTPAddr(seed) == localAddr {
				continue
			}
			wg.Add(1)
			go func(s string, rid string) {
				defer wg.Done()
				info := fetchNodeInfo(s, false, rid)
				mu.Lock()
				membersResults = append(membersResults, info)
				mu.Unlock()
			}(seed, seedToRaftID[seed])
		}
		wg.Wait()
	}

	sort.Slice(membersResults, func(i, j int) bool {
		if membersResults[i].IsLocal {
			return true
		}
		if membersResults[j].IsLocal {
			return false
		}
		return membersResults[i].ID < membersResults[j].ID
	})

	return membersResults, nil
}

func normalizeHTTPAddr(addr string) string {
	if addr == "" {
		return ""
	}
	res := addr
	if strings.HasPrefix(res, ":") {
		res = "127.0.0.1" + res
	}
	res = strings.Replace(res, "[::]", "127.0.0.1", 1)
	res = strings.Replace(res, "0.0.0.0", "127.0.0.1", 1)
	if !strings.HasPrefix(res, "http") {
		res = "http://" + res
	}
	return res
}

func normalizeDisplayAddr(addr string) string {
	if addr == "" {
		return ""
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		if strings.HasPrefix(addr, ":") {
			return "127.0.0.1" + addr
		}
		return addr
	}
	if host == "" || host == "::" || host == "0.0.0.0" || host == "[::]" {
		return "127.0.0.1:" + port
	}
	return addr
}
