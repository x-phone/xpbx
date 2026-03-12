package handlers

import (
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/database"
	"github.com/x-phone/xpbx/server/templates/pages"
	"github.com/x-phone/xpbx/server/templates/partials"
)

type DashboardHandler struct {
	ari     *ari.Client
	db      *database.DB
	hostIPs []string
	sipPort int
}

func NewDashboardHandler(ariClient *ari.Client, db *database.DB, hostIPs []string, sipPort int) *DashboardHandler {
	return &DashboardHandler{ari: ariClient, db: db, hostIPs: hostIPs, sipPort: sipPort}
}

func (h *DashboardHandler) Dashboard(w http.ResponseWriter, r *http.Request) {
	pages.Dashboard().Render(r.Context(), w)
}

func (h *DashboardHandler) SystemInfo(w http.ResponseWriter, r *http.Request) {
	info, err := h.ari.GetInfo()
	if err != nil {
		log.WithError(err).Debug("ARI system info unavailable")
	}
	partials.SystemInfo(info).Render(r.Context(), w)
}

func (h *DashboardHandler) Registrations(w http.ResponseWriter, r *http.Request) {
	endpoints, err := h.ari.GetEndpoints()
	if err != nil {
		log.WithError(err).Debug("ARI endpoints unavailable")
		endpoints = nil
	}
	partials.Registrations(endpoints).Render(r.Context(), w)
}

func (h *DashboardHandler) ActiveCalls(w http.ResponseWriter, r *http.Request) {
	channels, err := h.ari.GetChannels()
	if err != nil {
		log.WithError(err).Debug("ARI channels unavailable")
		channels = nil
	}
	partials.ActiveCalls(channels).Render(r.Context(), w)
}

func (h *DashboardHandler) SIPConfig(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ext, err := h.db.GetExtension(id)
	if err != nil || ext == nil {
		http.Error(w, "Extension not found", http.StatusNotFound)
		return
	}
	partials.SIPConfigModal(ext, h.hostIPs, h.sipPort).Render(r.Context(), w)
}

func (h *DashboardHandler) HangupCall(w http.ResponseWriter, r *http.Request) {
	channelID := r.URL.Path[len("/api/calls/"):]
	if err := h.ari.HangupChannel(channelID); err != nil {
		log.WithError(err).Error("Failed to hangup channel")
		http.Error(w, "Failed to hangup", http.StatusInternalServerError)
		return
	}
	log.WithField("channel", channelID).Info("Channel hung up via ARI")
	w.WriteHeader(http.StatusOK)
}

func (h *DashboardHandler) ReloadPJSIP(w http.ResponseWriter, r *http.Request) {
	if err := h.ari.ReloadModule("res_pjsip.so"); err != nil {
		log.WithError(err).Error("Failed to reload PJSIP")
		http.Error(w, "Failed to reload", http.StatusInternalServerError)
		return
	}
	log.Info("PJSIP reloaded via ARI")
	w.Header().Set("HX-Trigger", `{"showToast": "PJSIP reloaded successfully"}`)
	w.WriteHeader(http.StatusOK)
}
