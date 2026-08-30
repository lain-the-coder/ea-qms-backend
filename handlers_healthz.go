package main

import (
	"net/http"

	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
)

func (cfg *apiConfig) HandlerHealthz(w http.ResponseWriter, r *http.Request) {
	type HealthResponse struct {
		Status   string `json:"status"`
		Database string `json:"database"`
	}
	log := logging.LoggerFrom(r.Context())
	// Ping database to verify live connectivity
	if err := cfg.rawDB.PingContext(r.Context()); err != nil {
		log.Error("health check failed", "reason", "database unreachable", "error", err)
		respondWithJSON(w, http.StatusServiceUnavailable, HealthResponse{
			Status:   "unhealthy",
			Database: "disconnected",
		})
		return
	}
	log.Debug("health check passed")
	respondWithJSON(w, http.StatusOK, HealthResponse{
		Status:   "ok",
		Database: "connected",
	})
}
