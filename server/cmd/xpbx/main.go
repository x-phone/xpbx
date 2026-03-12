package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/x-phone/xpbx/server/internal/ari"
	"github.com/x-phone/xpbx/server/internal/config"
	"github.com/x-phone/xpbx/server/internal/database"
	"github.com/x-phone/xpbx/server/internal/router"
)

func main() {
	log.SetFormatter(&log.TextFormatter{FullTimestamp: true})
	log.SetLevel(log.InfoLevel)
	if os.Getenv("LOG_LEVEL") == "debug" {
		log.SetLevel(log.DebugLevel)
	}

	cfg := config.Load()
	log.WithFields(log.Fields{
		"listen":   cfg.ListenAddr,
		"db":       cfg.DBPath,
		"ari_url":  cfg.ARIURL(),
		"host_ips": cfg.HostIPs,
	}).Info("Starting xpbx")

	// Database
	db, err := database.Open(cfg.DBPath)
	if err != nil {
		log.WithError(err).Fatal("Failed to open database")
	}
	defer db.Close()

	if err := db.Migrate(); err != nil {
		log.WithError(err).Fatal("Failed to run migrations")
	}

	if err := db.Seed(); err != nil {
		log.WithError(err).Fatal("Failed to seed database")
	}

	// Ensure voicemail mailboxes file exists (Asterisk #includes it)
	if err := db.SyncVoicemailMailboxes(cfg.DataDir); err != nil {
		log.WithError(err).Warn("Failed to sync voicemail mailboxes")
	}

	// ARI client
	ariClient := ari.NewClient(cfg.ARIURL(), cfg.ARIUser, cfg.ARIPassword)

	// HTTP server
	handler := router.New(db, ariClient, cfg)
	srv := &http.Server{
		Addr:         cfg.ListenAddr,
		Handler:      handler,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Graceful shutdown
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGTERM)

	go func() {
		log.WithField("addr", cfg.ListenAddr).Info("HTTP server listening")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.WithError(err).Fatal("HTTP server error")
		}
	}()

	<-done
	log.Info("Shutting down...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		log.WithError(err).Error("Server shutdown error")
	}
	log.Info("xpbx stopped")
}
