package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gorilla/mux"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/database"
)

func setupAPI(t *testing.T) (*APIHandler, *database.DB) {
	t.Helper()
	dir := t.TempDir()
	db, err := database.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Migrate(); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })

	ariClient := ari.NewClient("http://localhost:8088", "user", "pass")
	return NewAPIHandler(db, ariClient), db
}

func doRequest(handler http.HandlerFunc, method, path string, body any, vars map[string]string) *httptest.ResponseRecorder {
	var reqBody *bytes.Buffer
	if body != nil {
		b, _ := json.Marshal(body)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = &bytes.Buffer{}
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	if vars != nil {
		req = mux.SetURLVars(req, vars)
	}
	w := httptest.NewRecorder()
	handler.ServeHTTP(w, req)
	return w
}

// --- Trunk API tests ---

func TestAPITrunks_CRUD(t *testing.T) {
	h, _ := setupAPI(t)

	// List — empty
	w := doRequest(h.ListTrunks, "GET", "/api/trunks", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d, want 200", w.Code)
	}
	var trunks []database.Trunk
	json.NewDecoder(w.Body).Decode(&trunks)
	if len(trunks) != 0 {
		t.Fatalf("list: got %d trunks, want 0", len(trunks))
	}

	// Create
	trunk := map[string]any{
		"name":    "test-trunk",
		"host":    "10.0.0.1",
		"port":    5080,
		"context": "from-trunk",
		"codecs":  "ulaw,alaw",
	}
	w = doRequest(h.CreateTrunk, "POST", "/api/trunks", trunk, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201. body: %s", w.Code, w.Body.String())
	}
	var created database.Trunk
	json.NewDecoder(w.Body).Decode(&created)
	if created.Name != "test-trunk" {
		t.Errorf("create: name = %q, want %q", created.Name, "test-trunk")
	}
	if created.ID == 0 {
		t.Error("create: ID should be set")
	}

	// Get
	w = doRequest(h.GetTrunk, "GET", "/api/trunks/1", nil, map[string]string{"id": "1"})
	if w.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", w.Code)
	}
	var got database.Trunk
	json.NewDecoder(w.Body).Decode(&got)
	if got.Host != "10.0.0.1" {
		t.Errorf("get: host = %q, want %q", got.Host, "10.0.0.1")
	}

	// Update
	update := map[string]any{
		"name":    "test-trunk",
		"host":    "10.0.0.2",
		"port":    5090,
		"context": "from-trunk",
	}
	w = doRequest(h.UpdateTrunk, "PUT", "/api/trunks/1", update, map[string]string{"id": "1"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: got %d, want 200. body: %s", w.Code, w.Body.String())
	}

	// Delete
	w = doRequest(h.DeleteTrunk, "DELETE", "/api/trunks/1", nil, map[string]string{"id": "1"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", w.Code)
	}

	// Get after delete — 404
	w = doRequest(h.GetTrunk, "GET", "/api/trunks/1", nil, map[string]string{"id": "1"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: got %d, want 404", w.Code)
	}
}

func TestAPITrunks_CreateValidation(t *testing.T) {
	h, _ := setupAPI(t)

	// Missing required fields
	w := doRequest(h.CreateTrunk, "POST", "/api/trunks", map[string]string{"name": "t"}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("validation: got %d, want 400", w.Code)
	}

	// Invalid JSON
	req := httptest.NewRequest("POST", "/api/trunks", bytes.NewBufferString("{bad"))
	req.Header.Set("Content-Type", "application/json")
	w2 := httptest.NewRecorder()
	h.CreateTrunk(w2, req)
	if w2.Code != http.StatusBadRequest {
		t.Errorf("bad json: got %d, want 400", w2.Code)
	}
}

// --- Dialplan API tests ---

func TestAPIDialplan_CRUD(t *testing.T) {
	h, _ := setupAPI(t)

	// List — empty
	w := doRequest(h.ListDialplanRules, "GET", "/api/dialplan", nil, nil)
	if w.Code != http.StatusOK {
		t.Fatalf("list: got %d, want 200", w.Code)
	}
	var rules []database.DialplanRule
	json.NewDecoder(w.Body).Decode(&rules)
	if len(rules) != 0 {
		t.Fatalf("list: got %d rules, want 0", len(rules))
	}

	// Create
	rule := map[string]any{
		"context":  "from-internal",
		"exten":    "_3XXX",
		"priority": 1,
		"app":      "Dial",
		"appdata":  "PJSIP/${EXTEN}@my-trunk,30",
	}
	w = doRequest(h.CreateDialplanRule, "POST", "/api/dialplan", rule, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201. body: %s", w.Code, w.Body.String())
	}
	var created database.DialplanRule
	json.NewDecoder(w.Body).Decode(&created)
	if created.App != "Dial" {
		t.Errorf("create: app = %q, want %q", created.App, "Dial")
	}
	if created.ID == 0 {
		t.Error("create: ID should be set")
	}

	// Get
	w = doRequest(h.GetDialplanRule, "GET", "/api/dialplan/1", nil, map[string]string{"id": "1"})
	if w.Code != http.StatusOK {
		t.Fatalf("get: got %d, want 200", w.Code)
	}
	var got database.DialplanRule
	json.NewDecoder(w.Body).Decode(&got)
	if got.Exten != "_3XXX" {
		t.Errorf("get: exten = %q, want %q", got.Exten, "_3XXX")
	}

	// Update
	update := map[string]any{
		"context":  "from-internal",
		"exten":    "_3XXX",
		"priority": 1,
		"app":      "NoOp",
		"appdata":  "Updated rule",
	}
	w = doRequest(h.UpdateDialplanRule, "PUT", "/api/dialplan/1", update, map[string]string{"id": "1"})
	if w.Code != http.StatusOK {
		t.Fatalf("update: got %d, want 200. body: %s", w.Code, w.Body.String())
	}

	// Verify update
	w = doRequest(h.GetDialplanRule, "GET", "/api/dialplan/1", nil, map[string]string{"id": "1"})
	json.NewDecoder(w.Body).Decode(&got)
	if got.App != "NoOp" {
		t.Errorf("after update: app = %q, want %q", got.App, "NoOp")
	}

	// Delete
	w = doRequest(h.DeleteDialplanRule, "DELETE", "/api/dialplan/1", nil, map[string]string{"id": "1"})
	if w.Code != http.StatusNoContent {
		t.Fatalf("delete: got %d, want 204", w.Code)
	}

	// Get after delete — 404
	w = doRequest(h.GetDialplanRule, "GET", "/api/dialplan/1", nil, map[string]string{"id": "1"})
	if w.Code != http.StatusNotFound {
		t.Fatalf("get after delete: got %d, want 404", w.Code)
	}
}

func TestAPIDialplan_CreateValidation(t *testing.T) {
	h, _ := setupAPI(t)

	// Missing required fields
	w := doRequest(h.CreateDialplanRule, "POST", "/api/dialplan", map[string]string{"context": "x"}, nil)
	if w.Code != http.StatusBadRequest {
		t.Errorf("validation: got %d, want 400", w.Code)
	}
}

func TestAPIDialplan_DefaultPriority(t *testing.T) {
	h, _ := setupAPI(t)

	rule := map[string]any{
		"context": "from-internal",
		"exten":   "1001",
		"app":     "NoOp",
	}
	w := doRequest(h.CreateDialplanRule, "POST", "/api/dialplan", rule, nil)
	if w.Code != http.StatusCreated {
		t.Fatalf("create: got %d, want 201", w.Code)
	}
	var created database.DialplanRule
	json.NewDecoder(w.Body).Decode(&created)
	if created.Priority != 1 {
		t.Errorf("priority = %d, want 1", created.Priority)
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
