package database

import (
	"database/sql"
	"fmt"
	"time"
)

func (db *DB) ListTrunks() ([]Trunk, error) {
	rows, err := db.Query(`
		SELECT t.id, t.name, t.display_name, t.provider, t.host, t.port,
		       t.context, t.transport, t.codecs, t.auth_user, t.auth_pass,
		       t.created_at, t.updated_at
		FROM pbx_trunks t ORDER BY t.name`)
	if err != nil {
		return nil, fmt.Errorf("list trunks: %w", err)
	}
	defer rows.Close()

	var trunks []Trunk
	for rows.Next() {
		var t Trunk
		if err := rows.Scan(&t.ID, &t.Name, &t.DisplayName, &t.Provider,
			&t.Host, &t.Port, &t.Context, &t.Transport, &t.Codecs,
			&t.AuthUser, &t.AuthPass, &t.CreatedAt, &t.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan trunk: %w", err)
		}
		trunks = append(trunks, t)
	}
	return trunks, rows.Err()
}

func (db *DB) GetTrunk(id int64) (*Trunk, error) {
	var t Trunk
	err := db.QueryRow(`
		SELECT id, name, display_name, provider, host, port,
		       context, transport, codecs, auth_user, auth_pass,
		       created_at, updated_at
		FROM pbx_trunks WHERE id=?`, id).
		Scan(&t.ID, &t.Name, &t.DisplayName, &t.Provider,
			&t.Host, &t.Port, &t.Context, &t.Transport, &t.Codecs,
			&t.AuthUser, &t.AuthPass, &t.CreatedAt, &t.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get trunk %d: %w", id, err)
	}
	return &t, nil
}

func (db *DB) CreateTrunk(t *Trunk) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// pbx_trunks metadata
	res, err := tx.Exec(`INSERT INTO pbx_trunks
		(name, display_name, provider, host, port, context, transport, codecs, auth_user, auth_pass, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		t.Name, t.DisplayName, t.Provider, t.Host, t.Port,
		t.Context, t.Transport, t.Codecs, t.AuthUser, t.AuthPass, now, now)
	if err != nil {
		return fmt.Errorf("insert pbx_trunks: %w", err)
	}
	t.ID, _ = res.LastInsertId()

	contact := fmt.Sprintf("sip:%s:%d", t.Host, t.Port)

	// ps_endpoints
	_, err = tx.Exec(`INSERT INTO ps_endpoints
		(id, transport, aors, auth, context, disallow, allow, direct_media,
		 force_rport, rewrite_contact, rtp_symmetric, connected_line_method,
		 direct_media_method, dtmf_mode, display_name, endpoint_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'all', ?, 'no', 'yes', 'yes', 'yes', 'invite',
		 'invite', 'rfc4733', ?, 'trunk', ?, ?)`,
		t.Name, t.Transport, t.Name+"-aor", t.Name, t.Context, t.Codecs, t.DisplayName, now, now)
	if err != nil {
		return fmt.Errorf("insert ps_endpoints for trunk: %w", err)
	}

	// ps_auths (only if auth credentials provided)
	if t.AuthUser != "" {
		_, err = tx.Exec(`INSERT INTO ps_auths (id, auth_type, password, username, created_at)
			VALUES (?, 'userpass', ?, ?, ?)`,
			t.Name, t.AuthPass, t.AuthUser, now)
		if err != nil {
			return fmt.Errorf("insert ps_auths for trunk: %w", err)
		}
	} else {
		// Trunk without auth — remove auth reference from endpoint
		_, err = tx.Exec(`UPDATE ps_endpoints SET auth='' WHERE id=?`, t.Name)
		if err != nil {
			return fmt.Errorf("clear auth on endpoint: %w", err)
		}
	}

	// ps_aors with static contact
	_, err = tx.Exec(`INSERT INTO ps_aors (id, contact, max_contacts, default_expiration, minimum_expiration, maximum_expiration, remove_existing, remove_unavailable, qualify_frequency, qualify_timeout, created_at)
		VALUES (?, ?, '10', '3600', '60', '7200', 'yes', 'yes', '60', '3.0', ?)`,
		t.Name+"-aor", contact, now)
	if err != nil {
		return fmt.Errorf("insert ps_aors for trunk: %w", err)
	}

	return tx.Commit()
}

func (db *DB) UpdateTrunk(t *Trunk) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// Get current trunk name for Asterisk table updates
	var oldName string
	err = tx.QueryRow(`SELECT name FROM pbx_trunks WHERE id=?`, t.ID).Scan(&oldName)
	if err != nil {
		return fmt.Errorf("get old trunk name: %w", err)
	}

	// Update pbx_trunks
	_, err = tx.Exec(`UPDATE pbx_trunks SET
		name=?, display_name=?, provider=?, host=?, port=?, context=?,
		transport=?, codecs=?, auth_user=?, auth_pass=?, updated_at=?
		WHERE id=?`,
		t.Name, t.DisplayName, t.Provider, t.Host, t.Port, t.Context,
		t.Transport, t.Codecs, t.AuthUser, t.AuthPass, now, t.ID)
	if err != nil {
		return fmt.Errorf("update pbx_trunks: %w", err)
	}

	contact := fmt.Sprintf("sip:%s:%d", t.Host, t.Port)

	// Update Asterisk realtime tables
	_, err = tx.Exec(`UPDATE ps_endpoints SET
		context=?, transport=?, allow=?, display_name=?, updated_at=?
		WHERE id=? AND endpoint_type='trunk'`,
		t.Context, t.Transport, t.Codecs, t.DisplayName, now, oldName)
	if err != nil {
		return fmt.Errorf("update ps_endpoints: %w", err)
	}

	_, err = tx.Exec(`UPDATE ps_aors SET contact=? WHERE id=?`,
		contact, oldName+"-aor")
	if err != nil {
		return fmt.Errorf("update ps_aors: %w", err)
	}

	return tx.Commit()
}

func (db *DB) DeleteTrunk(id int64) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var name string
	err = tx.QueryRow(`SELECT name FROM pbx_trunks WHERE id=?`, id).Scan(&name)
	if err != nil {
		return fmt.Errorf("get trunk name: %w", err)
	}

	tx.Exec(`DELETE FROM ps_aors WHERE id=?`, name+"-aor")
	tx.Exec(`DELETE FROM ps_auths WHERE id=?`, name)
	tx.Exec(`DELETE FROM ps_endpoints WHERE id=? AND endpoint_type='trunk'`, name)
	_, err = tx.Exec(`DELETE FROM pbx_trunks WHERE id=?`, id)
	if err != nil {
		return fmt.Errorf("delete pbx_trunks: %w", err)
	}

	return tx.Commit()
}
