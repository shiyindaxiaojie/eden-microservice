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

	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"

	logger "github.com/shiyindaxiaojie/eden-go-logger"
	pb_cluster "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	pb_reg "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/catalog"
	platformcluster "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/ap"
	cp "github.com/shiyindaxiaojie/eden-go-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/pkg/crypto"
	"github.com/shiyindaxiaojie/eden-go-registry/internal/settings"
	grpcapi "github.com/shiyindaxiaojie/eden-go-registry/internal/transport/grpc"
	httpapi "github.com/shiyindaxiaojie/eden-go-registry/internal/transport/http"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	var (
		cfgFile  = flag.String("config", "configs/config.yaml", "Path to configuration file")
		dataDir  = flag.String("data-dir", "", "Override data directory")
		nodeID   = flag.String("node-id", "", "Override node ID")
		httpAddr = flag.String("http-addr", "", "Override HTTP listen address")
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

	// 2. Create runtime state
	runtimeState := platformcluster.NewRuntimeState(cfg.DataDir)
	if !runtimeState.HasAPIKeyAuthSetting() {
		runtimeState.SetAPIKeyAuthEnabled(cfg.Auth.APIKey.Enabled)
	}
	cfg.Auth.APIKey.Enabled = runtimeState.GetAPIKeyAuthEnabled()

	// Initialize Logger from config
	persistedLevel := runtimeState.GetLogLevel()
	if persistedLevel != "" {
		cfg.Log.Level = persistedLevel
	}
	applyLogRetentionDays(&cfg.Log, runtimeState.GetLogRetentionDays())
	logger.Init(config.ToLoggerConfiguration(cfg.Log))
	// (Logs moved and combined below after port resolution)

	// Seed built-in users from config
	var builtInUsers []auth.User
	for _, uc := range cfg.Auth.Users {
		// Frontend will send SHA256 of the password
		sha256Pwd := crypto.HashSHA256(uc.Password)
		// We store it as bcrypt(SHA256(password))
		hashed, err := bcrypt.GenerateFromPassword([]byte(sha256Pwd), bcrypt.DefaultCost)
		finalPwd := sha256Pwd
		if err == nil {
			finalPwd = string(hashed)
		}

		builtInUsers = append(builtInUsers, auth.User{
			Username: uc.Username,
			Password: finalPwd,
			Nickname: uc.Nickname,
			Remark:   uc.Remark,
			Role:     uc.Role,
		})
	}
	runtimeState.Auth.SeedBuiltInUsers(builtInUsers)

	// 3. Setup Consistency Mode (Initialize BOTH for online switching)
	var cpNode *cp.Node
	var apNode *ap.Node

	// Helper to resolve free port in range
	resolvePortRange := func(addr string, start, end int, network string) string {
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			if strings.HasPrefix(addr, ":") {
				host = "127.0.0.1"
				port = addr[1:]
			} else {
				host = "127.0.0.1"
				port = "0"
			}
		}
		if host == "" || host == "0.0.0.0" || host == "[::]" {
			host = "127.0.0.1"
		}

		if port == "0" || port == "" {
			for p := start; p <= end; p++ {
				testAddr := fmt.Sprintf("%s:%d", host, p)
				if network == "udp" {
					if l, err := net.ListenPacket("udp", testAddr); err == nil {
						l.Close()
						return testAddr
					}
				} else {
					if l, err := net.Listen("tcp", testAddr); err == nil {
						l.Close()
						return testAddr
					}
				}
			}
			// Fallback to OS-assigned port if range is exhausted
			if network == "udp" {
				if l, err := net.ListenPacket("udp", host+":0"); err == nil {
					defer l.Close()
					return l.LocalAddr().String()
				}
			} else {
				if l, err := net.Listen("tcp", host+":0"); err == nil {
					defer l.Close()
					return l.Addr().String()
				}
			}
		}
		return fmt.Sprintf("%s:%s", host, port)
	}

	// Resolve Ports within preferred ranges
	cfg.RaftAddr = resolvePortRange(cfg.RaftAddr, 7000, 7999, "tcp")
	cfg.GRPCAddr = resolvePortRange(cfg.GRPCAddr, 9000, 9999, "tcp")
	cfg.QUICAddr = resolvePortRange(cfg.QUICAddr, 10000, 10999, "udp")

	// Always setup CP (Raft)
	raftCfg := cp.Config{
		NodeID:    cfg.NodeID,
		BindAddr:  cfg.RaftAddr,
		DataDir:   cfg.DataDir,
		Bootstrap: cfg.Bootstrap,
	}
	cpNode, err = cp.NewNode(raftCfg, runtimeState)
	if err != nil {
		logger.Fatal("Failed to start Raft node: %v", err)
	}

	logger.Info("========================================")
	logger.Info("    Eden Go Registry")
	logger.Info("========================================")
	logger.Info("  Node ID   : %s", cfg.NodeID)
	logger.Info("  Mode      : %s", strings.ToUpper(runtimeState.GetMode()))
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
	apNode = ap.NewNode(cfg, runtimeState)
	// If registry already has seeds, sync them
	if len(runtimeState.GetSeeds()) == 0 && len(cfg.Seeds) > 0 {
		runtimeState.SetSeeds(cfg.Seeds)
	}
	apNode.SyncSeeds()

	// Set initial mode from config if metadata doesn't have it
	if runtimeState.GetMode() == "" {
		runtimeState.SetMode(cfg.Mode)
	}
	logger.Info("  Active Mode: %s", strings.ToUpper(runtimeState.GetMode()))

	// 4. Start Health Checker
	// Default TTL 30s, check every 10s if not specified in future config
	checker := catalog.NewChecker(runtimeState.Catalog, runtimeState.Settings, 30*time.Second, 10*time.Second)
	checker.Start()

	// 5. Setup Specialized Services
	catSvc := catalog.NewRegistry(runtimeState.Catalog, runtimeState.Settings, cpNode, apNode)
	authSvc := auth.NewAuthenticator(runtimeState.Auth)
	setSvc := settings.NewController(runtimeState.Settings, runtimeState.Auth, cpNode, apNode, runtimeState.Catalog.Events)
	clsSvc := platformcluster.NewMembership(runtimeState.Catalog, cpNode)

	// 6. Start HTTP API
	h := httpapi.NewHandler(cfg, catSvc, authSvc, setSvc, clsSvc)

	// Start gRPC API
	grpcServer := grpc.NewServer()
	regServer := grpcapi.NewRegistryServer(cfg, catSvc, setSvc, clsSvc)
	clusterServer := grpcapi.NewClusterServer(runtimeState, apNode)
	pb_reg.RegisterRegistryServiceServer(grpcServer, regServer)
	pb_cluster.RegisterClusterServiceServer(grpcServer, clusterServer)
	reflection.Register(grpcServer)

	go func() {
		lis, err := net.Listen("tcp", cfg.GRPCAddr)
		if err != nil {
			logger.Fatal("Failed to listen for gRPC: %v", err)
		}
		logger.Info("gRPC server listening on %s", lis.Addr().String())
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
		qlis, err := grpcapi.NewQUICListener(cfg.QUICAddr, tlsConf)
		if err != nil {
			logger.Error("Failed to listen for QUIC: %v", err)
			return
		}
		logger.Info("gRPC over QUIC server listening on %s", qlis.Addr().String())
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

func applyLogRetentionDays(logCfg *config.LogConfig, days int) {
	if logCfg == nil || days <= 0 {
		return
	}

	retention := fmt.Sprintf("%dd", days)
	if logCfg.Rollover == nil {
		logCfg.Rollover = &config.RolloverConfig{}
	}
	logCfg.Rollover.Retention = retention

	for i := range logCfg.Appenders {
		if logCfg.Appenders[i].Type == "" {
			continue
		}
		if logCfg.Appenders[i].Rollover == nil {
			logCfg.Appenders[i].Rollover = &config.RolloverConfig{}
		}
		logCfg.Appenders[i].Rollover.Retention = retention
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
