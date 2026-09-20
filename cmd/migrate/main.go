package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
)

func main() {
	direction := flag.String("direction", "up", "migration direction: up or down")
	dbURL := flag.String("db-url", os.Getenv("DB_URL"), "PostgreSQL database connection URL")
	migrationsPath := flag.String("path", "file://migrations", "Path to migrations directory")
	flag.Parse()

	if *dbURL == "" {
		*dbURL = "postgres://postgres:password@localhost:5433/golearning?sslmode=disable"
	}

	// golang-migrate requires pgx5:// driver scheme for pgx/v5 driver
	migrateURL := *dbURL
	if len(migrateURL) >= 11 && migrateURL[:11] == "postgres://" {
		migrateURL = "pgx5://" + migrateURL[11:]
	} else if len(migrateURL) >= 13 && migrateURL[:13] == "postgresql://" {
		migrateURL = "pgx5://" + migrateURL[13:]
	}

	m, err := migrate.New(*migrationsPath, migrateURL)
	if err != nil {
		log.Fatalf("failed to initialize migrate: %v", err)
	}
	defer m.Close()

	switch *direction {
	case "up":
		if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate up failed: %v", err)
		}
		fmt.Println("migration up completed successfully")
	case "down":
		if err := m.Down(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
			log.Fatalf("migrate down failed: %v", err)
		}
		fmt.Println("migration down completed successfully")
	default:
		log.Fatalf("unsupported migration direction: %s. Use 'up' or 'down'", *direction)
	}
}
