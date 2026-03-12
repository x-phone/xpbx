package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/database"
	"github.com/x-phone/xpbx/server/internal/dialplan"
	"github.com/x-phone/xpbx/server/templates/pages"
)

type DialplanHandler struct {
	db  *database.DB
	ari *ari.Client
}

func NewDialplanHandler(db *database.DB, ariClient *ari.Client) *DialplanHandler {
	return &DialplanHandler{db: db, ari: ariClient}
}

func (h *DialplanHandler) notifyAsterisk() {
	h.db.Checkpoint()
	if err := h.ari.RestartModule("res_config_sqlite3.so"); err != nil {
		log.WithError(err).Warn("Failed to reload Asterisk SQLite module")
	}
}

func (h *DialplanHandler) List(w http.ResponseWriter, r *http.Request) {
	rules, err := h.db.ListDialplanRules()
	if err != nil {
		log.WithError(err).Error("Failed to list dialplan rules")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Check if advanced mode requested
	mode := r.URL.Query().Get("mode")
	if mode == "advanced" {
		pages.DialplanAdvanced(rules).Render(r.Context(), w)
		return
	}

	// Default: friendly routing rules view
	groups := dialplan.Recognize(rules)
	pages.DialplanSimple(groups).Render(r.Context(), w)
}

func (h *DialplanHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	rule := &database.DialplanRule{
		Context:  "from-internal",
		Priority: 1,
	}
	pages.DialplanForm(rule, false).Render(r.Context(), w)
}

func (h *DialplanHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	rule, err := h.db.GetDialplanRule(id)
	if err != nil {
		log.WithError(err).Error("Failed to get dialplan rule")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if rule == nil {
		http.NotFound(w, r)
		return
	}
	pages.DialplanForm(rule, true).Render(r.Context(), w)
}

func (h *DialplanHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	priority, _ := strconv.Atoi(r.FormValue("priority"))
	if priority == 0 {
		priority = 1
	}

	rule := &database.DialplanRule{
		Context:  r.FormValue("context"),
		Exten:    r.FormValue("exten"),
		Priority: priority,
		App:      r.FormValue("app"),
		AppData:  r.FormValue("appdata"),
	}

	if rule.Context == "" || rule.Exten == "" || rule.App == "" {
		http.Error(w, "Context, extension, and application are required", http.StatusBadRequest)
		return
	}

	if err := h.db.CreateDialplanRule(rule); err != nil {
		log.WithError(err).Error("Failed to create dialplan rule")
		http.Error(w, "Failed to create rule: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.notifyAsterisk()
	log.WithFields(log.Fields{"context": rule.Context, "exten": rule.Exten}).Info("Dialplan rule created")
	w.Header().Set("HX-Redirect", "/dialplan")
	w.WriteHeader(http.StatusOK)
}

func (h *DialplanHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	priority, _ := strconv.Atoi(r.FormValue("priority"))
	if priority == 0 {
		priority = 1
	}

	rule := &database.DialplanRule{
		ID:       id,
		Context:  r.FormValue("context"),
		Exten:    r.FormValue("exten"),
		Priority: priority,
		App:      r.FormValue("app"),
		AppData:  r.FormValue("appdata"),
	}

	if err := h.db.UpdateDialplanRule(rule); err != nil {
		log.WithError(err).Error("Failed to update dialplan rule")
		http.Error(w, "Failed to update rule", http.StatusInternalServerError)
		return
	}

	h.notifyAsterisk()
	log.WithField("id", id).Info("Dialplan rule updated")
	w.Header().Set("HX-Redirect", "/dialplan")
	w.WriteHeader(http.StatusOK)
}

func (h *DialplanHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err := h.db.DeleteDialplanRule(id); err != nil {
		log.WithError(err).Error("Failed to delete dialplan rule")
		http.Error(w, "Failed to delete rule", http.StatusInternalServerError)
		return
	}

	h.notifyAsterisk()
	log.WithField("id", id).Info("Dialplan rule deleted")
	w.WriteHeader(http.StatusOK)
}
