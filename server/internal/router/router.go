package router

import (
	"net/http"

	"github.com/gorilla/mux"
	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/config"
	"github.com/x-phone/xpbx/server/internal/database"
	"github.com/x-phone/xpbx/server/internal/handlers"
)

func New(db *database.DB, ariClient *ari.Client, cfg *config.Config) http.Handler {
	r := mux.NewRouter()

	// Handlers
	extH := handlers.NewExtensionHandler(db, ariClient, cfg.DataDir)
	trunkH := handlers.NewTrunkHandler(db, ariClient)
	dpH := handlers.NewDialplanHandler(db, ariClient)
	dashH := handlers.NewDashboardHandler(ariClient, db, cfg.HostIPs, cfg.SIPPort)

	// Root redirect
	r.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/dashboard", http.StatusFound)
	})

	// Dashboard
	r.HandleFunc("/dashboard", dashH.Dashboard).Methods("GET")

	// htmx partials — dashboard polling
	r.HandleFunc("/partials/system-info", dashH.SystemInfo).Methods("GET")
	r.HandleFunc("/partials/registrations", dashH.Registrations).Methods("GET")
	r.HandleFunc("/partials/active-calls", dashH.ActiveCalls).Methods("GET")
	r.HandleFunc("/partials/sip-config/{id}", dashH.SIPConfig).Methods("GET")

	// Extensions
	r.HandleFunc("/extensions", extH.List).Methods("GET")
	r.HandleFunc("/extensions/new", extH.NewForm).Methods("GET")
	r.HandleFunc("/extensions/{id}/edit", extH.EditForm).Methods("GET")
	r.HandleFunc("/extensions", extH.Create).Methods("POST")
	r.HandleFunc("/extensions/{id}", extH.Update).Methods("PUT")
	r.HandleFunc("/extensions/{id}", extH.Delete).Methods("DELETE")

	// Trunks
	r.HandleFunc("/trunks", trunkH.List).Methods("GET")
	r.HandleFunc("/trunks/new", trunkH.NewForm).Methods("GET")
	r.HandleFunc("/trunks/{id}/edit", trunkH.EditForm).Methods("GET")
	r.HandleFunc("/trunks", trunkH.Create).Methods("POST")
	r.HandleFunc("/trunks/{id}", trunkH.Update).Methods("PUT")
	r.HandleFunc("/trunks/{id}", trunkH.Delete).Methods("DELETE")

	// Dialplan
	r.HandleFunc("/dialplan", dpH.List).Methods("GET")
	r.HandleFunc("/dialplan/new", dpH.NewForm).Methods("GET")
	r.HandleFunc("/dialplan/{id}/edit", dpH.EditForm).Methods("GET")
	r.HandleFunc("/dialplan", dpH.Create).Methods("POST")
	r.HandleFunc("/dialplan/{id}", dpH.Update).Methods("PUT")
	r.HandleFunc("/dialplan/{id}", dpH.Delete).Methods("DELETE")

	// API actions
	r.PathPrefix("/api/calls/").HandlerFunc(dashH.HangupCall).Methods("DELETE")
	r.HandleFunc("/api/asterisk/reload", dashH.ReloadPJSIP).Methods("POST")

	// Static files
	r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("static"))))

	// Logging middleware
	r.Use(loggingMiddleware)

	return r
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		log.WithFields(log.Fields{
			"method": r.Method,
			"path":   r.URL.Path,
		}).Debug("Request")
		next.ServeHTTP(w, r)
	})
}
