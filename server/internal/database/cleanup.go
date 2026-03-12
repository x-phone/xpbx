package database

import (
	"context"
	"time"

	log "github.com/sirupsen/logrus"
)

// PruneStaleContacts removes contacts whose expiration_time has passed.
// Uses a single fast DELETE with minimal lock time since Asterisk's
// res_config_sqlite3 driver also writes to this database concurrently.
func (db *DB) PruneStaleContacts() (int64, error) {
	now := time.Now().Unix()
	res, err := db.Exec(
		`DELETE FROM ps_contacts WHERE CAST(expiration_time AS INTEGER) > 0 AND CAST(expiration_time AS INTEGER) < ?`,
		now,
	)
	if err != nil {
		return 0, err
	}
	return res.RowsAffected()
}

// StartContactCleanup runs periodic stale contact pruning.
func (db *DB) StartContactCleanup(ctx context.Context, interval time.Duration) {
	if n, err := db.PruneStaleContacts(); err != nil {
		log.WithError(err).Warn("Failed to prune stale contacts on startup")
	} else if n > 0 {
		log.WithField("pruned", n).Info("Pruned stale SIP contacts on startup")
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if n, err := db.PruneStaleContacts(); err != nil {
					log.WithError(err).Warn("Failed to prune stale contacts")
				} else if n > 0 {
					log.WithField("pruned", n).Info("Pruned stale SIP contacts")
				}
			}
		}
	}()
}
