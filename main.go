package main

import (
	"database/sql"
	"log/slog"
	"net/http"
	"os"
	"strings"

	"github.com/alexedwards/argon2id"
	"github.com/joho/godotenv"
	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	"github.com/lain-the-coder/ea-qms-backend/internal/logging"
	_ "github.com/lib/pq"
)

type apiConfig struct {
	db             *database.Queries
	platform       string
	secret         string
	params         *argon2id.Params
	rawDB          *sql.DB
	logger         *slog.Logger
	dummyHash      string
	allowedOrigins map[string]struct{}
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
	// CORS — browsers refuse cross-origin requests unless the response says the
	// origin is permitted. Postman and curl are unaffected; they are not browsers.
	allowedOrigins := make(map[string]struct{})
	for _, o := range strings.Split(os.Getenv("ALLOWED_ORIGINS"), ",") {
		o = strings.TrimSpace(o)
		if o != "" {
			allowedOrigins[o] = struct{}{}
		}
	}
	if len(allowedOrigins) == 0 {
		logger.Warn("ALLOWED_ORIGINS is empty — browser clients will be blocked")
	}
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
		db:             db,
		platform:       platform,
		secret:         secret,
		params:         argonParams,
		rawDB:          rawDB,
		logger:         logger,
		dummyHash:      dummyHash,
		allowedOrigins: allowedOrigins,
	}
	// authentication routes
	mux.Handle("POST /api/login", cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerLogin)))
	mux.Handle("POST /api/refresh", cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerRefresh)))
	mux.Handle("POST /api/revoke", cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerRevoke)))
	// user routes
	mux.Handle("GET /api/me", cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerGetMe)))
	mux.Handle("GET /api/approvers", cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerListApprovers)))
	mux.Handle("POST /api/users", cfg.middlewareLogging(cfg.middlewareAuth(cfg.requireRole(roleAdmin, cfg.HandlerCreateUser))))
	mux.Handle("GET /api/users", cfg.middlewareLogging(cfg.middlewareAuth(cfg.requireRole(roleAdmin, cfg.HandlerListUsers))))
	mux.Handle("PUT /api/users/{userID}/active",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.requireRole(roleAdmin, cfg.HandlerUpdateUserStatus))))
	mux.Handle("PUT /api/users/{userID}",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.requireRole(roleAdmin, cfg.HandlerUpdateUserDetails))))
	mux.Handle("GET /api/dashboard",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerDashboard)))
	// change control routes
	mux.Handle("POST /api/changecontrols",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.requireRole(roleCCOwner, cfg.HandlerCreateChangeControl))))
	mux.Handle("GET /api/changecontrols/{ccID}",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerGetChangeControl)))
	mux.Handle("GET /api/changecontrols",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerListChangeControls)))
	mux.Handle("PUT /api/changecontrols/{ccID}",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerSaveDraft)))
	mux.Handle("PUT /api/changecontrols/{ccID}/implementation",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerSaveImplementationDetails)))
	// workflow routes
	mux.Handle("POST /api/changecontrols/{ccID}/submit",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerSubmitForImplApproval)))
	mux.Handle("POST /api/changecontrols/{ccID}/cancel",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerCancelChangeControl)))
	mux.Handle("POST /api/changecontrols/{ccID}/decision",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerImplementationDecision)))
	mux.Handle("POST /api/changecontrols/{ccID}/submit-final",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerSubmitForFinalApproval)))
	mux.Handle("POST /api/changecontrols/{ccID}/final-decision",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerFinalDecision)))
	mux.Handle("GET /api/changecontrols/{ccID}/signatures",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerListSignatures)))
	// file attachment routes
	mux.Handle("POST /api/changecontrols/{ccID}/files/{fieldName}",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerUploadFile)))
	mux.Handle("GET /api/changecontrols/{ccID}/files/{fieldName}",
		cfg.middlewareLogging(cfg.middlewareAuth(cfg.HandlerDownloadFile)))
	// API documentation — public, no auth. Swagger UI cannot send a bearer
	// token to fetch its own spec.
	mux.Handle("GET /docs", cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerDocsPage)))
	mux.Handle("GET /docs/openapi.yaml", cfg.middlewareLogging(http.HandlerFunc(cfg.HandlerOpenAPISpec)))
	server := &http.Server{
		Addr:    ":1304",
		Handler: cfg.middlewareCORS(mux),
	}
	logger.Error("server failed", "error", server.ListenAndServe())
	os.Exit(1)
}
