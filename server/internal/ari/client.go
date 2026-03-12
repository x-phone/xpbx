package ari

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	baseURL  string
	user     string
	password string
	http     *http.Client
}

func NewClient(baseURL, user, password string) *Client {
	return &Client{
		baseURL:  baseURL,
		user:     user,
		password: password,
		http:     &http.Client{Timeout: 5 * time.Second},
	}
}

type Endpoint struct {
	Resource   string   `json:"resource"`
	State      string   `json:"state"`
	Technology string   `json:"technology"`
	ChannelIDs []string `json:"channel_ids"`
}

type Channel struct {
	ID           string       `json:"id"`
	Name         string       `json:"name"`
	State        string       `json:"state"`
	Caller       CallerID     `json:"caller"`
	Connected    CallerID     `json:"connected"`
	CreationTime string       `json:"creationtime"`
	Dialplan     DialplanInfo `json:"dialplan"`
}

type CallerID struct {
	Name   string `json:"name"`
	Number string `json:"number"`
}

type DialplanInfo struct {
	Context  string `json:"context"`
	Exten    string `json:"exten"`
	Priority int    `json:"priority"`
	AppName  string `json:"app_name"`
	AppData  string `json:"app_data"`
}

type AsteriskInfo struct {
	Build  BuildInfo  `json:"build"`
	System SystemInfo `json:"system"`
	Status StatusInfo `json:"status"`
	Config ConfigInfo `json:"config"`
}

type BuildInfo struct {
	OS      string `json:"os"`
	Machine string `json:"machine"`
	Options string `json:"options"`
}

type SystemInfo struct {
	Version  string `json:"version"`
	EntityID string `json:"entity_id"`
}

type StatusInfo struct {
	StartupTime    string `json:"startup_time"`
	LastReloadTime string `json:"last_reload_time"`
}

type ConfigInfo struct {
	Name            string `json:"name"`
	DefaultLanguage string `json:"default_language"`
}

func (c *Client) do(method, path string, body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest(method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(c.user, c.password)
	req.Header.Set("Content-Type", "application/json")
	return c.http.Do(req)
}

func (c *Client) GetEndpoints() ([]Endpoint, error) {
	resp, err := c.do("GET", "/ari/endpoints", nil)
	if err != nil {
		return nil, fmt.Errorf("get endpoints: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get endpoints: status %d", resp.StatusCode)
	}

	var endpoints []Endpoint
	if err := json.NewDecoder(resp.Body).Decode(&endpoints); err != nil {
		return nil, fmt.Errorf("decode endpoints: %w", err)
	}
	return endpoints, nil
}

func (c *Client) GetChannels() ([]Channel, error) {
	resp, err := c.do("GET", "/ari/channels", nil)
	if err != nil {
		return nil, fmt.Errorf("get channels: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get channels: status %d", resp.StatusCode)
	}

	var channels []Channel
	if err := json.NewDecoder(resp.Body).Decode(&channels); err != nil {
		return nil, fmt.Errorf("decode channels: %w", err)
	}
	return channels, nil
}

func (c *Client) HangupChannel(channelID string) error {
	resp, err := c.do("DELETE", "/ari/channels/"+channelID, nil)
	if err != nil {
		return fmt.Errorf("hangup channel: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("hangup channel: status %d", resp.StatusCode)
	}
	return nil
}

func (c *Client) GetInfo() (*AsteriskInfo, error) {
	resp, err := c.do("GET", "/ari/asterisk/info", nil)
	if err != nil {
		return nil, fmt.Errorf("get info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("get info: status %d", resp.StatusCode)
	}

	var info AsteriskInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, fmt.Errorf("decode info: %w", err)
	}
	return &info, nil
}

func (c *Client) ReloadModule(module string) error {
	resp, err := c.do("PUT", "/ari/asterisk/modules/"+module, nil)
	if err != nil {
		return fmt.Errorf("reload module: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("reload module %s: status %d", module, resp.StatusCode)
	}
	return nil
}

// RestartModule unloads then loads a module, forcing it to re-open all
// resources (e.g. SQLite connections). ReloadModule alone doesn't re-open
// file handles, so WAL changes from other processes remain invisible.
func (c *Client) RestartModule(module string) error {
	// Unload
	resp, err := c.do("DELETE", "/ari/asterisk/modules/"+module, nil)
	if err != nil {
		return fmt.Errorf("unload module %s: %w", module, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unload module %s: status %d", module, resp.StatusCode)
	}

	// Load
	resp, err = c.do("POST", "/ari/asterisk/modules/"+module, nil)
	if err != nil {
		return fmt.Errorf("load module %s: %w", module, err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("load module %s: status %d", module, resp.StatusCode)
	}
	return nil
}
