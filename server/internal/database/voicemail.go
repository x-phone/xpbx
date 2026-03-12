package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/sirupsen/logrus"
)

// GetVoicemailSettings returns voicemail settings for an extension.
// Returns default settings if none exist.
func (db *DB) GetVoicemailSettings(extension string) (*VoicemailSettings, error) {
	var vm VoicemailSettings
	err := db.QueryRow(`
		SELECT extension, enabled, pin, email, created_at, updated_at
		FROM voicemail_settings WHERE extension=?`, extension).
		Scan(&vm.Extension, &vm.Enabled, &vm.PIN, &vm.Email, &vm.CreatedAt, &vm.UpdatedAt)
	if err == sql.ErrNoRows {
		// Return defaults
		return &VoicemailSettings{
			Extension: extension,
			Enabled:   true,
			PIN:       "0000",
		}, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get voicemail settings %s: %w", extension, err)
	}
	return &vm, nil
}

// UpsertVoicemailSettings creates or updates voicemail settings for an extension.
func (db *DB) UpsertVoicemailSettings(vm *VoicemailSettings) error {
	now := time.Now()
	_, err := db.Exec(`INSERT INTO voicemail_settings (extension, enabled, pin, email, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?)
		ON CONFLICT(extension) DO UPDATE SET
			enabled=excluded.enabled, pin=excluded.pin, email=excluded.email, updated_at=excluded.updated_at`,
		vm.Extension, vm.Enabled, vm.PIN, vm.Email, now, now)
	if err != nil {
		return fmt.Errorf("upsert voicemail settings: %w", err)
	}
	return nil
}

// DeleteVoicemailSettings removes voicemail settings for an extension.
func (db *DB) DeleteVoicemailSettings(extension string) error {
	_, err := db.Exec(`DELETE FROM voicemail_settings WHERE extension=?`, extension)
	return err
}

// SyncVoicemailMailboxes regenerates the voicemail_mailboxes.conf file
// in the shared data directory. Asterisk's voicemail.conf includes this file.
func (db *DB) SyncVoicemailMailboxes(dataDir string) error {
	exts, err := db.ListExtensions()
	if err != nil {
		return fmt.Errorf("list extensions for voicemail sync: %w", err)
	}

	var lines []string
	lines = append(lines, "; Auto-managed by xpbx — do not edit manually")

	for _, ext := range exts {
		vm, err := db.GetVoicemailSettings(ext.Extension)
		if err != nil {
			log.WithError(err).WithField("extension", ext.Extension).Warn("Failed to get voicemail settings")
			continue
		}

		if !vm.Enabled {
			continue
		}

		name := ext.DisplayName
		if name == "" {
			name = "Extension " + ext.Extension
		}

		// Format: extension => password,name,email,pager,options
		email := vm.Email
		options := ""
		if email != "" {
			options = "attach=yes"
		}
		lines = append(lines, fmt.Sprintf("%s => %s,%s,%s,,%s", ext.Extension, vm.PIN, name, email, options))
	}
	lines = append(lines, "")

	path := filepath.Join(dataDir, "voicemail_mailboxes.conf")
	if err := os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0666); err != nil {
		return fmt.Errorf("write voicemail mailboxes: %w", err)
	}

	log.WithFields(log.Fields{"path": path, "count": len(exts)}).Debug("Synced voicemail mailboxes")
	return nil
}

// CountVoicemailMessages counts messages in an extension's voicemail spool.
// Returns (new, old) counts.
func CountVoicemailMessages(dataDir, extension string) (newCount, oldCount int) {
	// Asterisk stores voicemail in /var/spool/asterisk/voicemail/default/<ext>/
	// But since we share /data, check if there's a symlink or standard path.
	// For now, check the standard Asterisk path inside the container.
	// This is best-effort — returns 0,0 if the directory doesn't exist.
	base := filepath.Join("/var/spool/asterisk/voicemail/default", extension)

	newDir := filepath.Join(base, "INBOX")
	oldDir := filepath.Join(base, "Old")

	newCount = countWavFiles(newDir)
	oldCount = countWavFiles(oldDir)
	return
}

func countWavFiles(dir string) int {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return 0
	}
	count := 0
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".wav") {
			count++
		}
	}
	return count
}
