package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"

	"github.com/alexedwards/argon2id"
	"github.com/joho/godotenv"
	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	db        *database.Queries
	platform  string
	secret    string
	params    *argon2id.Params
	rawDB     *sql.DB
	logger    *slog.Logger
	dummyHash string
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
	// A throwaway hash used only to equalise login timing on the
	// user-not-found path, so valid emails aren't enumerable by response time.
	dummyHash, err := auth.HashPassword("timing-equalisation-placeholder", argonParams)
	if err != nil {
		logger.Error("failed to generate dummy hash", "error", err)
		os.Exit(1)
	}

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

	// shared configuration struct
	cfg := &apiConfig{
		db:        db,
		platform:  platform,
		secret:    secret,
		params:    argonParams,
		rawDB:     rawDB,
		logger:    logger,
		dummyHash: dummyHash,
	}

	// routes
	mux.Handle("POST /api/login", cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerLogin)))
	mux.Handle("GET /api/me", cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerGetMe)))
	server := &http.Server{
		Addr:    ":1304",
		Handler: mux,
	}
	logger.Error("server failed", "error", server.ListenAndServe())
	os.Exit(1)
}
