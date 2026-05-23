package db

import (
	"errors"
	"fmt"
	"log"
	"path/filepath"
	"runtime"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"gorm.io/gorm"
)

func migrationSourceURL() (string, error) {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		return "", errors.New("migrationSourceURL: runtime.Caller failed")
	}
	dir := filepath.Dir(file)
	abs, err := filepath.Abs(dir)
	if err != nil {
		return "", err
	}
	p := filepath.ToSlash(abs)
	// migrate's file driver does u.Host+u.Path then os.DirFS(p). On Windows,
	// file:///C:/path yields Path "/C:/..." which DirFS cannot open; use
	// file://C:/rest so Host is "C:" and filepath.Abs fixes the path.
	return "file://" + p, nil
}

func RunMigrations(databaseURL string) error {
	src, err := migrationSourceURL()
	if err != nil {
		return fmt.Errorf("migrations path: %w", err)
	}
	m, err := migrate.New(
		src,
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil {
		if errors.Is(err, migrate.ErrNoChange) {
			log.Println("migrations: no new migrations to apply")
			return nil
		}
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	log.Println("migrations: all migrations applied successfully")
	return nil
}

func RunSQLiteMigrations(conn *gorm.DB) error {
	if conn == nil {
		return errors.New("sqlite migrations require a database connection")
	}

	statements := []string{
		`CREATE TABLE IF NOT EXISTS jobs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			external_id TEXT UNIQUE NOT NULL,
			title TEXT NOT NULL,
			company TEXT,
			location TEXT,
			salary TEXT,
			description TEXT,
			url TEXT NOT NULL,
			posted_at DATETIME,
			scraped_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			is_match BOOLEAN DEFAULT FALSE,
			match_score INTEGER,
			match_reason TEXT,
			notified BOOLEAN DEFAULT FALSE,
			analyzed_at DATETIME
		)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_is_match ON jobs(is_match)`,
		`CREATE INDEX IF NOT EXISTS idx_jobs_notified ON jobs(notified)`,
		`CREATE TABLE IF NOT EXISTS profiles (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			cv_text TEXT NOT NULL,
			is_active BOOLEAN NOT NULL DEFAULT FALSE,
			created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_profiles_is_active ON profiles(is_active)`,
		`CREATE TABLE IF NOT EXISTS seen_jobs (
			external_id TEXT PRIMARY KEY,
			url TEXT NOT NULL,
			title TEXT NOT NULL,
			status TEXT NOT NULL DEFAULT 'seen',
			first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
			last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
		)`,
		`CREATE INDEX IF NOT EXISTS idx_seen_jobs_status ON seen_jobs(status)`,
		`CREATE INDEX IF NOT EXISTS idx_seen_jobs_last_seen_at ON seen_jobs(last_seen_at)`,
	}

	for _, stmt := range statements {
		if err := conn.Exec(stmt).Error; err != nil {
			return fmt.Errorf("run sqlite migration statement: %w", err)
		}
	}

	log.Println("sqlite migrations: schema is ready")
	return nil
}

// RollbackOne steps down exactly one migration.
// Useful during development when you want to undo the last migration.
func RollbackOne(databaseURL string) error {
	src, err := migrationSourceURL()
	if err != nil {
		return fmt.Errorf("migrations path: %w", err)
	}
	m, err := migrate.New(
		src,
		databaseURL,
	)
	if err != nil {
		return fmt.Errorf("failed to create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Steps(-1); err != nil {
		return fmt.Errorf("failed to rollback: %w", err)
	}

	log.Println("migrations: rolled back one step")
	return nil
}
