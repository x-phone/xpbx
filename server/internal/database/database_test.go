package database

import (
	"os"
	"path/filepath"
	"testing"
)

// testDB creates a temporary in-memory-like SQLite database for testing.
func testDB(t *testing.T) *DB {
	t.Helper()
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "test.db")
	db, err := Open(dbPath)
	if err != nil {
		t.Fatalf("open test db: %v", err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatalf("migrate test db: %v", err)
	}
	t.Cleanup(func() { db.Close() })
	return db
}

func TestOpenAndMigrate(t *testing.T) {
	db := testDB(t)

	// Verify tables exist by querying them
	tables := []string{"ps_endpoints", "ps_auths", "ps_aors", "ps_contacts", "extensions", "pbx_trunks", "voicemail_settings"}
	for _, table := range tables {
		var count int
		err := db.QueryRow("SELECT COUNT(*) FROM " + table).Scan(&count)
		if err != nil {
			t.Errorf("table %s should exist: %v", table, err)
		}
	}
}

func TestMigrateIdempotent(t *testing.T) {
	db := testDB(t)
	// Running migrate again should not error
	if err := db.Migrate(); err != nil {
		t.Errorf("second migrate should succeed: %v", err)
	}
}

func TestCheckpoint(t *testing.T) {
	db := testDB(t)
	// Should not panic or error
	db.Checkpoint()
}

// --- Extension CRUD ---

func TestExtensionCRUD(t *testing.T) {
	db := testDB(t)

	ext := &Extension{
		Extension:   "2001",
		DisplayName: "Test User",
		Password:    "secret123",
		Context:     "from-internal",
		Transport:   "transport-udp",
		Codecs:      "ulaw",
		MaxContacts: 5,
	}

	// Create
	if err := db.CreateExtension(ext); err != nil {
		t.Fatalf("create extension: %v", err)
	}

	// Verify ps_endpoints
	var epID string
	err := db.QueryRow("SELECT id FROM ps_endpoints WHERE id=? AND endpoint_type='extension'", "2001").Scan(&epID)
	if err != nil {
		t.Errorf("ps_endpoints row missing: %v", err)
	}

	// Verify ps_auths
	var authPass string
	err = db.QueryRow("SELECT password FROM ps_auths WHERE id=?", "2001").Scan(&authPass)
	if err != nil {
		t.Errorf("ps_auths row missing: %v", err)
	}
	if authPass != "secret123" {
		t.Errorf("password = %q, want %q", authPass, "secret123")
	}

	// Verify ps_aors
	var maxContacts string
	err = db.QueryRow("SELECT max_contacts FROM ps_aors WHERE id=?", "2001").Scan(&maxContacts)
	if err != nil {
		t.Errorf("ps_aors row missing: %v", err)
	}
	if maxContacts != "5" {
		t.Errorf("max_contacts = %q, want %q", maxContacts, "5")
	}

	// Get
	got, err := db.GetExtension("2001")
	if err != nil {
		t.Fatalf("get extension: %v", err)
	}
	if got == nil {
		t.Fatal("get extension returned nil")
	}
	if got.DisplayName != "Test User" {
		t.Errorf("display_name = %q, want %q", got.DisplayName, "Test User")
	}

	// List
	exts, err := db.ListExtensions()
	if err != nil {
		t.Fatalf("list extensions: %v", err)
	}
	if len(exts) != 1 {
		t.Errorf("list returned %d extensions, want 1", len(exts))
	}

	// Update
	ext.DisplayName = "Updated User"
	ext.Password = "newpass"
	ext.MaxContacts = 10
	if err := db.UpdateExtension(ext); err != nil {
		t.Fatalf("update extension: %v", err)
	}
	got, _ = db.GetExtension("2001")
	if got.DisplayName != "Updated User" {
		t.Errorf("after update, display_name = %q, want %q", got.DisplayName, "Updated User")
	}

	// Delete
	if err := db.DeleteExtension("2001"); err != nil {
		t.Fatalf("delete extension: %v", err)
	}
	got, err = db.GetExtension("2001")
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Error("extension should be nil after delete")
	}
}

func TestGetExtension_NotFound(t *testing.T) {
	db := testDB(t)
	ext, err := db.GetExtension("nonexistent")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if ext != nil {
		t.Error("expected nil for nonexistent extension")
	}
}

// --- Trunk CRUD ---

func TestTrunkCRUD(t *testing.T) {
	db := testDB(t)

	trunk := &Trunk{
		Name:        "test-trunk",
		DisplayName: "Test Trunk",
		Provider:    "telnyx",
		Host:        "sip.telnyx.com",
		Port:        5060,
		Context:     "from-trunk",
		Transport:   "transport-udp",
		Codecs:      "ulaw",
		AuthUser:    "myuser",
		AuthPass:    "mypass",
	}

	// Create
	if err := db.CreateTrunk(trunk); err != nil {
		t.Fatalf("create trunk: %v", err)
	}
	if trunk.ID == 0 {
		t.Error("trunk ID should be set after create")
	}

	// Verify ps_endpoints with endpoint_type=trunk
	var epType string
	err := db.QueryRow("SELECT endpoint_type FROM ps_endpoints WHERE id=?", "test-trunk").Scan(&epType)
	if err != nil {
		t.Errorf("ps_endpoints row missing: %v", err)
	}
	if epType != "trunk" {
		t.Errorf("endpoint_type = %q, want %q", epType, "trunk")
	}

	// Verify ps_aors with static contact
	var contact string
	err = db.QueryRow("SELECT contact FROM ps_aors WHERE id=?", "test-trunk-aor").Scan(&contact)
	if err != nil {
		t.Errorf("ps_aors row missing: %v", err)
	}
	if contact != "sip:sip.telnyx.com:5060" {
		t.Errorf("contact = %q, want %q", contact, "sip:sip.telnyx.com:5060")
	}

	// Verify ps_auths
	var authUser string
	err = db.QueryRow("SELECT username FROM ps_auths WHERE id=?", "test-trunk").Scan(&authUser)
	if err != nil {
		t.Errorf("ps_auths row missing: %v", err)
	}
	if authUser != "myuser" {
		t.Errorf("auth_user = %q, want %q", authUser, "myuser")
	}

	// Get
	got, err := db.GetTrunk(trunk.ID)
	if err != nil {
		t.Fatalf("get trunk: %v", err)
	}
	if got.Name != "test-trunk" {
		t.Errorf("name = %q, want %q", got.Name, "test-trunk")
	}

	// List
	trunks, err := db.ListTrunks()
	if err != nil {
		t.Fatalf("list trunks: %v", err)
	}
	if len(trunks) != 1 {
		t.Errorf("list returned %d trunks, want 1", len(trunks))
	}

	// Update
	trunk.Host = "sip2.telnyx.com"
	trunk.Port = 5080
	if err := db.UpdateTrunk(trunk); err != nil {
		t.Fatalf("update trunk: %v", err)
	}
	err = db.QueryRow("SELECT contact FROM ps_aors WHERE id=?", "test-trunk-aor").Scan(&contact)
	if err != nil {
		t.Fatalf("get updated contact: %v", err)
	}
	if contact != "sip:sip2.telnyx.com:5080" {
		t.Errorf("updated contact = %q, want %q", contact, "sip:sip2.telnyx.com:5080")
	}

	// Delete
	if err := db.DeleteTrunk(trunk.ID); err != nil {
		t.Fatalf("delete trunk: %v", err)
	}
	got, err = db.GetTrunk(trunk.ID)
	if err != nil {
		t.Fatalf("get after delete: %v", err)
	}
	if got != nil {
		t.Error("trunk should be nil after delete")
	}
}

func TestTrunkCreateWithoutAuth(t *testing.T) {
	db := testDB(t)

	trunk := &Trunk{
		Name:      "noauth-trunk",
		Host:      "192.168.1.1",
		Port:      5060,
		Context:   "from-trunk",
		Transport: "transport-udp",
		Codecs:    "ulaw",
	}

	if err := db.CreateTrunk(trunk); err != nil {
		t.Fatalf("create trunk without auth: %v", err)
	}

	// Verify no ps_auths row
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ps_auths WHERE id=?", "noauth-trunk").Scan(&count)
	if count != 0 {
		t.Errorf("expected 0 ps_auths rows, got %d", count)
	}

	// Verify endpoint has empty auth
	var auth string
	db.QueryRow("SELECT auth FROM ps_endpoints WHERE id=?", "noauth-trunk").Scan(&auth)
	if auth != "" {
		t.Errorf("auth = %q, want empty", auth)
	}
}

// --- Dialplan CRUD ---

func TestDialplanCRUD(t *testing.T) {
	db := testDB(t)

	rule := &DialplanRule{
		Context:  "from-internal",
		Exten:    "1001",
		Priority: 1,
		App:      "Dial",
		AppData:  "PJSIP/1001,20",
	}

	// Create
	if err := db.CreateDialplanRule(rule); err != nil {
		t.Fatalf("create rule: %v", err)
	}
	if rule.ID == 0 {
		t.Error("rule ID should be set after create")
	}

	// Get
	got, err := db.GetDialplanRule(rule.ID)
	if err != nil {
		t.Fatalf("get rule: %v", err)
	}
	if got.App != "Dial" {
		t.Errorf("app = %q, want %q", got.App, "Dial")
	}

	// List
	rules, err := db.ListDialplanRules()
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 1 {
		t.Errorf("list returned %d rules, want 1", len(rules))
	}

	// Update
	rule.AppData = "PJSIP/1001,30"
	if err := db.UpdateDialplanRule(rule); err != nil {
		t.Fatalf("update rule: %v", err)
	}
	got, _ = db.GetDialplanRule(rule.ID)
	if got.AppData != "PJSIP/1001,30" {
		t.Errorf("appdata = %q, want %q", got.AppData, "PJSIP/1001,30")
	}

	// Delete
	if err := db.DeleteDialplanRule(rule.ID); err != nil {
		t.Fatalf("delete rule: %v", err)
	}
	got, _ = db.GetDialplanRule(rule.ID)
	if got != nil {
		t.Error("rule should be nil after delete")
	}
}

func TestEnsureRouting_RingOnly(t *testing.T) {
	db := testDB(t)

	if err := db.EnsureRouting("from-internal", "1001", PatternRingOnly, 25); err != nil {
		t.Fatalf("ensure routing: %v", err)
	}

	rules, err := db.GetDialplanRulesForExten("from-internal", "1001")
	if err != nil {
		t.Fatalf("get rules: %v", err)
	}
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}

	if rules[0].App != "NoOp" {
		t.Errorf("rule 1 app = %q, want NoOp", rules[0].App)
	}
	if rules[1].App != "Dial" {
		t.Errorf("rule 2 app = %q, want Dial", rules[1].App)
	}
	if rules[1].AppData != "PJSIP/1001,25" {
		t.Errorf("rule 2 appdata = %q, want PJSIP/1001,25", rules[1].AppData)
	}
	if rules[2].App != "Hangup" {
		t.Errorf("rule 3 app = %q, want Hangup", rules[2].App)
	}
}

func TestEnsureRouting_RingVoicemail(t *testing.T) {
	db := testDB(t)

	if err := db.EnsureRouting("from-internal", "1001", PatternRingVoicemail, 20); err != nil {
		t.Fatalf("ensure routing: %v", err)
	}

	rules, _ := db.GetDialplanRulesForExten("from-internal", "1001")
	if len(rules) != 4 {
		t.Fatalf("expected 4 rules, got %d", len(rules))
	}
	if rules[2].App != "VoiceMail" {
		t.Errorf("rule 3 app = %q, want VoiceMail", rules[2].App)
	}
	if rules[2].AppData != "1001@default,u" {
		t.Errorf("rule 3 appdata = %q, want 1001@default,u", rules[2].AppData)
	}
}

func TestEnsureRouting_VoicemailOnly(t *testing.T) {
	db := testDB(t)

	if err := db.EnsureRouting("from-internal", "1001", PatternVoicemailOnly, 0); err != nil {
		t.Fatalf("ensure routing: %v", err)
	}

	rules, _ := db.GetDialplanRulesForExten("from-internal", "1001")
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[1].App != "VoiceMail" {
		t.Errorf("rule 2 app = %q, want VoiceMail", rules[1].App)
	}
}

func TestEnsureRouting_ReplacesExisting(t *testing.T) {
	db := testDB(t)

	// Set ring-only first
	db.EnsureRouting("from-internal", "1001", PatternRingOnly, 20)
	// Switch to voicemail-only
	db.EnsureRouting("from-internal", "1001", PatternVoicemailOnly, 0)

	rules, _ := db.GetDialplanRulesForExten("from-internal", "1001")
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules after replace, got %d", len(rules))
	}
	if rules[1].App != "VoiceMail" {
		t.Errorf("should be voicemail-only after replace, got app = %q", rules[1].App)
	}
}

func TestEnsureTrunkRoute(t *testing.T) {
	db := testDB(t)

	if err := db.EnsureTrunkRoute("from-internal", "_2XXX", "my-trunk", 30); err != nil {
		t.Fatalf("ensure trunk route: %v", err)
	}

	rules, _ := db.GetDialplanRulesForExten("from-internal", "_2XXX")
	if len(rules) != 3 {
		t.Fatalf("expected 3 rules, got %d", len(rules))
	}
	if rules[1].AppData != "PJSIP/${EXTEN}@my-trunk,30" {
		t.Errorf("trunk route appdata = %q, want PJSIP/${EXTEN}@my-trunk,30", rules[1].AppData)
	}
}

func TestDeleteDialplanRulesForExten(t *testing.T) {
	db := testDB(t)

	db.EnsureRouting("from-internal", "1001", PatternRingOnly, 20)
	db.EnsureRouting("from-internal", "1002", PatternRingOnly, 20)

	if err := db.DeleteDialplanRulesForExten("from-internal", "1001"); err != nil {
		t.Fatalf("delete rules for exten: %v", err)
	}

	// 1001 rules should be gone
	rules, _ := db.GetDialplanRulesForExten("from-internal", "1001")
	if len(rules) != 0 {
		t.Errorf("expected 0 rules for 1001, got %d", len(rules))
	}
	// 1002 rules should remain
	rules, _ = db.GetDialplanRulesForExten("from-internal", "1002")
	if len(rules) != 3 {
		t.Errorf("expected 3 rules for 1002, got %d", len(rules))
	}
}

// --- Voicemail ---

func TestVoicemailSettings(t *testing.T) {
	db := testDB(t)

	// Get defaults for nonexistent
	vm, err := db.GetVoicemailSettings("1001")
	if err != nil {
		t.Fatalf("get defaults: %v", err)
	}
	if !vm.Enabled {
		t.Error("default should be enabled")
	}
	if vm.PIN != "0000" {
		t.Errorf("default PIN = %q, want %q", vm.PIN, "0000")
	}

	// Upsert
	vm.PIN = "1234"
	vm.Email = "test@example.com"
	if err := db.UpsertVoicemailSettings(vm); err != nil {
		t.Fatalf("upsert: %v", err)
	}

	// Get after upsert
	vm, _ = db.GetVoicemailSettings("1001")
	if vm.PIN != "1234" {
		t.Errorf("PIN = %q, want %q", vm.PIN, "1234")
	}
	if vm.Email != "test@example.com" {
		t.Errorf("email = %q, want %q", vm.Email, "test@example.com")
	}

	// Update via upsert
	vm.Enabled = false
	db.UpsertVoicemailSettings(vm)
	vm, _ = db.GetVoicemailSettings("1001")
	if vm.Enabled {
		t.Error("should be disabled after update")
	}

	// Delete
	if err := db.DeleteVoicemailSettings("1001"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	vm, _ = db.GetVoicemailSettings("1001")
	if vm.PIN != "0000" {
		t.Errorf("after delete, should return defaults, PIN = %q", vm.PIN)
	}
}

func TestSyncVoicemailMailboxes(t *testing.T) {
	db := testDB(t)

	// Create an extension
	ext := &Extension{
		Extension:   "3001",
		DisplayName: "VM Test",
		Password:    "pass",
		Context:     "from-internal",
		Transport:   "transport-udp",
		Codecs:      "ulaw",
		MaxContacts: 5,
	}
	db.CreateExtension(ext)

	// Set voicemail settings with email
	db.UpsertVoicemailSettings(&VoicemailSettings{
		Extension: "3001",
		Enabled:   true,
		PIN:       "5678",
		Email:     "vm@test.com",
	})

	dir := t.TempDir()
	if err := db.SyncVoicemailMailboxes(dir); err != nil {
		t.Fatalf("sync: %v", err)
	}

	// Read generated file
	data, err := os.ReadFile(filepath.Join(dir, "voicemail_mailboxes.conf"))
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	content := string(data)

	if !contains(content, "3001 => 5678,VM Test,vm@test.com,,attach=yes") {
		t.Errorf("expected mailbox line in:\n%s", content)
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// --- Contact Cleanup ---

func TestPruneStaleContacts(t *testing.T) {
	db := testDB(t)

	// Insert a stale contact (expired 1 hour ago)
	staleTime := "1000000"
	db.Exec(`INSERT INTO ps_contacts (id, uri, expiration_time, endpoint) VALUES (?, ?, ?, ?)`,
		"stale-contact", "sip:1001@192.168.1.1", staleTime, "1001")

	// Insert a valid contact (expires far in the future)
	futureTime := "9999999999"
	db.Exec(`INSERT INTO ps_contacts (id, uri, expiration_time, endpoint) VALUES (?, ?, ?, ?)`,
		"valid-contact", "sip:1002@192.168.1.2", futureTime, "1002")

	pruned, err := db.PruneStaleContacts()
	if err != nil {
		t.Fatalf("prune: %v", err)
	}
	if pruned != 1 {
		t.Errorf("pruned = %d, want 1", pruned)
	}

	// Valid contact should remain
	var count int
	db.QueryRow("SELECT COUNT(*) FROM ps_contacts").Scan(&count)
	if count != 1 {
		t.Errorf("remaining contacts = %d, want 1", count)
	}
}

// --- Seed ---

func TestSeed(t *testing.T) {
	db := testDB(t)

	if err := db.Seed(); err != nil {
		t.Fatalf("seed: %v", err)
	}

	// Check extensions created
	exts, err := db.ListExtensions()
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(exts) != 3 {
		t.Errorf("seed created %d extensions, want 3", len(exts))
	}

	// Check dialplan rules created:
	// 3 per extension in from-internal = 9, plus 3 per extension in from-trunk = 9,
	// plus 3 outbound trunk route = 21
	rules, err := db.ListDialplanRules()
	if err != nil {
		t.Fatalf("list rules: %v", err)
	}
	if len(rules) != 21 {
		t.Errorf("seed created %d rules, want 21", len(rules))
	}

	// Check trunk created
	trunks, err := db.ListTrunks()
	if err != nil {
		t.Fatalf("list trunks: %v", err)
	}
	if len(trunks) != 1 {
		t.Errorf("seed created %d trunks, want 1", len(trunks))
	}

	// Seed again — should be idempotent
	if err := db.Seed(); err != nil {
		t.Fatalf("second seed: %v", err)
	}
	exts, _ = db.ListExtensions()
	if len(exts) != 3 {
		t.Errorf("after second seed: %d extensions, want 3", len(exts))
	}
}
