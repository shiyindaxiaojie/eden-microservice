package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shiyindaxiaojie/eden-go-registry/internal/ap"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/configs"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/handler"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/health"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	raftpkg "github.com/shiyindaxiaojie/eden-go-registry/internal/raft"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/utils"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	var (
		cfgFile   = flag.String("config", "configs/config.yaml", "Path to configuration file")
		httpAddr  = flag.String("http-addr", "", "Override HTTP API listen address")
		raftAddr  = flag.String("raft-addr", "", "Override Raft transport bind address")
		dataDir   = flag.String("data-dir", "", "Override Raft data directory")
		nodeID    = flag.String("node-id", "", "Override node ID")
		mode      = flag.String("mode", "", "Override mode (ap or cp)")
		bootstrap = flag.Bool("bootstrap", false, "Bootstrap as first node in new CP cluster")
		joinAddr  = flag.String("join", "", "Address of leader node to join in CP mode")
		seedsFlag = flag.String("seeds", "", "Override AP mode seeds (comma separated, e.g. http://127.0.0.1:8500,http://127.0.0.1:8501)")
		ttl       = flag.Duration("ttl", 30*time.Second, "Instance heartbeat TTL")
	)
	flag.Parse()

	// 1. Load configuration
	cfg, err := configs.LoadConfig(*cfgFile)
	if err != nil {
		log.Printf("Failed to load config file: %v. Using defaults/env/flags.", err)
		cfg = &configs.Config{
			NodeID:   "node-1",
			Mode:     "ap",
			HTTPAddr: ":8500",
			RaftAddr: "127.0.0.1:7000",
			DataDir:  "./data",
		}
	}

	// Override with CLI flags if provided
	if *httpAddr != "" {
		cfg.HTTPAddr = *httpAddr
	}
	if *raftAddr != "" {
		cfg.RaftAddr = *raftAddr
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *nodeID != "" {
		cfg.NodeID = *nodeID
	}
	if *mode != "" {
		cfg.Mode = *mode
	}
	if *joinAddr != "" {
		cfg.Join = *joinAddr
	}
	if *seedsFlag != "" {
		cfg.Seeds = strings.Split(*seedsFlag, ",")
	}

	cfg.Mode = strings.ToLower(cfg.Mode)
	if cfg.Mode != "ap" && cfg.Mode != "cp" {
		cfg.Mode = "ap"
	}

	log.SetFlags(log.LstdFlags | log.Lshortfile)
	log.Println("========================================")
	log.Println("    Eden Go Registry")
	log.Println("========================================")
	log.Printf("  Node ID   : %s", cfg.NodeID)
	log.Printf("  Mode      : %s", strings.ToUpper(cfg.Mode))
	log.Printf("  HTTP Addr : %s", cfg.HTTPAddr)
	log.Printf("  Data Dir  : %s", cfg.DataDir)

	// 2. Create Registry Store
	registry := store.NewRegistry(cfg.DataDir)

	// Seed built-in users from config
	var builtInUsers []model.User
	for _, uc := range cfg.Auth.Users {
		// Frontend will send SHA256 of the password
		sha256Pwd := utils.HashSHA256(uc.Password)
		// We store it as bcrypt(SHA256(password))
		hashed, err := bcrypt.GenerateFromPassword([]byte(sha256Pwd), bcrypt.DefaultCost)
		finalPwd := sha256Pwd
		if err == nil {
			finalPwd = string(hashed)
		}
		
		builtInUsers = append(builtInUsers, model.User{
			Username: uc.Username,
			Password: finalPwd,
			Nickname: uc.Nickname,
			Remark:   uc.Remark,
			Role:     uc.Role,
		})
	}
	registry.SeedBuiltInUsers(builtInUsers)

	// 3. Setup Consistency Mode (Initialize BOTH for online switching)
	var cpNode *raftpkg.Node
	var apNode *ap.Node

	// Always setup CP (Raft)
	log.Printf("  Raft Addr : %s", cfg.RaftAddr)
	log.Printf("  Bootstrap : %v", *bootstrap)
	raftCfg := raftpkg.Config{
		NodeID:    cfg.NodeID,
		BindAddr:  cfg.RaftAddr,
		DataDir:   cfg.DataDir,
		Bootstrap: *bootstrap,
	}
	cpNode, err = raftpkg.NewNode(raftCfg, registry)
	if err != nil {
		log.Fatalf("Failed to start Raft node: %v", err)
	}

	if cfg.Join != "" {
		go func() {
			time.Sleep(3 * time.Second)
			joinURL := fmt.Sprintf("%s/v1/cluster/join", cfg.Join)
			body := fmt.Sprintf(`{"node_id":"%s","address":"%s"}`, cfg.NodeID, cfg.RaftAddr)
			resp, err := http.Post(joinURL, "application/json", strings.NewReader(body))
			if err != nil {
				log.Printf("Failed to join CP cluster: %v", err)
				return
			}
			resp.Body.Close()
			log.Printf("Joined CP cluster via %s", cfg.Join)
		}()
	}

	// Always setup AP (Gossip/HTTP)
	log.Printf("  AP Seeds  : %v", cfg.Seeds)
	apNode = ap.NewNode(cfg, registry)

	// Set initial mode from config if metadata doesn't have it
	if registry.GetMode() == "" {
		registry.SetMode(cfg.Mode)
	}
	log.Printf("  Active Mode: %s", strings.ToUpper(registry.GetMode()))

	// 4. Start Health Checker
	checker := health.NewChecker(registry, *ttl, 10*time.Second)
	checker.Start()

	// 5. Start HTTP API
	h := handler.NewHandler(cfg, registry, cpNode, apNode)
	httpServer := &http.Server{
		Addr:    cfg.HTTPAddr,
		Handler: h,
	}

	go func() {
		log.Printf("HTTP API server listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server error: %v", err)
		}
	}()

	// 6. Wait for Shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	log.Printf("Received signal %v, shutting down...", sig)

	checker.Stop()
	if cfg.Mode == "cp" && cpNode != nil {
		if err := cpNode.Raft.Shutdown().Error(); err != nil {
			log.Printf("Raft shutdown error: %v", err)
		}
	}
	log.Println("Eden Go Registry stopped.")
}
