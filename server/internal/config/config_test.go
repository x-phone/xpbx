package config

import (
	"os"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	// Clear any env vars that might be set
	os.Unsetenv("XPBX_LISTEN_ADDR")
	os.Unsetenv("XPBX_DB_PATH")
	os.Unsetenv("ARI_HOST")
	os.Unsetenv("ARI_PORT")
	os.Unsetenv("ARI_USER")
	os.Unsetenv("ARI_PASSWORD")
	os.Unsetenv("EXTERNAL_IP")
	os.Unsetenv("SIP_PORT")

	cfg := Load()

	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":8080")
	}
	if cfg.DBPath != "/data/asterisk-realtime.db" {
		t.Errorf("DBPath = %q, want %q", cfg.DBPath, "/data/asterisk-realtime.db")
	}
	if cfg.ARIHost != "asterisk" {
		t.Errorf("ARIHost = %q, want %q", cfg.ARIHost, "asterisk")
	}
	if cfg.ARIPort != 8088 {
		t.Errorf("ARIPort = %d, want %d", cfg.ARIPort, 8088)
	}
	if cfg.ARIUser != "xpbx" {
		t.Errorf("ARIUser = %q, want %q", cfg.ARIUser, "xpbx")
	}
	if cfg.SIPPort != 5060 {
		t.Errorf("SIPPort = %d, want %d", cfg.SIPPort, 5060)
	}
}

func TestLoadOverrides(t *testing.T) {
	os.Setenv("XPBX_LISTEN_ADDR", ":9090")
	os.Setenv("ARI_PORT", "9999")
	os.Setenv("EXTERNAL_IP", "10.0.0.1,10.0.0.2")
	defer func() {
		os.Unsetenv("XPBX_LISTEN_ADDR")
		os.Unsetenv("ARI_PORT")
		os.Unsetenv("EXTERNAL_IP")
	}()

	cfg := Load()

	if cfg.ListenAddr != ":9090" {
		t.Errorf("ListenAddr = %q, want %q", cfg.ListenAddr, ":9090")
	}
	if cfg.ARIPort != 9999 {
		t.Errorf("ARIPort = %d, want %d", cfg.ARIPort, 9999)
	}
	if len(cfg.HostIPs) != 2 {
		t.Fatalf("HostIPs len = %d, want 2", len(cfg.HostIPs))
	}
	if cfg.HostIPs[0] != "10.0.0.1" {
		t.Errorf("HostIPs[0] = %q, want %q", cfg.HostIPs[0], "10.0.0.1")
	}
	if cfg.HostIPs[1] != "10.0.0.2" {
		t.Errorf("HostIPs[1] = %q, want %q", cfg.HostIPs[1], "10.0.0.2")
	}
}

func TestDetectHostIPs_Dedup(t *testing.T) {
	os.Setenv("EXTERNAL_IP", "10.0.0.1, 10.0.0.1, 10.0.0.2")
	defer os.Unsetenv("EXTERNAL_IP")

	ips := detectHostIPs()
	if len(ips) != 2 {
		t.Errorf("expected 2 unique IPs, got %d: %v", len(ips), ips)
	}
}

func TestDetectHostIPs_Empty(t *testing.T) {
	os.Unsetenv("EXTERNAL_IP")

	ips := detectHostIPs()
	if len(ips) != 1 || ips[0] != "localhost" {
		t.Errorf("expected [localhost], got %v", ips)
	}
}

func TestARIURL(t *testing.T) {
	cfg := &Config{ARIHost: "myhost", ARIPort: 9090}
	if cfg.ARIURL() != "http://myhost:9090" {
		t.Errorf("ARIURL = %q, want %q", cfg.ARIURL(), "http://myhost:9090")
	}
}

func TestEnvIntOrDefault_InvalidInput(t *testing.T) {
	os.Setenv("TEST_INT", "not-a-number")
	defer os.Unsetenv("TEST_INT")

	got := envIntOrDefault("TEST_INT", 42)
	if got != 42 {
		t.Errorf("got %d, want 42 (default) for invalid input", got)
	}
}
