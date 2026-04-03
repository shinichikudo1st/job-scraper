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
