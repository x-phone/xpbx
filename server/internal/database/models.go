package database

import "time"

// Extension represents a SIP extension managed by xpbx.
// Maps to ps_endpoints + ps_auths + ps_aors in Asterisk Realtime.
type Extension struct {
	ID          int64     `json:"id"`
	Extension   string    `json:"extension"`
	DisplayName string    `json:"display_name"`
	Password    string    `json:"password"`
	Context     string    `json:"context"`
	Transport   string    `json:"transport"`
	Codecs      string    `json:"codecs"`
	MaxContacts int       `json:"max_contacts"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Trunk represents a SIP trunk for outbound/inbound routing.
// Maps to ps_endpoints + ps_auths + ps_aors + pbx_trunks in Asterisk Realtime.
type Trunk struct {
	ID          int64     `json:"id"`
	Name        string    `json:"name"`
	DisplayName string    `json:"display_name"`
	Provider    string    `json:"provider"`
	Host        string    `json:"host"`
	Port        int       `json:"port"`
	Context     string    `json:"context"`
	Transport   string    `json:"transport"`
	Codecs      string    `json:"codecs"`
	AuthUser    string    `json:"auth_user"`
	AuthPass    string    `json:"auth_pass"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DialplanRule represents a single dialplan entry.
// Maps to the extensions table in Asterisk Realtime.
type DialplanRule struct {
	ID        int64     `json:"id"`
	Context   string    `json:"context"`
	Exten     string    `json:"exten"`
	Priority  int       `json:"priority"`
	App       string    `json:"app"`
	AppData   string    `json:"appdata"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// VoicemailSettings stores per-extension voicemail configuration.
type VoicemailSettings struct {
	Extension string    `json:"extension"`
	Enabled   bool      `json:"enabled"`
	PIN       string    `json:"pin"`
	Email     string    `json:"email"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Contact represents a registered SIP contact (written by Asterisk).
type Contact struct {
	ID             string `json:"id"`
	URI            string `json:"uri"`
	ExpirationTime int64  `json:"expiration_time"`
	UserAgent      string `json:"user_agent"`
}
