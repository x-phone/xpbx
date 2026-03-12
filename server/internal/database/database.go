package database

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	log "github.com/sirupsen/logrus"
	_ "modernc.org/sqlite"
)

type DB struct {
	*sql.DB
}

func Open(dbPath string) (*DB, error) {
	dir := filepath.Dir(dbPath)
	// 0777 so Asterisk (uid=1000) can also write to the directory (for WAL files)
	if err := os.MkdirAll(dir, 0777); err != nil {
		return nil, fmt.Errorf("create db directory: %w", err)
	}
	os.Chmod(dir, 0777)

	db, err := sql.Open("sqlite", dbPath+"?_pragma=journal_mode(WAL)&_pragma=busy_timeout(5000)&_pragma=foreign_keys(ON)")
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("ping database: %w", err)
	}

	// Make DB files writable by all (Asterisk runs as uid=1000, xpbx as root)
	os.Chmod(dbPath, 0666)

	log.WithField("path", dbPath).Info("Database connected (WAL mode)")
	return &DB{db}, nil
}

func (db *DB) Migrate() error {
	migrations := []string{
		// ps_endpoints — Narrow schema: only columns we populate.
		// res_config_sqlite3 does SELECT * and converts NULL → "" which breaks
		// Asterisk enum/bool/int parsers. Omitting unused columns makes Asterisk
		// use compiled-in defaults instead.
		`CREATE TABLE IF NOT EXISTS ps_endpoints (
			id TEXT PRIMARY KEY,
			transport TEXT,
			aors TEXT,
			auth TEXT,
			context TEXT,
			message_context TEXT DEFAULT 'from-internal-messages',
			disallow TEXT,
			allow TEXT,
			direct_media TEXT,
			connected_line_method TEXT,
			direct_media_method TEXT,
			dtmf_mode TEXT,
			force_rport TEXT,
			rewrite_contact TEXT,
			rtp_symmetric TEXT,
			callerid TEXT,
			-- xpbx metadata (ignored by Asterisk)
			display_name TEXT DEFAULT '',
			endpoint_type TEXT DEFAULT 'extension',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ps_auths — Narrow schema: only columns we populate
		`CREATE TABLE IF NOT EXISTS ps_auths (
			id TEXT PRIMARY KEY,
			auth_type TEXT,
			password TEXT,
			username TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ps_aors — Narrow schema: only columns we populate
		`CREATE TABLE IF NOT EXISTS ps_aors (
			id TEXT PRIMARY KEY,
			contact TEXT,
			default_expiration TEXT,
			max_contacts TEXT,
			minimum_expiration TEXT,
			maximum_expiration TEXT,
			remove_existing TEXT,
			remove_unavailable TEXT,
			qualify_frequency TEXT,
			qualify_timeout TEXT,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// ps_contacts — Full schema: Asterisk WRITES here when phones register.
		// All columns needed so Asterisk can store registration data.
		`CREATE TABLE IF NOT EXISTS ps_contacts (
			id TEXT PRIMARY KEY,
			uri TEXT,
			expiration_time TEXT,
			qualify_frequency TEXT,
			outbound_proxy TEXT,
			path TEXT,
			user_agent TEXT,
			qualify_timeout TEXT,
			reg_server TEXT,
			authenticate_qualify TEXT,
			via_addr TEXT,
			via_port TEXT,
			call_id TEXT,
			endpoint TEXT,
			prune_on_boot TEXT,
			qualify_2xx_only TEXT
		)`,

		// extensions — Asterisk Realtime dialplan
		`CREATE TABLE IF NOT EXISTS extensions (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			context TEXT NOT NULL,
			exten TEXT NOT NULL,
			priority INTEGER NOT NULL,
			app TEXT NOT NULL,
			appdata TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// pbx_trunks — xpbx metadata only (not read by Asterisk)
		`CREATE TABLE IF NOT EXISTS pbx_trunks (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT UNIQUE NOT NULL,
			display_name TEXT DEFAULT '',
			provider TEXT DEFAULT '',
			host TEXT NOT NULL,
			port INTEGER DEFAULT 5060,
			context TEXT DEFAULT 'from-trunk',
			transport TEXT DEFAULT 'transport-udp',
			codecs TEXT DEFAULT 'ulaw',
			auth_user TEXT DEFAULT '',
			auth_pass TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// voicemail_settings — per-extension voicemail config (managed by xpbx)
		`CREATE TABLE IF NOT EXISTS voicemail_settings (
			extension TEXT PRIMARY KEY,
			enabled INTEGER DEFAULT 1,
			pin TEXT DEFAULT '0000',
			email TEXT DEFAULT '',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)`,

		// Indexes
		`CREATE INDEX IF NOT EXISTS idx_extensions_context_exten ON extensions(context, exten)`,
		`CREATE INDEX IF NOT EXISTS idx_ps_contacts_endpoint ON ps_contacts(endpoint)`,
	}

	// Column migrations for existing databases
	alterMigrations := []string{
		`ALTER TABLE ps_aors ADD COLUMN remove_unavailable TEXT`,
		`ALTER TABLE ps_aors ADD COLUMN qualify_timeout TEXT`,
		`ALTER TABLE ps_endpoints ADD COLUMN message_context TEXT DEFAULT 'from-internal-messages'`,
	}
	for _, m := range alterMigrations {
		db.Exec(m) // Ignore "duplicate column" errors on fresh DBs
	}

	for _, m := range migrations {
		if _, err := db.Exec(m); err != nil {
			return fmt.Errorf("migration failed: %w\nSQL: %s", err, m)
		}
	}

	log.Info("Database migrations complete")
	return nil
}

// Checkpoint forces a WAL checkpoint so that other processes (Asterisk)
// sharing the same database file can see recent writes immediately.
func (db *DB) Checkpoint() {
	if _, err := db.Exec(`PRAGMA wal_checkpoint(PASSIVE)`); err != nil {
		log.WithError(err).Warn("WAL checkpoint failed")
	}
}
