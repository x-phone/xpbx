package database

import (
	"database/sql"
	"fmt"
	"time"
)

func (db *DB) ListDialplanRules() ([]DialplanRule, error) {
	rows, err := db.Query(`
		SELECT id, context, exten, priority, app, appdata, created_at, updated_at
		FROM extensions ORDER BY context, exten, priority`)
	if err != nil {
		return nil, fmt.Errorf("list dialplan: %w", err)
	}
	defer rows.Close()

	var rules []DialplanRule
	for rows.Next() {
		var r DialplanRule
		if err := rows.Scan(&r.ID, &r.Context, &r.Exten, &r.Priority,
			&r.App, &r.AppData, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dialplan: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

func (db *DB) GetDialplanRule(id int64) (*DialplanRule, error) {
	var r DialplanRule
	err := db.QueryRow(`
		SELECT id, context, exten, priority, app, appdata, created_at, updated_at
		FROM extensions WHERE id=?`, id).
		Scan(&r.ID, &r.Context, &r.Exten, &r.Priority,
			&r.App, &r.AppData, &r.CreatedAt, &r.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get dialplan %d: %w", id, err)
	}
	return &r, nil
}

func (db *DB) CreateDialplanRule(r *DialplanRule) error {
	now := time.Now()
	res, err := db.Exec(`INSERT INTO extensions
		(context, exten, priority, app, appdata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		r.Context, r.Exten, r.Priority, r.App, r.AppData, now, now)
	if err != nil {
		return fmt.Errorf("insert dialplan: %w", err)
	}
	r.ID, _ = res.LastInsertId()
	return nil
}

func (db *DB) UpdateDialplanRule(r *DialplanRule) error {
	now := time.Now()
	_, err := db.Exec(`UPDATE extensions SET
		context=?, exten=?, priority=?, app=?, appdata=?, updated_at=?
		WHERE id=?`,
		r.Context, r.Exten, r.Priority, r.App, r.AppData, now, r.ID)
	if err != nil {
		return fmt.Errorf("update dialplan: %w", err)
	}
	return nil
}

func (db *DB) DeleteDialplanRule(id int64) error {
	_, err := db.Exec(`DELETE FROM extensions WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete dialplan: %w", err)
	}
	return nil
}

// GetDialplanRulesForExten returns all dialplan rows for a context+exten pair.
func (db *DB) GetDialplanRulesForExten(context, exten string) ([]DialplanRule, error) {
	rows, err := db.Query(`
		SELECT id, context, exten, priority, app, appdata, created_at, updated_at
		FROM extensions WHERE context=? AND exten=?
		ORDER BY priority`, context, exten)
	if err != nil {
		return nil, fmt.Errorf("get dialplan for exten: %w", err)
	}
	defer rows.Close()

	var rules []DialplanRule
	for rows.Next() {
		var r DialplanRule
		if err := rows.Scan(&r.ID, &r.Context, &r.Exten, &r.Priority,
			&r.App, &r.AppData, &r.CreatedAt, &r.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan dialplan: %w", err)
		}
		rules = append(rules, r)
	}
	return rules, rows.Err()
}

// RoutingPattern identifies which quick-routing template to use.
type RoutingPattern string

const (
	PatternRingOnly      RoutingPattern = "ring_only"      // NoOp → Dial → Hangup
	PatternRingVoicemail RoutingPattern = "ring_voicemail"  // NoOp → Dial → VoiceMail(u) → Hangup
	PatternVoicemailOnly RoutingPattern = "voicemail_only"  // NoOp → VoiceMail → Hangup
)

// EnsureRouting creates (or replaces) the dialplan rules for an extension
// based on the selected routing pattern. This is the quick-routing shortcut.
func (db *DB) EnsureRouting(context, exten string, pattern RoutingPattern, timeout int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Remove any existing rules for this context+exten
	if _, err := tx.Exec(`DELETE FROM extensions WHERE context=? AND exten=?`, context, exten); err != nil {
		return fmt.Errorf("clear existing rules: %w", err)
	}

	type rule struct {
		pri  int
		app  string
		data string
	}

	var rules []rule

	switch pattern {
	case PatternRingVoicemail:
		rules = []rule{
			{1, "NoOp", fmt.Sprintf("Calling extension %s", exten)},
			{2, "Dial", fmt.Sprintf("PJSIP/%s,%d", exten, timeout)},
			{3, "VoiceMail", fmt.Sprintf("%s@default,u", exten)},
			{4, "Hangup", ""},
		}
	case PatternVoicemailOnly:
		rules = []rule{
			{1, "NoOp", fmt.Sprintf("Voicemail for extension %s", exten)},
			{2, "VoiceMail", fmt.Sprintf("%s@default", exten)},
			{3, "Hangup", ""},
		}
	default: // PatternRingOnly
		rules = []rule{
			{1, "NoOp", fmt.Sprintf("Calling extension %s", exten)},
			{2, "Dial", fmt.Sprintf("PJSIP/%s,%d", exten, timeout)},
			{3, "Hangup", ""},
		}
	}

	for _, r := range rules {
		if _, err := tx.Exec(`INSERT INTO extensions (context, exten, priority, app, appdata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			context, exten, r.pri, r.app, r.data, now, now); err != nil {
			return fmt.Errorf("insert rule %s: %w", r.app, err)
		}
	}

	return tx.Commit()
}

// EnsureTrunkRoute creates (or replaces) the dialplan rules that route
// a specific extension pattern through a SIP trunk.
// e.g. EnsureTrunkRoute("from-internal", "2001", "my-trunk", 30)
// produces: NoOp → Dial(PJSIP/${EXTEN}@my-trunk,30) → Hangup
func (db *DB) EnsureTrunkRoute(context, exten, trunkName string, timeout int) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	if _, err := tx.Exec(`DELETE FROM extensions WHERE context=? AND exten=?`, context, exten); err != nil {
		return fmt.Errorf("clear existing rules: %w", err)
	}

	rules := []struct {
		pri  int
		app  string
		data string
	}{
		{1, "NoOp", fmt.Sprintf("Route %s via trunk %s", exten, trunkName)},
		{2, "Dial", fmt.Sprintf("PJSIP/${EXTEN}@%s,%d", trunkName, timeout)},
		{3, "Hangup", ""},
	}

	for _, r := range rules {
		if _, err := tx.Exec(`INSERT INTO extensions (context, exten, priority, app, appdata, created_at, updated_at)
			VALUES (?, ?, ?, ?, ?, ?, ?)`,
			context, exten, r.pri, r.app, r.data, now, now); err != nil {
			return fmt.Errorf("insert rule %s: %w", r.app, err)
		}
	}

	return tx.Commit()
}

// DeleteDialplanRulesForExten removes all dialplan rows for a context+exten pair.
func (db *DB) DeleteDialplanRulesForExten(context, exten string) error {
	_, err := db.Exec(`DELETE FROM extensions WHERE context=? AND exten=?`, context, exten)
	if err != nil {
		return fmt.Errorf("delete dialplan for exten: %w", err)
	}
	return nil
}
