package main

import (
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"

	nacosgrpc "github.com/nacos-group/nacos-sdk-go/v2/api/grpc"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
	pb_cluster "github.com/shiyindaxiaojie/eden-go-registry/api/proto/cluster/v1"
	pb_reg "github.com/shiyindaxiaojie/eden-go-registry/api/proto/registry/v1"
	nacosadapter "github.com/shiyindaxiaojie/eden-go-registry/internal/adapter/nacos"
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
		cfgFile         = flag.String("config", "configs/config.yaml", "Path to configuration file")
		dataDir         = flag.String("data-dir", "", "Override data directory")
		nodeID          = flag.String("node-id", "", "Override node ID")
		httpAddr        = flag.String("http-addr", "", "Override HTTP listen address")
		modeFlag        = flag.String("mode", "", "Override runtime mode: standalone or cluster")
		consistencyFlag = flag.String("consistency", "", "Override consistency: ap or cp")
		grpcFlag        = flag.String("grpc", "", "Override gRPC transport: auto, on, or off")
		quicFlag        = flag.String("quic", "", "Override QUIC transport: auto, on, or off")
		raftFlag        = flag.String("raft", "", "Override Raft transport: auto, on, or off")
	)
	flag.Parse()

	// 1. Load configuration
	cfg, err := config.LoadConfig(*cfgFile)
	if err != nil {
		logger.Warn("Failed to load config file: %v. Using defaults.", err)
		cfg = &config.Config{
			NodeID:      "node-1",
			Mode:        "standalone",
			Consistency: "ap",
			HTTPAddr:    ":8500",
			GRPCAddr:    ":0",
			DataDir:     "./data",
			Transport: config.TransportConfig{
				GRPC: "auto",
				QUIC: "auto",
				Raft: "auto",
			},
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
	if *modeFlag != "" {
		cfg.Mode = strings.ToLower(strings.TrimSpace(*modeFlag))
	}
	if *consistencyFlag != "" {
		cfg.Consistency = strings.ToLower(strings.TrimSpace(*consistencyFlag))
	}
	if *grpcFlag != "" {
		cfg.Transport.GRPC = strings.ToLower(strings.TrimSpace(*grpcFlag))
	}
	if *quicFlag != "" {
		cfg.Transport.QUIC = strings.ToLower(strings.TrimSpace(*quicFlag))
	}
	if *raftFlag != "" {
		cfg.Transport.Raft = strings.ToLower(strings.TrimSpace(*raftFlag))
	}
	cfg.Mode, cfg.Consistency = normalizeRuntimeSelection(cfg.Mode, cfg.Consistency)
	cfg.Transport.GRPC = normalizeTransportSetting(cfg.Transport.GRPC)
	cfg.Transport.QUIC = normalizeTransportSetting(cfg.Transport.QUIC)
	cfg.Transport.Raft = normalizeTransportSetting(cfg.Transport.Raft)

	// 2. Create runtime state
	runtimeState := platformcluster.NewRuntimeState(cfg.DataDir)
	bootMode := runtimeState.GetEnvironment()
	bootConsistency := runtimeState.GetMode()
	if !runtimeState.Settings.LoadedFromDisk() {
		bootMode = cfg.Mode
		bootConsistency = cfg.Consistency
	}
	if *modeFlag != "" {
		bootMode = cfg.Mode
	}
	if *consistencyFlag != "" {
		bootConsistency = cfg.Consistency
	}
	bootMode, bootConsistency = normalizeRuntimeSelection(bootMode, bootConsistency)
	runtimeState.SetEnvironment(bootMode)
	runtimeState.SetMode(bootConsistency)
	cfg.Mode = bootMode
	cfg.Consistency = bootConsistency
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

	// 3. Setup runtime transports.
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

	getLocalIP := func() string {
		addrs, err := net.InterfaceAddrs()
		if err != nil {
			return "127.0.0.1"
		}
		for _, address := range addrs {
			if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
				if ipnet.IP.To4() != nil {
					return ipnet.IP.String()
				}
			}
		}
		return "127.0.0.1"
	}

	resolveLogAddr := func(addr string) string {
		if addr == "" {
			return ""
		}
		host, port, err := net.SplitHostPort(addr)
		if err != nil {
			if strings.HasPrefix(addr, ":") {
				return getLocalIP() + addr
			}
			return addr
		}
		if host == "" || host == "0.0.0.0" || host == "[::]" {
			return getLocalIP() + ":" + port
		}
		return addr
	}

	isAutoPort := func(addr string) bool {
		trimmed := strings.TrimSpace(addr)
		if trimmed == "" || trimmed == ":0" {
			return true
		}
		_, port, err := net.SplitHostPort(trimmed)
		return err == nil && port == "0"
	}

	resolveCompanionAddr := func(baseAddr string, offset int, network string) string {
		host, port, err := net.SplitHostPort(baseAddr)
		if err != nil {
			if strings.HasPrefix(baseAddr, ":") {
				host = "127.0.0.1"
				port = baseAddr[1:]
			} else {
				return ""
			}
		}
		if host == "" || host == "0.0.0.0" || host == "[::]" {
			host = "127.0.0.1"
		}

		basePort, err := strconv.Atoi(port)
		if err != nil || basePort <= 0 {
			return ""
		}

		candidate := fmt.Sprintf("%s:%d", host, basePort+offset)
		if network == "udp" {
			lis, err := net.ListenPacket("udp", candidate)
			if err != nil {
				return ""
			}
			lis.Close()
			return candidate
		}

		lis, err := net.Listen("tcp", candidate)
		if err != nil {
			return ""
		}
		lis.Close()
		return candidate
	}

	raftEnabled := cfg.RaftEnabled(cfg.Mode, cfg.Consistency)
	grpcEnabled := cfg.GRPCEnabled(cfg.Mode)
	quicEnabled := cfg.QUICEnabled(cfg.Mode)
	if raftEnabled && cfg.Mode == "cluster" && cfg.Consistency == "cp" && len(runtimeState.GetSeeds()) == 0 && !cfg.Bootstrap {
		cfg.Bootstrap = true
		logger.Warn("[Raft] no seeds configured in CP mode; enabling single-node bootstrap automatically")
	}

	if raftEnabled {
		cfg.RaftAddr = resolvePortRange(cfg.RaftAddr, 7000, 7999, "tcp")
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
	} else {
		cfg.RaftAddr = ""
	}

	if grpcEnabled {
		if isAutoPort(cfg.GRPCAddr) {
			if companion := resolveCompanionAddr(cfg.HTTPAddr, 1000, "tcp"); companion != "" {
				cfg.GRPCAddr = companion
			} else {
				cfg.GRPCAddr = resolvePortRange(cfg.GRPCAddr, 9000, 9999, "tcp")
			}
		}
	} else {
		cfg.GRPCAddr = ""
	}
	if quicEnabled {
		cfg.QUICAddr = resolvePortRange(cfg.QUICAddr, 10000, 10999, "udp")
	} else {
		cfg.QUICAddr = ""
	}

	logger.Info("========================================")
	logger.Info("    Eden Go Registry")
	logger.Info("========================================")
	logger.Info("  Node ID   : %s", cfg.NodeID)
	logger.Info("  Mode      : %s", strings.ToUpper(cfg.Mode))
	logger.Info("  Consistency: %s", strings.ToUpper(cfg.Consistency))
	logger.Info("  HTTP Addr : %s", resolveLogAddr(cfg.HTTPAddr))
	logger.Info("  GRPC Addr : %s", resolveLogAddr(cfg.GRPCAddr))
	logger.Info("  QUIC Addr : %s", resolveLogAddr(cfg.QUICAddr))
	logger.Info("  Raft Addr : %s", resolveLogAddr(cfg.RaftAddr))
	logger.Info("  Data Dir  : %s", cfg.DataDir)
	logger.Info("========================================")

	if cfg.Mode == "cluster" && grpcEnabled {
		logger.Info("  AP Seeds  : %v", runtimeState.GetSeeds())
		apNode = ap.NewNode(cfg, runtimeState)
		apNode.SyncSeeds()
	}
	logger.Info("  Active Topology: %s", strings.ToUpper(runtimeState.GetEnvironment()))
	logger.Info("  Active Consistency: %s", strings.ToUpper(runtimeState.GetMode()))

	// 4. Start Health Checker
	// Default TTL 30s, check every 10s if not specified in future config
	checker := catalog.NewChecker(runtimeState.Catalog, runtimeState.Settings, 30*time.Second, 10*time.Second)
	checker.Start()

	// 5. Setup Specialized Services
	var catalogCPNode catalog.CPNode
	var catalogAPNode catalog.APNode
	var settingsCPNode settings.CPNode
	var settingsAPNode settings.APNode
	var membershipCPNode platformcluster.ConsensusNode
	var clusterReplicator grpcapi.Replicator
	if cpNode != nil {
		catalogCPNode = cpNode
		settingsCPNode = cpNode
		membershipCPNode = cpNode
	}
	if apNode != nil {
		catalogAPNode = apNode
		settingsAPNode = apNode
		clusterReplicator = apNode
	}

	catSvc := catalog.NewRegistry(runtimeState.Catalog, runtimeState.Settings, catalogCPNode, catalogAPNode)
	authSvc := auth.NewAuthenticator(runtimeState.Auth)
	setSvc := settings.NewController(runtimeState.Settings, runtimeState.Auth, settingsCPNode, settingsAPNode, runtimeState.Catalog.Events, settings.StartupState{
		Mode:        cfg.Mode,
		Consistency: cfg.Consistency,
		GRPCEnabled: grpcEnabled,
		QUICEnabled: quicEnabled,
		RaftEnabled: raftEnabled,
	})
	clsSvc := platformcluster.NewMembership(runtimeState.Catalog, membershipCPNode)

	// 6. Start HTTP API
	h := httpapi.NewHandler(cfg, runtimeState.Catalog, catSvc, authSvc, setSvc, clsSvc)

	var grpcServer *grpc.Server
	if grpcEnabled || quicEnabled {
		grpcServer = grpc.NewServer()
		regServer := grpcapi.NewRegistryServer(cfg, catSvc, setSvc, clsSvc)
		clusterServer := grpcapi.NewClusterServer(runtimeState, clusterReplicator)
		nacosServer := nacosadapter.NewNacosNamingServer(cfg, catSvc)
		pb_reg.RegisterRegistryServiceServer(grpcServer, regServer)
		pb_cluster.RegisterClusterServiceServer(grpcServer, clusterServer)
		nacosgrpc.RegisterRequestServer(grpcServer, nacosServer)
		nacosgrpc.RegisterBiRequestStreamServer(grpcServer, nacosServer)
		reflection.Register(grpcServer)
	}

	if grpcEnabled && grpcServer != nil {
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
	}

	if quicEnabled && grpcServer != nil {
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
	}

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
	if grpcServer != nil {
		grpcServer.GracefulStop()
	}
	if raftEnabled && cpNode != nil {
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

func normalizeModeSetting(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "cluster") {
		return "cluster"
	}
	return "standalone"
}

func normalizeConsistencySetting(mode string) string {
	if strings.EqualFold(strings.TrimSpace(mode), "cp") {
		return "cp"
	}
	return "ap"
}

func normalizeRuntimeSelection(mode, consistency string) (string, string) {
	normalizedMode := normalizeModeSetting(mode)
	if normalizedMode != "cluster" {
		return normalizedMode, "ap"
	}
	return normalizedMode, normalizeConsistencySetting(consistency)
}

func normalizeTransportSetting(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "on":
		return "on"
	case "off":
		return "off"
	default:
		return "auto"
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
