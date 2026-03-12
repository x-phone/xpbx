package database

import (
	log "github.com/sirupsen/logrus"
)

// Seed populates the database with sample extensions and dialplan rules.
// Only runs if ps_endpoints is empty (first startup).
func (db *DB) Seed() error {
	var count int
	err := db.QueryRow(`SELECT COUNT(*) FROM ps_endpoints`).Scan(&count)
	if err != nil {
		return err
	}
	if count > 0 {
		log.Info("Database already seeded, skipping")
		return nil
	}

	log.Info("Seeding database with sample extensions and dialplan...")

	// Sample extensions 1001-1003
	extensions := []Extension{
		{Extension: "1001", DisplayName: "Extension 1001", Password: "password123", Context: "from-internal", Transport: "transport-udp", Codecs: "ulaw", MaxContacts: 10},
		{Extension: "1002", DisplayName: "Extension 1002", Password: "password123", Context: "from-internal", Transport: "transport-udp", Codecs: "ulaw", MaxContacts: 10},
		{Extension: "1003", DisplayName: "Extension 1003", Password: "password123", Context: "from-internal", Transport: "transport-udp", Codecs: "ulaw", MaxContacts: 10},
	}
	for _, ext := range extensions {
		if err := db.CreateExtension(&ext); err != nil {
			return err
		}
		log.WithField("extension", ext.Extension).Info("Seeded extension")
	}

	// Dialplan rules — internal extension routing
	rules := []DialplanRule{
		{Context: "from-internal", Exten: "1001", Priority: 1, App: "NoOp", AppData: "Calling extension 1001"},
		{Context: "from-internal", Exten: "1001", Priority: 2, App: "Dial", AppData: "PJSIP/1001,20"},
		{Context: "from-internal", Exten: "1001", Priority: 3, App: "Hangup", AppData: ""},

		{Context: "from-internal", Exten: "1002", Priority: 1, App: "NoOp", AppData: "Calling extension 1002"},
		{Context: "from-internal", Exten: "1002", Priority: 2, App: "Dial", AppData: "PJSIP/1002,20"},
		{Context: "from-internal", Exten: "1002", Priority: 3, App: "Hangup", AppData: ""},

		{Context: "from-internal", Exten: "1003", Priority: 1, App: "NoOp", AppData: "Calling extension 1003"},
		{Context: "from-internal", Exten: "1003", Priority: 2, App: "Dial", AppData: "PJSIP/1003,20"},
		{Context: "from-internal", Exten: "1003", Priority: 3, App: "Hangup", AppData: ""},
	}
	for i := range rules {
		if err := db.CreateDialplanRule(&rules[i]); err != nil {
			return err
		}
	}
	log.WithField("count", len(rules)).Info("Seeded dialplan rules")

	return nil
}
