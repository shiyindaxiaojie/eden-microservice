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

	nacosgrpc "github.com/nacos-group/nacos-sdk-go/v2/api/grpc"
	logger "github.com/shiyindaxiaojie/eden-go-logger"
	pb_cluster "github.com/shiyindaxiaojie/eden-registry/api/proto/cluster/v1"
	pb_reg "github.com/shiyindaxiaojie/eden-registry/api/proto/registry/v1"
	nacosadapter "github.com/shiyindaxiaojie/eden-registry/internal/adapter/nacos"
	"github.com/shiyindaxiaojie/eden-registry/internal/auth"
	"github.com/shiyindaxiaojie/eden-registry/internal/catalog"
	platformcluster "github.com/shiyindaxiaojie/eden-registry/internal/cluster"
	"github.com/shiyindaxiaojie/eden-registry/internal/cluster/ap"
	cp "github.com/shiyindaxiaojie/eden-registry/internal/cluster/cp"
	"github.com/shiyindaxiaojie/eden-registry/internal/config"
	"github.com/shiyindaxiaojie/eden-registry/internal/configcenter"
	"github.com/shiyindaxiaojie/eden-registry/internal/gateway"
	"github.com/shiyindaxiaojie/eden-registry/internal/pkg/crypto"
	"github.com/shiyindaxiaojie/eden-registry/internal/settings"
	httpapi "github.com/shiyindaxiaojie/eden-registry/internal/transport/http"
	quictransport "github.com/shiyindaxiaojie/eden-registry/internal/transport/quic"
	rpcapi "github.com/shiyindaxiaojie/eden-registry/internal/transport/rpc"
	"golang.org/x/crypto/bcrypt"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"
)

func main() {
	var (
		cfgFile         = flag.String("config", "config/config.yaml", "Path to configuration file")
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
			Server: config.ServerConfig{
				HTTP: ":8500",
				GRPC: "auto",
				QUIC: "auto",
				Raft: "auto",
			},
			DataDir: "./data",
		}
	}

	// Override with CLI flags if provided
	if *httpAddr != "" {
		cfg.Server.HTTP = *httpAddr
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
		cfg.Server.GRPC = strings.ToLower(strings.TrimSpace(*grpcFlag))
	}
	if *quicFlag != "" {
		cfg.Server.QUIC = strings.ToLower(strings.TrimSpace(*quicFlag))
	}
	if *raftFlag != "" {
		cfg.Server.Raft = strings.ToLower(strings.TrimSpace(*raftFlag))
	}
	cfg.Mode, cfg.Consistency = normalizeRuntimeSelection(cfg.Mode, cfg.Consistency)

	configService, err := configcenter.Open(cfg.DataDir)
	if err != nil {
		logger.Fatal("Failed to open config center storage: %v", err)
	}
	defer func() {
		if err := configService.Close(); err != nil {
			logger.Error("Failed to close config center storage: %v", err)
		}
	}()

	gatewayService, err := gateway.Open(cfg.DataDir)
	if err != nil {
		logger.Fatal("Failed to open gateway storage: %v", err)
	}
	defer func() {
		if err := gatewayService.Close(); err != nil {
			logger.Error("Failed to close gateway storage: %v", err)
		}
	}()

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

	// Apply storage modes from persisted runtime state when available.
	eventStorageMode := cfg.Storage.EventStorageMode
	if runtimeState.Settings.LoadedFromDisk() && runtimeState.GetEventStorageMode() != "" {
		eventStorageMode = runtimeState.GetEventStorageMode()
	}
	metricsStorageMode := cfg.Storage.MetricsStorageMode
	if runtimeState.Settings.LoadedFromDisk() && runtimeState.GetMetricsStorageMode() != "" {
		metricsStorageMode = runtimeState.GetMetricsStorageMode()
	}
	registryFlushMode := cfg.Storage.RegistryFlushMode
	if runtimeState.Settings.LoadedFromDisk() && runtimeState.GetRegistryFlushMode() != "" {
		registryFlushMode = runtimeState.GetRegistryFlushMode()
	}
	registryFlushIntervalMS := cfg.Storage.RegistryFlushIntervalMS
	if runtimeState.Settings.LoadedFromDisk() && runtimeState.GetRegistryFlushIntervalMS() > 0 {
		registryFlushIntervalMS = runtimeState.GetRegistryFlushIntervalMS()
	}
	runtimeState.SetRegistryFlushMode(registryFlushMode)
	runtimeState.SetRegistryFlushIntervalMS(registryFlushIntervalMS)
	runtimeState.SetEventStorageMode(eventStorageMode)
	runtimeState.SetMetricsStorageMode(metricsStorageMode)

	runtimeState.StartMetricsRecorder()

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

	isAutoPort := func(addr string) bool {
		trimmed := strings.ToLower(strings.TrimSpace(addr))
		if trimmed == "" || trimmed == ":0" || trimmed == "auto" {
			return true
		}
		_, port, err := net.SplitHostPort(trimmed)
		return err == nil && port == "0"
	}

	raftEnabled := cfg.RaftEnabled(cfg.Mode, cfg.Consistency)
	grpcEnabled := cfg.GRPCEnabled(cfg.Mode)
	quicEnabled := cfg.QUICEnabled(cfg.Mode)
	if raftEnabled && cfg.Mode == "cluster" && cfg.Consistency == "cp" && len(runtimeState.GetSeeds()) == 0 && !cfg.Bootstrap {
		cfg.Bootstrap = true
		logger.Warn("[Raft] no seeds configured in CP mode; enabling single-node bootstrap automatically")
	}

	if raftEnabled {
		cfg.Server.Raft = resolvePortRange(cfg.Server.Raft, 7000, 7999, "tcp")
		raftCfg := cp.Config{
			NodeID:    cfg.NodeID,
			BindAddr:  cfg.Server.Raft,
			DataDir:   cfg.DataDir,
			Bootstrap: cfg.Bootstrap,
		}
		cpNode, err = cp.NewNode(raftCfg, runtimeState)
		if err != nil {
			logger.Fatal("Failed to start Raft node: %v", err)
		}
	} else {
		cfg.Server.Raft = ""
	}

	if grpcEnabled {
		if isAutoPort(cfg.Server.GRPC) {
			cfg.Server.GRPC = resolvePortRange(cfg.Server.GRPC, 9000, 9999, "tcp")
		}
	} else {
		cfg.Server.GRPC = ""
	}
	if quicEnabled {
		cfg.Server.QUIC = resolvePortRange(cfg.Server.QUIC, 10000, 10999, "udp")
	} else {
		cfg.Server.QUIC = ""
	}

	logger.Info("========================================")
	logger.Info("    Registry Server")
	logger.Info("========================================")
	logger.Info("  Node ID   : %s", cfg.NodeID)
	logger.Info("  Mode      : %s", strings.ToUpper(cfg.Mode))
	logger.Info("  Consistency: %s", strings.ToUpper(cfg.Consistency))
	logger.Info("  HTTP Addr : %s", displayListenAddr(cfg.Server.HTTP))
	logger.Info("  GRPC Addr : %s", displayListenAddr(cfg.Server.GRPC))
	logger.Info("  QUIC Addr : %s", displayListenAddr(cfg.Server.QUIC))
	logger.Info("  Raft Addr : %s", displayListenAddr(cfg.Server.Raft))
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
	var clusterReplicator rpcapi.Replicator
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
	setSvc := settings.NewController(runtimeState.Settings, runtimeState.Auth, settingsCPNode, settingsAPNode, runtimeState.Catalog.Events, runtimeState.Catalog.Metrics, runtimeState, settings.StartupState{
		Mode:        cfg.Mode,
		Consistency: cfg.Consistency,
		GRPCEnabled: grpcEnabled,
		QUICEnabled: quicEnabled,
		RaftEnabled: raftEnabled,
	})
	clsSvc := platformcluster.NewMembership(runtimeState.Catalog, membershipCPNode)
	gatewayRuntime, err := gateway.NewRuntime(gatewayService, catSvc, gatewayRuntimeConfig(cfg))
	if err != nil {
		logger.Fatal("Failed to initialize gateway runtime: %v", err)
	}
	defer gatewayRuntime.Close()

	// 6. Start HTTP API
	h := httpapi.NewHandler(cfg, runtimeState.Catalog, catSvc, configService, authSvc, setSvc, clsSvc)
	h.SetGateway(gatewayService, gatewayRuntime)
	if gatewayListenerEnabled(cfg) && gatewayListenerAddressesConflict(cfg.Server.HTTP, cfg.Gateway.HTTP) {
		logger.Fatal("Gateway HTTP listener must use a non-overlapping address from the control-plane HTTP listener")
	}

	var grpcServer *grpc.Server
	if grpcEnabled || quicEnabled {
		grpcServer = grpc.NewServer()
		regServer := rpcapi.NewRegistryServer(cfg, catSvc, setSvc, clsSvc)
		clusterServer := rpcapi.NewClusterServer(runtimeState, clusterReplicator)
		nacosServer := nacosadapter.NewNacosNamingServer(cfg, catSvc)
		pb_reg.RegisterRegistryServiceServer(grpcServer, regServer)
		pb_cluster.RegisterClusterServiceServer(grpcServer, clusterServer)
		nacosgrpc.RegisterRequestServer(grpcServer, nacosServer)
		nacosgrpc.RegisterBiRequestStreamServer(grpcServer, nacosServer)
		reflection.Register(grpcServer)
	}

	if grpcEnabled && grpcServer != nil {
		go func() {
			lis, err := net.Listen("tcp", cfg.Server.GRPC)
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
			qlis, err := quictransport.NewListener(cfg.Server.QUIC, tlsConf)
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
	logger.Info("HTTP API server listening on %s", cfg.Server.HTTP)
	go func() {
		if err := http.ListenAndServe(cfg.Server.HTTP, h); err != nil {
			logger.Fatal("HTTP server error: %v", err)
		}
	}()

	if gatewayListenerEnabled(cfg) {
		logger.Info("Gateway data-plane server listening on %s", cfg.Gateway.HTTP)
		go func() {
			if err := http.ListenAndServe(cfg.Gateway.HTTP, gatewayRuntime); err != nil {
				logger.Fatal("Gateway data-plane server error: %v", err)
			}
		}()
	}

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
	logger.Info("Registry server stopped.")
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

func displayListenAddr(addr string) string {
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

	if host == "" || host == "0.0.0.0" || host == "::" || host == "[::]" {
		return net.JoinHostPort("127.0.0.1", port)
	}
	return addr
}

func gatewayListenerEnabled(cfg *config.Config) bool {
	return cfg != nil && cfg.Gateway.Enabled && strings.TrimSpace(cfg.Gateway.HTTP) != ""
}

func gatewayListenerAddressesConflict(controlAddress, gatewayAddress string) bool {
	control, controlErr := net.ResolveTCPAddr("tcp", strings.TrimSpace(controlAddress))
	gateway, gatewayErr := net.ResolveTCPAddr("tcp", strings.TrimSpace(gatewayAddress))
	if controlErr != nil || gatewayErr != nil {
		return strings.EqualFold(strings.TrimSpace(controlAddress), strings.TrimSpace(gatewayAddress))
	}
	if control.Port != gateway.Port {
		return false
	}
	return tcpAddressesOverlap(control.IP, gateway.IP)
}

func tcpAddressesOverlap(left, right net.IP) bool {
	if len(left) == 0 || len(right) == 0 || left.IsUnspecified() || right.IsUnspecified() {
		return true
	}
	return left.Equal(right)
}

func gatewayRuntimeConfig(cfg *config.Config) gateway.RuntimeConfig {
	if cfg == nil {
		return gateway.RuntimeConfig{}
	}
	return gateway.RuntimeConfig{TrustedProxyCIDRs: cfg.Gateway.TrustedProxyCIDRs}
}

func generateSelfSignedCert() (tls.Certificate, error) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return tls.Certificate{}, err
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Registry Server"},
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
