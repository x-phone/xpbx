package database

import (
	"database/sql"
	"fmt"
	"time"
)

func (db *DB) ListExtensions() ([]Extension, error) {
	rows, err := db.Query(`
		SELECT e.id, e.display_name, e.context, e.transport, e.allow,
		       a.password, ao.max_contacts, e.created_at, e.updated_at
		FROM ps_endpoints e
		LEFT JOIN ps_auths a ON a.id = e.auth
		LEFT JOIN ps_aors ao ON ao.id = e.aors
		WHERE e.endpoint_type = 'extension'
		ORDER BY e.id`)
	if err != nil {
		return nil, fmt.Errorf("list extensions: %w", err)
	}
	defer rows.Close()

	var exts []Extension
	for rows.Next() {
		var ext Extension
		var maxContacts string
		if err := rows.Scan(&ext.Extension, &ext.DisplayName, &ext.Context,
			&ext.Transport, &ext.Codecs, &ext.Password, &maxContacts,
			&ext.CreatedAt, &ext.UpdatedAt); err != nil {
			return nil, fmt.Errorf("scan extension: %w", err)
		}
		fmt.Sscanf(maxContacts, "%d", &ext.MaxContacts)
		exts = append(exts, ext)
	}
	return exts, rows.Err()
}

func (db *DB) GetExtension(id string) (*Extension, error) {
	var ext Extension
	var maxContacts string
	err := db.QueryRow(`
		SELECT e.id, e.display_name, e.context, e.transport, e.allow,
		       a.password, ao.max_contacts, e.created_at, e.updated_at
		FROM ps_endpoints e
		LEFT JOIN ps_auths a ON a.id = e.auth
		LEFT JOIN ps_aors ao ON ao.id = e.aors
		WHERE e.id = ? AND e.endpoint_type = 'extension'`, id).
		Scan(&ext.Extension, &ext.DisplayName, &ext.Context,
			&ext.Transport, &ext.Codecs, &ext.Password, &maxContacts,
			&ext.CreatedAt, &ext.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get extension %s: %w", id, err)
	}
	fmt.Sscanf(maxContacts, "%d", &ext.MaxContacts)
	return &ext, nil
}

func (db *DB) CreateExtension(ext *Extension) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	// ps_endpoints
	_, err = tx.Exec(`INSERT INTO ps_endpoints
		(id, transport, aors, auth, context, disallow, allow, direct_media,
		 force_rport, rewrite_contact, rtp_symmetric, connected_line_method,
		 direct_media_method, dtmf_mode, display_name, endpoint_type, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, 'all', ?, 'no', 'yes', 'yes', 'yes', 'invite',
		 'invite', 'rfc4733', ?, 'extension', ?, ?)`,
		ext.Extension, ext.Transport, ext.Extension, ext.Extension,
		ext.Context, ext.Codecs, ext.DisplayName, now, now)
	if err != nil {
		return fmt.Errorf("insert ps_endpoints: %w", err)
	}

	// ps_auths
	_, err = tx.Exec(`INSERT INTO ps_auths (id, auth_type, password, username, created_at)
		VALUES (?, 'userpass', ?, ?, ?)`,
		ext.Extension, ext.Password, ext.Extension, now)
	if err != nil {
		return fmt.Errorf("insert ps_auths: %w", err)
	}

	// ps_aors
	_, err = tx.Exec(`INSERT INTO ps_aors (id, max_contacts, default_expiration, minimum_expiration, maximum_expiration, qualify_frequency, qualify_timeout, remove_existing, remove_unavailable, created_at)
		VALUES (?, ?, '3600', '60', '7200', '60', '3.0', 'yes', 'yes', ?)`,
		ext.Extension, fmt.Sprintf("%d", ext.MaxContacts), now)
	if err != nil {
		return fmt.Errorf("insert ps_aors: %w", err)
	}

	return tx.Commit()
}

func (db *DB) UpdateExtension(ext *Extension) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	now := time.Now()

	_, err = tx.Exec(`UPDATE ps_endpoints SET
		context=?, transport=?, allow=?, display_name=?, updated_at=?
		WHERE id=? AND endpoint_type='extension'`,
		ext.Context, ext.Transport, ext.Codecs, ext.DisplayName, now, ext.Extension)
	if err != nil {
		return fmt.Errorf("update ps_endpoints: %w", err)
	}

	_, err = tx.Exec(`UPDATE ps_auths SET password=?, username=? WHERE id=?`,
		ext.Password, ext.Extension, ext.Extension)
	if err != nil {
		return fmt.Errorf("update ps_auths: %w", err)
	}

	_, err = tx.Exec(`UPDATE ps_aors SET max_contacts=? WHERE id=?`,
		fmt.Sprintf("%d", ext.MaxContacts), ext.Extension)
	if err != nil {
		return fmt.Errorf("update ps_aors: %w", err)
	}

	return tx.Commit()
}

func (db *DB) DeleteExtension(id string) error {
	tx, err := db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	tx.Exec(`DELETE FROM ps_aors WHERE id=?`, id)
	tx.Exec(`DELETE FROM ps_auths WHERE id=?`, id)
	tx.Exec(`DELETE FROM ps_contacts WHERE endpoint=?`, id)
	_, err = tx.Exec(`DELETE FROM ps_endpoints WHERE id=? AND endpoint_type='extension'`, id)
	if err != nil {
		return fmt.Errorf("delete ps_endpoints: %w", err)
	}

	return tx.Commit()
}
