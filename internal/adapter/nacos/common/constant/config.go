// Package constant is a drop-in replacement for
// github.com/nacos-group/nacos-sdk-go/v2/common/constant.
// It provides the same types so existing Nacos code compiles without changes.
package constant

// ClientConfig holds Nacos client configuration.
type ClientConfig struct {
	NamespaceId         string
	TimeoutMs           uint64
	BeatInterval        int64
	NotLoadCacheAtStart bool
	CacheDir            string
	LogDir              string
	LogLevel            string
	Username            string
	Password            string
}

// ServerConfig holds Nacos server connection info.
type ServerConfig struct {
	IpAddr      string
	Port        uint64
	ContextPath string
	Scheme      string
}
