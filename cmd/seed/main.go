package main

import (
	"context"
	"database/sql"
	"log"
	"os"

	"github.com/alexedwards/argon2id"
	"github.com/joho/godotenv"
	"github.com/lain-the-coder/ea-qms-backend/internal/auth"
	"github.com/lain-the-coder/ea-qms-backend/internal/database"
	_ "github.com/lib/pq"
)

func main() {
	if err := godotenv.Load(); err != nil {
		log.Fatal("error loading .env file")
	}

	if os.Getenv("PLATFORM") != "dev" {
		log.Fatal("seed refused: PLATFORM is not 'dev' — this command never runs in production")
	}
	dbURL := os.Getenv("DB_URL")
	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}
	if err := db.Ping(); err != nil {
		log.Fatalf("database connection failed: %v", err)
	}

	queries := database.New(db)
	ctx := context.Background()
	params := &argon2id.Params{
		Memory:      64 * 1024,
		Iterations:  3,
		Parallelism: 2,
		SaltLength:  16,
		KeyLength:   32,
	}

	hash, err := auth.HashPassword("DevPassw0rd!", params)
	if err != nil {
		log.Fatalf("error hashing seed password: %v", err)
	}
	seedUsers := []struct {
		name, email, role string
	}{
		{"System Administrator", "admin@eaqms.local", "Admin"},
		{"Default CC Owner", "owner@eaqms.local", "CC Owner"},
		{"Default Approver", "approver@eaqms.local", "Approver"},
		{"Default Viewer", "viewer@eaqms.local", "Viewer"},
	}
	for _, u := range seedUsers {
		user, err := queries.CreateUser(ctx, database.CreateUserParams{
			FullName:       u.name,
			Email:          u.email,
			HashedPassword: hash,
			Role:           u.role,
		})
		if err != nil {
			log.Fatalf("failed to seed %s (%s): %v", u.name, u.email, err)
		}
		log.Printf("seeded %s as %s (id %s)", user.Email, user.Role, user.ID)
	}

	log.Println("seed complete: 4 users")
}
