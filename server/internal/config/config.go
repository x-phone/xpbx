package config

import (
	"os"
	"strconv"
	"strings"
)

type Config struct {
	// Server
	ListenAddr string
	DBPath     string
	DataDir    string // Shared data directory (also mounted by Asterisk)

	// ARI
	ARIHost     string
	ARIPort     int
	ARIUser     string
	ARIPassword string

	// SIP / NAT
	HostIPs []string // All reachable host IPs for SIP clients
	SIPPort int
}

func Load() *Config {
	dbPath := envOrDefault("XPBX_DB_PATH", "/data/asterisk-realtime.db")
	return &Config{
		ListenAddr:  envOrDefault("XPBX_LISTEN_ADDR", ":8080"),
		DBPath:      dbPath,
		DataDir:     envOrDefault("XPBX_DATA_DIR", "/data"),
		ARIHost:     envOrDefault("ARI_HOST", "asterisk"),
		ARIPort:     envIntOrDefault("ARI_PORT", 8088),
		ARIUser:     envOrDefault("ARI_USER", "xpbx"),
		ARIPassword: envOrDefault("ARI_PASSWORD", "secret"),
		HostIPs:     detectHostIPs(),
		SIPPort:     envIntOrDefault("SIP_PORT", 5060),
	}
}

// detectHostIPs returns the host IPs where Asterisk's SIP port is reachable.
// Reads comma-separated IPs from the EXTERNAL_IP env var (set by Makefile or .env).
func detectHostIPs() []string {
	var ips []string
	seen := map[string]bool{}

	if v := os.Getenv("EXTERNAL_IP"); v != "" {
		for _, ip := range strings.Split(v, ",") {
			ip = strings.TrimSpace(ip)
			if ip != "" && !seen[ip] {
				seen[ip] = true
				ips = append(ips, ip)
			}
		}
	}

	if len(ips) == 0 {
		return []string{"localhost"}
	}
	return ips
}

func (c *Config) ARIURL() string {
	return "http://" + c.ARIHost + ":" + strconv.Itoa(c.ARIPort)
}

func envOrDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envIntOrDefault(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
