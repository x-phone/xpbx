package handlers

import (
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/database"
	"github.com/x-phone/xpbx/server/templates/pages"
)

type TrunkHandler struct {
	db  *database.DB
	ari *ari.Client
}

func NewTrunkHandler(db *database.DB, ariClient *ari.Client) *TrunkHandler {
	return &TrunkHandler{db: db, ari: ariClient}
}

func (h *TrunkHandler) notifyAsterisk() {
	h.db.Checkpoint()
	if err := h.ari.RestartModule("res_config_sqlite3.so"); err != nil {
		log.WithError(err).Warn("Failed to reload Asterisk SQLite module")
	}
}

func (h *TrunkHandler) List(w http.ResponseWriter, r *http.Request) {
	trunks, err := h.db.ListTrunks()
	if err != nil {
		log.WithError(err).Error("Failed to list trunks")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	pages.TrunksList(trunks).Render(r.Context(), w)
}

func (h *TrunkHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	t := &database.Trunk{
		Port:      5060,
		Context:   "from-trunk",
		Transport: "transport-udp",
		Codecs:    "ulaw",
	}
	pages.TrunkForm(t, false).Render(r.Context(), w)
}

func (h *TrunkHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	t, err := h.db.GetTrunk(id)
	if err != nil {
		log.WithError(err).Error("Failed to get trunk")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if t == nil {
		http.NotFound(w, r)
		return
	}
	pages.TrunkForm(t, true).Render(r.Context(), w)
}

func (h *TrunkHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	port, _ := strconv.Atoi(r.FormValue("port"))
	if port == 0 {
		port = 5060
	}

	t := &database.Trunk{
		Name:        r.FormValue("name"),
		DisplayName: r.FormValue("display_name"),
		Provider:    r.FormValue("provider"),
		Host:        r.FormValue("host"),
		Port:        port,
		Context:     r.FormValue("context"),
		Transport:   "transport-udp",
		Codecs:      r.FormValue("codecs"),
		AuthUser:    r.FormValue("auth_user"),
		AuthPass:    r.FormValue("auth_pass"),
	}

	if t.Name == "" || t.Host == "" || t.Context == "" {
		http.Error(w, "Name, host, and context are required", http.StatusBadRequest)
		return
	}
	if t.Codecs == "" {
		t.Codecs = "ulaw"
	}

	if err := h.db.CreateTrunk(t); err != nil {
		log.WithError(err).Error("Failed to create trunk")
		http.Error(w, "Failed to create trunk: "+err.Error(), http.StatusInternalServerError)
		return
	}

	h.notifyAsterisk()
	log.WithField("trunk", t.Name).Info("Trunk created")
	w.Header().Set("HX-Redirect", "/trunks")
	w.WriteHeader(http.StatusOK)
}

func (h *TrunkHandler) Update(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	port, _ := strconv.Atoi(r.FormValue("port"))
	if port == 0 {
		port = 5060
	}

	t := &database.Trunk{
		ID:          id,
		Name:        r.FormValue("name"),
		DisplayName: r.FormValue("display_name"),
		Provider:    r.FormValue("provider"),
		Host:        r.FormValue("host"),
		Port:        port,
		Context:     r.FormValue("context"),
		Transport:   "transport-udp",
		Codecs:      r.FormValue("codecs"),
		AuthUser:    r.FormValue("auth_user"),
		AuthPass:    r.FormValue("auth_pass"),
	}
	if t.Codecs == "" {
		t.Codecs = "ulaw"
	}

	if err := h.db.UpdateTrunk(t); err != nil {
		log.WithError(err).Error("Failed to update trunk")
		http.Error(w, "Failed to update trunk", http.StatusInternalServerError)
		return
	}

	h.notifyAsterisk()
	log.WithField("trunk", t.Name).Info("Trunk updated")
	w.Header().Set("HX-Redirect", "/trunks")
	w.WriteHeader(http.StatusOK)
}

func (h *TrunkHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(mux.Vars(r)["id"], 10, 64)
	if err := h.db.DeleteTrunk(id); err != nil {
		log.WithError(err).Error("Failed to delete trunk")
		http.Error(w, "Failed to delete trunk", http.StatusInternalServerError)
		return
	}

	h.notifyAsterisk()
	log.WithField("trunk_id", id).Info("Trunk deleted")
	w.WriteHeader(http.StatusOK)
}
