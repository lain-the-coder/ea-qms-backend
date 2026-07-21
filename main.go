package main

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/joho/godotenv"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	db       *database.Queries
	platform string
	secret   string
	params   *argon2id.Params
	rawDB    *sql.DB
	logger   *slog.Logger
}

func (cfg *apiConfig) WelcomeHome(w http.ResponseWriter, r *http.Request) {
	type WelcomeRequest struct {
		Message string `json:"message"`
	}
	type WelcomeResponse struct {
		Company string `json:"company"`
		Message string `json:"message"`
	}
	log := logging.LoggerFrom(r.Context())
	reqBody := WelcomeRequest{}
	err := json.NewDecoder(r.Body).Decode(&reqBody)
	if err != nil {
		log.Error("failed to decode request body", "error", err)
		// delegating error structuring to helper function
		respondWithError(w, "Something went wrong", http.StatusInternalServerError)
		return
	}
	reqBody.Message = strings.TrimSpace(reqBody.Message)
	if reqBody.Message == "" {
		log.Warn("message field was blank")
		respondWithError(w, "Message cannot be blank", http.StatusBadRequest)
		return
	}
	log.Info("welcome home hit", "company", "EA QMS")
	resBody := WelcomeResponse{
		Company: "EA QMS",
		Message: "Welcome! I hope you enjoy this system!",
	}
	respondWithJSON(w, http.StatusOK, resBody)
}

func main() {
	mux := http.NewServeMux()

	// build logger
	logger, err := logging.NewLogger("logs")
	if err != nil {
		// Standard log fallback since slog isn't ready if NewLogger fails
		slog.Error("failed to initialize logger", "error", err)
		os.Exit(1)
	}
	slog.SetDefault(logger)

	// load .env file
	err = godotenv.Load()
	if err != nil {
		logger.Error("error loading .env file", "error", err)
		os.Exit(1)
	}

	// load config struct with env variables
	dbURL := os.Getenv("DB_URL")
	platform := os.Getenv("PLATFORM")
	secret := os.Getenv("JWT_SECRET")
	argonParams := loadArgon2idParams()

	// db setup
	rawDB, err := sql.Open("postgres", dbURL)
	if err != nil {
		logger.Error("Database initialization failed (check driver registration or URL format)", "error", err)
		os.Exit(1)
	}
	err = rawDB.Ping()
	if err != nil {
		logger.Error("Database connection failed (check network, credentials, or server status)", "error", err)
		os.Exit(1)
	}

	db := database.New(rawDB)

	cfg := &apiConfig{
		db:       db,
		platform: platform,
		secret:   secret,
		params:   argonParams,
		rawDB:    rawDB,
		logger:   logger,
	}

	// routes
	mux.Handle("POST /", cfg.middlewareLogging(http.HandlerFunc(cfg.WelcomeHome)))
	server := &http.Server{
		Addr:    ":1304",
		Handler: mux,
	}
	logger.Error("server failed", "error", server.ListenAndServe())
	os.Exit(1)
}
