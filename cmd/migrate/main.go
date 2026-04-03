package main

import (
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"

	"github.com/AlexSamarskii/URL-shortener/internal/pkg/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if cfg.Storage.Type != "postgres" {
		log.Println("Skipping migrations because non-postgres storage selected.")
		return
	}

	migrationsPath := "file:///migrations"
	if envPath := os.Getenv("MIGRATIONS_PATH"); envPath != "" {
		migrationsPath = "file://" + envPath
	}
	log.Printf("Using migrations from: %s", migrationsPath)

	m, err := migrate.New(migrationsPath, cfg.Postgres.DSN)
	if err != nil {
		log.Fatalf("Failed to create migrator: %v", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		log.Fatalf("Migration failed: %v", err)
	}

	if err == migrate.ErrNoChange {
		log.Println("No new migrations to apply")
	} else {
		log.Println("Migrations applied successfully")
	}
}
