package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/database"
	"github.com/x-phone/xpbx/server/templates/pages"
)

type ExtensionHandler struct {
	db      *database.DB
	ari     *ari.Client
	dataDir string
}

func NewExtensionHandler(db *database.DB, ariClient *ari.Client, dataDir string) *ExtensionHandler {
	return &ExtensionHandler{db: db, ari: ariClient, dataDir: dataDir}
}

// notifyAsterisk checkpoints the WAL and reloads the SQLite module so Asterisk
// sees the latest DB changes without a container restart.
func (h *ExtensionHandler) notifyAsterisk() {
	h.db.Checkpoint()
	if err := h.ari.RestartModule("res_config_sqlite3.so"); err != nil {
		log.WithError(err).Warn("Failed to reload Asterisk SQLite module")
	}
}

func (h *ExtensionHandler) List(w http.ResponseWriter, r *http.Request) {
	exts, err := h.db.ListExtensions()
	if err != nil {
		log.WithError(err).Error("Failed to list extensions")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	// Build detail info for each extension
	var items []pages.ExtensionItem
	for _, ext := range exts {
		ri := h.detectRouting(ext.Context, ext.Extension)
		vm, _ := h.db.GetVoicemailSettings(ext.Extension)
		if vm == nil {
			vm = &database.VoicemailSettings{}
		}
		items = append(items, pages.ExtensionItem{
			Extension: ext,
			Routing:   ri,
			Voicemail: vm,
		})
	}

	pages.ExtensionsList(items).Render(r.Context(), w)
}

func (h *ExtensionHandler) NewForm(w http.ResponseWriter, r *http.Request) {
	ext := &database.Extension{
		Context:     "from-internal",
		Transport:   "transport-udp",
		Codecs:      "ulaw",
		MaxContacts: 10,
	}
	ri := pages.RoutingInfo{Enabled: true, Pattern: "ring_voicemail", Timeout: 20}
	vm := &database.VoicemailSettings{Enabled: true, PIN: "0000"}
	pages.ExtensionForm(ext, false, ri, vm).Render(r.Context(), w)
}

func (h *ExtensionHandler) EditForm(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	ext, err := h.db.GetExtension(id)
	if err != nil {
		log.WithError(err).Error("Failed to get extension")
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}
	if ext == nil {
		http.NotFound(w, r)
		return
	}

	ri := h.detectRouting(ext.Context, ext.Extension)
	vm, err := h.db.GetVoicemailSettings(id)
	if err != nil {
		log.WithError(err).Warn("Failed to get voicemail settings")
		vm = &database.VoicemailSettings{Enabled: true, PIN: "0000"}
	}
	pages.ExtensionForm(ext, true, ri, vm).Render(r.Context(), w)
}

// detectRouting checks existing dialplan rules and detects the routing pattern.
func (h *ExtensionHandler) detectRouting(context, exten string) pages.RoutingInfo {
	rules, err := h.db.GetDialplanRulesForExten(context, exten)
	if err != nil || len(rules) == 0 {
		return pages.RoutingInfo{Enabled: false, Pattern: "ring_voicemail", Timeout: 20}
	}

	timeout := 20

	// Check ring_only: NoOp(1) → Dial(2) → Hangup(3)
	if len(rules) == 3 &&
		rules[0].App == "NoOp" && rules[0].Priority == 1 &&
		rules[1].App == "Dial" && rules[1].Priority == 2 &&
		rules[2].App == "Hangup" && rules[2].Priority == 3 {
		timeout = parseDialTimeout(rules[1].AppData)
		return pages.RoutingInfo{Enabled: true, Pattern: "ring_only", Timeout: timeout, IsBasicPattern: true}
	}

	// Check ring_voicemail: NoOp(1) → Dial(2) → VoiceMail(3) → Hangup(4)
	if len(rules) == 4 &&
		rules[0].App == "NoOp" && rules[0].Priority == 1 &&
		rules[1].App == "Dial" && rules[1].Priority == 2 &&
		rules[2].App == "VoiceMail" && rules[2].Priority == 3 &&
		rules[3].App == "Hangup" && rules[3].Priority == 4 {
		timeout = parseDialTimeout(rules[1].AppData)
		return pages.RoutingInfo{Enabled: true, Pattern: "ring_voicemail", Timeout: timeout, IsBasicPattern: true}
	}

	// Check voicemail_only: NoOp(1) → VoiceMail(2) → Hangup(3)
	if len(rules) == 3 &&
		rules[0].App == "NoOp" && rules[0].Priority == 1 &&
		rules[1].App == "VoiceMail" && rules[1].Priority == 2 &&
		rules[2].App == "Hangup" && rules[2].Priority == 3 {
		return pages.RoutingInfo{Enabled: true, Pattern: "voicemail_only", Timeout: timeout, IsBasicPattern: true}
	}

	// Has rules but they're custom
	return pages.RoutingInfo{Enabled: false, Pattern: "ring_voicemail", Timeout: 20, HasCustomRules: true, RuleCount: len(rules)}
}

// parseDialTimeout extracts the timeout from Dial app data like "PJSIP/1001,20"
func parseDialTimeout(appData string) int {
	if parts := strings.Split(appData, ","); len(parts) >= 2 {
		if t, err := strconv.Atoi(strings.TrimSpace(parts[len(parts)-1])); err == nil {
			return t
		}
	}
	return 20
}

func (h *ExtensionHandler) Create(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	maxContacts, _ := strconv.Atoi(r.FormValue("max_contacts"))

	ext := &database.Extension{
		Extension:   r.FormValue("extension"),
		DisplayName: r.FormValue("display_name"),
		Password:    r.FormValue("password"),
		Context:     r.FormValue("context"),
		Transport:   "transport-udp",
		Codecs:      r.FormValue("codecs"),
		MaxContacts: maxContacts,
	}

	if ext.Extension == "" || ext.Password == "" || ext.Context == "" {
		http.Error(w, "Extension, password, and context are required", http.StatusBadRequest)
		return
	}
	if ext.Codecs == "" {
		ext.Codecs = "ulaw"
	}

	if err := h.db.CreateExtension(ext); err != nil {
		log.WithError(err).Error("Failed to create extension")
		http.Error(w, "Failed to create extension: "+err.Error(), http.StatusInternalServerError)
		return
	}
	h.notifyAsterisk()

	// Handle quick routing
	h.applyRouting(r, ext.Context, ext.Extension)

	// Handle voicemail settings
	h.applyVoicemail(r, ext.Extension)

	log.WithField("extension", ext.Extension).Info("Extension created")
	w.Header().Set("HX-Redirect", "/extensions")
	w.WriteHeader(http.StatusOK)
}

func (h *ExtensionHandler) Update(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	maxContacts, _ := strconv.Atoi(r.FormValue("max_contacts"))
	context := r.FormValue("context")

	ext := &database.Extension{
		Extension:   id,
		DisplayName: r.FormValue("display_name"),
		Password:    r.FormValue("password"),
		Context:     context,
		Transport:   "transport-udp",
		Codecs:      r.FormValue("codecs"),
		MaxContacts: maxContacts,
	}
	if ext.Codecs == "" {
		ext.Codecs = "ulaw"
	}

	if err := h.db.UpdateExtension(ext); err != nil {
		log.WithError(err).Error("Failed to update extension")
		http.Error(w, "Failed to update extension", http.StatusInternalServerError)
		return
	}
	h.notifyAsterisk()

	// Handle quick routing
	h.applyRouting(r, context, id)

	// Handle voicemail settings
	h.applyVoicemail(r, id)

	log.WithField("extension", ext.Extension).Info("Extension updated")
	w.Header().Set("HX-Redirect", "/extensions")
	w.WriteHeader(http.StatusOK)
}

// applyRouting reads routing form fields and creates/updates/removes dialplan rules.
func (h *ExtensionHandler) applyRouting(r *http.Request, context, exten string) {
	if r.FormValue("routing_enabled") == "on" {
		timeout, _ := strconv.Atoi(r.FormValue("routing_timeout"))
		if timeout <= 0 {
			timeout = 20
		}
		pattern := database.RoutingPattern(r.FormValue("routing_pattern"))
		switch pattern {
		case database.PatternRingOnly, database.PatternRingVoicemail, database.PatternVoicemailOnly:
			// valid
		default:
			pattern = database.PatternRingVoicemail
		}

		if err := h.db.EnsureRouting(context, exten, pattern, timeout); err != nil {
			log.WithError(err).Error("Failed to create routing rules")
		} else {
			log.WithFields(log.Fields{"extension": exten, "pattern": pattern}).Info("Routing rules applied")
			h.notifyAsterisk()
		}
	} else if r.FormValue("routing_remove") == "on" {
		if err := h.db.DeleteDialplanRulesForExten(context, exten); err != nil {
			log.WithError(err).Error("Failed to remove routing rules")
		} else {
			log.WithField("extension", exten).Info("Routing rules removed")
			h.notifyAsterisk()
		}
	}
}

// applyVoicemail reads voicemail form fields and saves settings.
func (h *ExtensionHandler) applyVoicemail(r *http.Request, extension string) {
	vm := &database.VoicemailSettings{
		Extension: extension,
		Enabled:   r.FormValue("vm_enabled") == "on",
		PIN:       r.FormValue("vm_pin"),
		Email:     strings.TrimSpace(r.FormValue("vm_email")),
	}
	if vm.PIN == "" {
		vm.PIN = "0000"
	}

	if err := h.db.UpsertVoicemailSettings(vm); err != nil {
		log.WithError(err).Error("Failed to save voicemail settings")
	}

	// Sync the mailboxes conf file
	h.syncVoicemail()
}

func (h *ExtensionHandler) syncVoicemail() {
	if err := h.db.SyncVoicemailMailboxes(h.dataDir); err != nil {
		log.WithError(err).Error("Failed to sync voicemail mailboxes")
	}
}

func (h *ExtensionHandler) Delete(w http.ResponseWriter, r *http.Request) {
	id := mux.Vars(r)["id"]

	// Clean up dialplan rules and voicemail settings
	ext, err := h.db.GetExtension(id)
	if err == nil && ext != nil {
		h.db.DeleteDialplanRulesForExten(ext.Context, id)
	}
	h.db.DeleteVoicemailSettings(id)

	if err := h.db.DeleteExtension(id); err != nil {
		log.WithError(err).Error("Failed to delete extension")
		http.Error(w, "Failed to delete extension", http.StatusInternalServerError)
		return
	}
	h.notifyAsterisk()

	// Sync voicemail mailboxes after deletion
	h.syncVoicemail()

	log.WithField("extension", id).Info("Extension deleted")
	w.WriteHeader(http.StatusOK)
}
