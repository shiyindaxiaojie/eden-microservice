package main

import (
	"flag"
	"fmt"
	"net"
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
	egrpc "github.com/shiyindaxiaojie/eden-go-registry/internal/grpc"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/handler"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/health"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/model"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/pkg/crypto"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/service"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/store"
	pb_cluster "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	pb_reg "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
	"crypto/tls"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"crypto/rand"
	"math/big"
)

func main() {
	var (
		cfgFile   = flag.String("config", "configs/config.yaml", "Path to configuration file")
		dataDir   = flag.String("data-dir", "", "Override data directory")
		nodeID    = flag.String("node-id", "", "Override node ID")
		httpAddr  = flag.String("http-addr", "", "Override HTTP listen address")
	)
	flag.Parse()

	// 1. Load configuration
	cfg, err := config.LoadConfig(*cfgFile)
	if err != nil {
		logger.Warn("Failed to load config file: %v. Using defaults.", err)
		cfg = &config.Config{
			NodeID:   "node-1",
			Mode:     "ap",
			HTTPAddr: ":8500",
			GRPCAddr: ":9000",
			RaftAddr: "127.0.0.1:7000",
			DataDir:  "./data",
		}
	}

	// Override with CLI flags if provided
	if *httpAddr != "" {
		cfg.HTTPAddr = *httpAddr
	}
	if *dataDir != "" {
		cfg.DataDir = *dataDir
	}
	if *nodeID != "" {
		cfg.NodeID = *nodeID
	}

	cfg.Mode = strings.ToLower(cfg.Mode)
	if cfg.Mode != "ap" && cfg.Mode != "cp" {
		cfg.Mode = "ap"
	}

	// 2. Create Registry Store
	registry := store.NewRegistry(cfg.DataDir)

	// Initialize Logger from config
	persistedLevel := registry.GetLogLevel()
	if persistedLevel != "" {
		cfg.Log.Level = persistedLevel
	}
	logger.Init(convertToLoggerConfig(cfg.Log))
	// (Logs moved and combined below after port resolution)

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
	registry.Auth.SeedBuiltInUsers(builtInUsers)

	// 3. Setup Consistency Mode (Initialize BOTH for online switching)
	var cpNode *cp.Node
	var apNode *ap.Node

	// Resolve Raft Addr if empty or just a port
	if cfg.RaftAddr == "" || strings.HasPrefix(cfg.RaftAddr, ":") {
		bindAddr := cfg.RaftAddr
		if bindAddr == "" || bindAddr == ":0" {
			bindAddr = "127.0.0.1:0"
		} else if strings.HasPrefix(bindAddr, ":") {
			bindAddr = "127.0.0.1" + bindAddr
		}
		
		l, err := net.Listen("tcp", bindAddr)
		if err != nil {
			logger.Fatal("Failed to resolve Raft port: %v", err)
		}
		cfg.RaftAddr = l.Addr().String()
		l.Close()
	}

	// Always setup CP (Raft)
	logger.Info("  Raft Addr : %s", cfg.RaftAddr)
	raftCfg := cp.Config{
		NodeID:    cfg.NodeID,
		BindAddr:  cfg.RaftAddr,
		DataDir:   cfg.DataDir,
		Bootstrap: cfg.Bootstrap,
	}
	cpNode, err = cp.NewNode(raftCfg, registry)
	if err != nil {
		logger.Fatal("Failed to start Raft node: %v", err)
	}

	// 4. Final Port Resolution & Logging
	// Resolve gRPC and QUIC immediately for logging (not just inside goroutines)
	if cfg.GRPCAddr == "" || strings.HasSuffix(cfg.GRPCAddr, ":0") {
		addr := cfg.GRPCAddr
		if addr == "" { addr = ":0" }
		l, _ := net.Listen("tcp", addr)
		cfg.GRPCAddr = l.Addr().String()
		l.Close()
	}
	if cfg.QUICAddr == "" || strings.HasSuffix(cfg.QUICAddr, ":0") {
		// Use same port as gRPC if possible, or new one
		cfg.QUICAddr = cfg.GRPCAddr // Just a suggestion or let it be auto
	}

	logger.Info("========================================")
	logger.Info("    Eden Go Registry")
	logger.Info("========================================")
	logger.Info("  Node ID   : %s", cfg.NodeID)
	logger.Info("  Mode      : %s", strings.ToUpper(registry.GetMode()))
	logger.Info("  HTTP Addr : %s", cfg.HTTPAddr)
	logger.Info("  GRPC Addr : %s", cfg.GRPCAddr)
	logger.Info("  QUIC Addr : %s", cfg.QUICAddr)
	logger.Info("  Raft Addr : %s", cfg.RaftAddr)
	logger.Info("  Data Dir  : %s", cfg.DataDir)
	logger.Info("========================================")

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
	// Default TTL 30s, check every 10s if not specified in future config
	checker := health.NewChecker(registry, 30*time.Second, 10*time.Second)
	checker.Start()

	// 5. Setup Specialized Services
	catSvc := service.NewCatalogService(registry, cpNode, apNode)
	authSvc := service.NewAuthService(registry)
	setSvc := service.NewSettingsService(registry, cpNode, apNode)
	clsSvc := service.NewClusterService(registry, cpNode, apNode)

	// 6. Start HTTP API
	h := handler.NewHandler(cfg, catSvc, authSvc, setSvc, clsSvc)

	// Start gRPC API
	grpcServer := grpc.NewServer()
	regServer := egrpc.NewRegistryServer(catSvc)
	clusterServer := egrpc.NewClusterServer(registry, apNode)
	pb_reg.RegisterRegistryServiceServer(grpcServer, regServer)
	pb_cluster.RegisterClusterServiceServer(grpcServer, clusterServer)
	reflection.Register(grpcServer)

	go func() {
		addr := cfg.GRPCAddr
		if addr == "" {
			addr = ":0"
		}
		lis, err := net.Listen("tcp", addr)
		if err != nil {
			logger.Fatal("Failed to listen for gRPC: %v", err)
		}
		cfg.GRPCAddr = lis.Addr().String() // Update resolved addr
		logger.Info("gRPC server listening on %s", cfg.GRPCAddr)
		if err := grpcServer.Serve(lis); err != nil {
			logger.Error("gRPC server error: %v", err)
		}
	}()

	// Start QUIC gRPC server
	go func() {
		cert, err := generateSelfSignedCert()
		if err != nil {
			logger.Error("Failed to generate self-signed cert for QUIC: %v", err)
			return
		}
		tlsConf := &tls.Config{
			Certificates: []tls.Certificate{cert},
			NextProtos:   []string{"h3"},
		}
		addr := cfg.QUICAddr
		if addr == "" {
			addr = ":0"
		}
		qlis, err := egrpc.NewQUICListener(addr, tlsConf)
		if err != nil {
			logger.Error("Failed to listen for QUIC: %v", err)
			return
		}
		cfg.QUICAddr = qlis.Addr().String() // Update resolved addr
		logger.Info("gRPC over QUIC server listening on %s", cfg.QUICAddr)
		if err := grpcServer.Serve(qlis); err != nil {
			logger.Error("QUIC gRPC server error: %v", err)
		}
	}()

	// Start HTTP API
	logger.Info("HTTP API server listening on %s", cfg.HTTPAddr)
	go func() {
		if err := http.ListenAndServe(cfg.HTTPAddr, h); err != nil {
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

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Eden Go Registry"},
		},
		NotBefore: time.Now(),
		NotAfter:  time.Now().Add(time.Hour * 24 * 365),

		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	return tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  key,
	}, nil
}
