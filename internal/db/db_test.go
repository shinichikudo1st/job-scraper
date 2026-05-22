package db_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/shinichikudo1st/job-scraper/internal/db"
)

func TestLoadDBConfigDefaultsToSQLite(t *testing.T) {
	t.Setenv("DB_DRIVER", "")
	t.Setenv("DB_PATH", "")

	config, err := db.LoadDBConfig()
	if err != nil {
		t.Fatalf("LoadDBConfig() error = %v", err)
	}
	if config.Driver != db.DriverSQLite {
		t.Fatalf("driver = %q, want %q", config.Driver, db.DriverSQLite)
	}
	if config.SQLitePath == "" {
		t.Fatalf("SQLitePath should be populated")
	}
}

func TestConnectDBCreatesSQLiteFile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "nested", "smarter-olj.db")
	t.Setenv("DB_DRIVER", db.DriverSQLite)
	t.Setenv("DB_PATH", dbPath)

	conn, err := db.ConnectDB()
	if err != nil {
		t.Fatalf("ConnectDB() error = %v", err)
	}

	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	defer sqlDB.Close()

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("Ping() error = %v", err)
	}
	if _, err := os.Stat(dbPath); err != nil {
		t.Fatalf("sqlite database file was not created: %v", err)
	}
}

func TestRunSQLiteMigrationsCreatesJobsTable(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "smarter-olj.db")
	config := &db.DBConfig{
		Driver:     db.DriverSQLite,
		SQLitePath: dbPath,
	}

	conn, err := db.ConnectConfiguredDB(config)
	if err != nil {
		t.Fatalf("ConnectConfiguredDB() error = %v", err)
	}
	sqlDB, err := conn.DB()
	if err != nil {
		t.Fatalf("DB() error = %v", err)
	}
	defer sqlDB.Close()

	if err := db.RunSQLiteMigrations(conn); err != nil {
		t.Fatalf("RunSQLiteMigrations() error = %v", err)
	}

	var tableName string
	err = conn.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", "jobs").Scan(&tableName).Error
	if err != nil {
		t.Fatalf("query sqlite schema: %v", err)
	}
	if tableName != "jobs" {
		t.Fatalf("jobs table missing, got %q", tableName)
	}
}
