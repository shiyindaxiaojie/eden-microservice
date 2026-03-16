package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/shiyindaxiaojie/eden-go-logger"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/ap"
	cp "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/handler"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/health"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/pkg/crypto"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
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
	cfg, err := config.LoadConfig(*cfgFile)
	if err != nil {
		logger.Warn("Failed to load config file: %v. Using defaults/env/flags.", err)
		cfg = &config.Config{
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

	// Initialize Logger from config
	logger.Init(convertToLoggerConfig(cfg.Log))
	logger.Info("========================================")
	logger.Info("    Eden Go Registry")
	logger.Info("========================================")
	logger.Info("  Node ID   : %s", cfg.NodeID)
	logger.Info("  Mode      : %s", strings.ToUpper(cfg.Mode))
	logger.Info("  HTTP Addr : %s", cfg.HTTPAddr)
	logger.Info("  Data Dir  : %s", cfg.DataDir)

	// 2. Create Registry Store
	registry := store.NewRegistry(cfg.DataDir)

	// Seed built-in users from config
	var builtInUsers []model.User
	for _, uc := range cfg.Auth.Users {
		// Frontend will send SHA256 of the password
		sha256Pwd := crypto.HashSHA256(uc.Password)
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
	var cpNode *cp.Node
	var apNode *ap.Node

	// Always setup CP (Raft)
	logger.Info("  Raft Addr : %s", cfg.RaftAddr)
	logger.Info("  Bootstrap : %v", *bootstrap)
	raftCfg := cp.Config{
		NodeID:    cfg.NodeID,
		BindAddr:  cfg.RaftAddr,
		DataDir:   cfg.DataDir,
		Bootstrap: *bootstrap,
	}
	cpNode, err = cp.NewNode(raftCfg, registry)
	if err != nil {
		logger.Fatal("Failed to start Raft node: %v", err)
	}

	if cfg.Join != "" {
		go func() {
			time.Sleep(3 * time.Second)
			joinURL := fmt.Sprintf("%s/v1/cluster/join", cfg.Join)
			body := fmt.Sprintf(`{"node_id":"%s","address":"%s"}`, cfg.NodeID, cfg.RaftAddr)
			resp, err := http.Post(joinURL, "application/json", strings.NewReader(body))
			if err != nil {
				logger.Error("Failed to join CP cluster: %v", err)
				return
			}
			resp.Body.Close()
			logger.Info("Joined CP cluster via %s", cfg.Join)
		}()
	}

	// Always setup AP (Gossip/HTTP)
	logger.Info("  AP Seeds  : %v", cfg.Seeds)
	apNode = ap.NewNode(cfg, registry)
	// If registry already has seeds, sync them
	if len(registry.GetSeeds()) == 0 && len(cfg.Seeds) > 0 {
		registry.SetSeeds(cfg.Seeds)
	}
	apNode.SyncSeeds()

	// Set initial mode from config if metadata doesn't have it
	if registry.GetMode() == "" {
		registry.SetMode(cfg.Mode)
	}
	logger.Info("  Active Mode: %s", strings.ToUpper(registry.GetMode()))

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
		logger.Info("HTTP API server listening on %s", cfg.HTTPAddr)
		if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal("HTTP server error: %v", err)
		}
	}()

	// 6. Wait for Shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	sig := <-sigCh
	logger.Info("Received signal %v, shutting down...", sig)

	checker.Stop()
	if cfg.Mode == "cp" && cpNode != nil {
		if err := cpNode.Raft.Shutdown().Error(); err != nil {
			logger.Error("Raft shutdown error: %v", err)
		}
	}
	logger.Info("Eden Go Registry stopped.")
}

func convertToLoggerConfig(lc config.LogConfig) logger.Configuration {
	var appenders []logger.AppenderConfig
	for _, a := range lc.Appenders {
		var rollover *logger.RolloverConfig
		if a.Rollover != nil {
			rollover = &logger.RolloverConfig{
				MaxFile:   a.Rollover.MaxFile,
				Retention: a.Rollover.Retention,
			}
		}
		appenders = append(appenders, logger.AppenderConfig{
			Name:        a.Name,
			Type:        a.Type,
			Level:       a.Level,
			Pattern:     a.Pattern,
			FileName:    a.FileName,
			FilePattern: a.FilePattern,
			Filter:      a.Filter,
			Async:       a.Async,
			Rollover:    rollover,
		})
	}

	var policies *logger.PoliciesConfig
	if lc.Policies != nil {
		policies = &logger.PoliciesConfig{}
		if lc.Policies.CronTriggeringPolicy != nil {
			policies.CronTriggeringPolicy = &logger.CronPolicyConfig{
				Schedule: lc.Policies.CronTriggeringPolicy.Schedule,
			}
		}
		if lc.Policies.SizeBasedTriggeringPolicy != nil {
			policies.SizeBasedTriggeringPolicy = &logger.SizePolicyConfig{
				Size: lc.Policies.SizeBasedTriggeringPolicy.Size,
			}
		}
	}

	var rollover *logger.RolloverConfig
	if lc.Rollover != nil {
		rollover = &logger.RolloverConfig{
			MaxFile:   lc.Rollover.MaxFile,
			Retention: lc.Rollover.Retention,
		}
	}

	return logger.Configuration{
		Level:           lc.Level,
		Format:          lc.Format,
		Pattern:         lc.Pattern,
		Policies:        policies,
		Rollover:        rollover,
		IncludeLocation: lc.IncludeLocation,
		Appenders:       appenders,
	}
}
