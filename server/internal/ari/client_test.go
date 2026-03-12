package ari

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetEndpoints(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/ari/endpoints" {
			t.Errorf("path = %q, want /ari/endpoints", r.URL.Path)
		}
		if r.Method != "GET" {
			t.Errorf("method = %q, want GET", r.Method)
		}
		// Verify basic auth
		user, pass, ok := r.BasicAuth()
		if !ok || user != "admin" || pass != "secret" {
			t.Errorf("auth = %q/%q, want admin/secret", user, pass)
		}

		json.NewEncoder(w).Encode([]Endpoint{
			{Resource: "1001", State: "online", Technology: "PJSIP"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	endpoints, err := client.GetEndpoints()
	if err != nil {
		t.Fatalf("GetEndpoints: %v", err)
	}
	if len(endpoints) != 1 {
		t.Fatalf("expected 1 endpoint, got %d", len(endpoints))
	}
	if endpoints[0].Resource != "1001" {
		t.Errorf("resource = %q, want %q", endpoints[0].Resource, "1001")
	}
}

func TestGetChannels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode([]Channel{
			{ID: "ch1", Name: "PJSIP/1001-00000001", State: "Up"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	channels, err := client.GetChannels()
	if err != nil {
		t.Fatalf("GetChannels: %v", err)
	}
	if len(channels) != 1 {
		t.Fatalf("expected 1 channel, got %d", len(channels))
	}
	if channels[0].State != "Up" {
		t.Errorf("state = %q, want %q", channels[0].State, "Up")
	}
}

func TestGetInfo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		json.NewEncoder(w).Encode(AsteriskInfo{
			System: SystemInfo{Version: "20.5.0"},
			Config: ConfigInfo{Name: "xpbx-asterisk"},
		})
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	info, err := client.GetInfo()
	if err != nil {
		t.Fatalf("GetInfo: %v", err)
	}
	if info.System.Version != "20.5.0" {
		t.Errorf("version = %q, want %q", info.System.Version, "20.5.0")
	}
}

func TestHangupChannel(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %q, want DELETE", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	if err := client.HangupChannel("ch1"); err != nil {
		t.Errorf("HangupChannel: %v", err)
	}
}

func TestReloadModule(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PUT" {
			t.Errorf("method = %q, want PUT", r.Method)
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	if err := client.ReloadModule("res_config_sqlite3.so"); err != nil {
		t.Errorf("ReloadModule: %v", err)
	}
}

func TestRestartModule(t *testing.T) {
	var methods []string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		methods = append(methods, r.Method)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	if err := client.RestartModule("res_config_sqlite3.so"); err != nil {
		t.Fatalf("RestartModule: %v", err)
	}

	if len(methods) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(methods))
	}
	if methods[0] != "DELETE" {
		t.Errorf("first request method = %q, want DELETE (unload)", methods[0])
	}
	if methods[1] != "POST" {
		t.Errorf("second request method = %q, want POST (load)", methods[1])
	}
}

func TestGetEndpoints_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	client := NewClient(server.URL, "admin", "secret")
	_, err := client.GetEndpoints()
	if err == nil {
		t.Error("expected error for 500 response")
	}
}
