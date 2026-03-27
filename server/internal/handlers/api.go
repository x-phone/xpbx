package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/database"
)

const maxRequestBody = 1 << 20 // 1 MB

type APIHandler struct {
	db  *database.DB
	ari *ari.Client
}

func NewAPIHandler(db *database.DB, ariClient *ari.Client) *APIHandler {
	return &APIHandler{db: db, ari: ariClient}
}

func (h *APIHandler) notifyAsterisk() {
	h.db.Checkpoint()
	if err := h.ari.RestartModule("res_config_sqlite3.so"); err != nil {
		log.WithError(err).Warn("Failed to reload Asterisk SQLite module")
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// sanitizeTrunk clears sensitive fields before sending a trunk in a response.
func sanitizeTrunk(t *database.Trunk) {
	t.AuthPass = ""
}

// --- Trunks ---

func (h *APIHandler) ListTrunks(w http.ResponseWriter, r *http.Request) {
	trunks, err := h.db.ListTrunks()
	if err != nil {
		log.WithError(err).Error("API: failed to list trunks")
		writeError(w, http.StatusInternalServerError, "failed to list trunks")
		return
	}
	if trunks == nil {
		trunks = []database.Trunk{}
	}
	for i := range trunks {
		sanitizeTrunk(&trunks[i])
	}
	writeJSON(w, http.StatusOK, trunks)
}

func (h *APIHandler) GetTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk ID")
		return
	}
	t, err := h.db.GetTrunk(id)
	if err != nil {
		log.WithError(err).Error("API: failed to get trunk")
		writeError(w, http.StatusInternalServerError, "failed to get trunk")
		return
	}
	if t == nil {
		writeError(w, http.StatusNotFound, "trunk not found")
		return
	}
	sanitizeTrunk(t)
	writeJSON(w, http.StatusOK, t)
}

func (h *APIHandler) CreateTrunk(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var t database.Trunk
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if t.Name == "" || t.Host == "" || t.Context == "" {
		writeError(w, http.StatusBadRequest, "name, host, and context are required")
		return
	}
	if t.Port == 0 {
		t.Port = 5060
	}
	if t.Codecs == "" {
		t.Codecs = "ulaw"
	}
	t.Transport = "transport-udp"

	if err := h.db.CreateTrunk(&t); err != nil {
		log.WithError(err).Error("API: failed to create trunk")
		writeError(w, http.StatusInternalServerError, "failed to create trunk")
		return
	}

	h.notifyAsterisk()
	log.WithField("trunk", t.Name).Info("API: trunk created")
	sanitizeTrunk(&t)
	writeJSON(w, http.StatusCreated, t)
}

func (h *APIHandler) UpdateTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var t database.Trunk
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	t.ID = id
	if t.Codecs == "" {
		t.Codecs = "ulaw"
	}
	t.Transport = "transport-udp"

	if err := h.db.UpdateTrunk(&t); err != nil {
		log.WithError(err).Error("API: failed to update trunk")
		writeError(w, http.StatusInternalServerError, "failed to update trunk")
		return
	}

	h.notifyAsterisk()
	log.WithField("trunk", t.Name).Info("API: trunk updated")
	sanitizeTrunk(&t)
	writeJSON(w, http.StatusOK, t)
}

func (h *APIHandler) DeleteTrunk(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid trunk ID")
		return
	}
	if err := h.db.DeleteTrunk(id); err != nil {
		log.WithError(err).Error("API: failed to delete trunk")
		writeError(w, http.StatusInternalServerError, "failed to delete trunk")
		return
	}

	h.notifyAsterisk()
	log.WithField("trunk_id", id).Info("API: trunk deleted")
	w.WriteHeader(http.StatusNoContent)
}

// --- Dialplan ---

func (h *APIHandler) ListDialplanRules(w http.ResponseWriter, r *http.Request) {
	rules, err := h.db.ListDialplanRules()
	if err != nil {
		log.WithError(err).Error("API: failed to list dialplan rules")
		writeError(w, http.StatusInternalServerError, "failed to list dialplan rules")
		return
	}
	if rules == nil {
		rules = []database.DialplanRule{}
	}
	writeJSON(w, http.StatusOK, rules)
}

func (h *APIHandler) GetDialplanRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}
	rule, err := h.db.GetDialplanRule(id)
	if err != nil {
		log.WithError(err).Error("API: failed to get dialplan rule")
		writeError(w, http.StatusInternalServerError, "failed to get dialplan rule")
		return
	}
	if rule == nil {
		writeError(w, http.StatusNotFound, "rule not found")
		return
	}
	writeJSON(w, http.StatusOK, rule)
}

func (h *APIHandler) CreateDialplanRule(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var rule database.DialplanRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}

	if rule.Context == "" || rule.Exten == "" || rule.App == "" {
		writeError(w, http.StatusBadRequest, "context, exten, and app are required")
		return
	}
	if rule.Priority == 0 {
		rule.Priority = 1
	}

	if err := h.db.CreateDialplanRule(&rule); err != nil {
		log.WithError(err).Error("API: failed to create dialplan rule")
		writeError(w, http.StatusInternalServerError, "failed to create rule")
		return
	}

	h.notifyAsterisk()
	log.WithFields(log.Fields{"context": rule.Context, "exten": rule.Exten}).Info("API: dialplan rule created")
	writeJSON(w, http.StatusCreated, rule)
}

func (h *APIHandler) UpdateDialplanRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}

	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	var rule database.DialplanRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return
	}
	rule.ID = id
	if rule.Priority == 0 {
		rule.Priority = 1
	}

	if err := h.db.UpdateDialplanRule(&rule); err != nil {
		log.WithError(err).Error("API: failed to update dialplan rule")
		writeError(w, http.StatusInternalServerError, "failed to update rule")
		return
	}

	h.notifyAsterisk()
	log.WithField("id", id).Info("API: dialplan rule updated")
	writeJSON(w, http.StatusOK, rule)
}

func (h *APIHandler) DeleteDialplanRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid rule ID")
		return
	}
	if err := h.db.DeleteDialplanRule(id); err != nil {
		log.WithError(err).Error("API: failed to delete dialplan rule")
		writeError(w, http.StatusInternalServerError, "failed to delete rule")
		return
	}

	h.notifyAsterisk()
	log.WithField("id", id).Info("API: dialplan rule deleted")
	w.WriteHeader(http.StatusNoContent)
}
