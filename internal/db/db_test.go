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

	tableName = ""
	err = conn.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", "profiles").Scan(&tableName).Error
	if err != nil {
		t.Fatalf("query sqlite profile schema: %v", err)
	}
	if tableName != "profiles" {
		t.Fatalf("profiles table missing, got %q", tableName)
	}

	tableName = ""
	err = conn.Raw("SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?", "seen_jobs").Scan(&tableName).Error
	if err != nil {
		t.Fatalf("query sqlite seen jobs schema: %v", err)
	}
	if tableName != "seen_jobs" {
		t.Fatalf("seen_jobs table missing, got %q", tableName)
	}
}

func TestUpsertAndGetActiveProfile(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "smarter-olj.db")
	conn, err := db.ConnectConfiguredDB(&db.DBConfig{
		Driver:     db.DriverSQLite,
		SQLitePath: dbPath,
	})
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

	profile, err := db.UpsertActiveProfile(conn, "Main", "Go developer")
	if err != nil {
		t.Fatalf("UpsertActiveProfile() error = %v", err)
	}
	if profile.ID == 0 || !profile.IsActive {
		t.Fatalf("unexpected saved profile: %+v", profile)
	}

	got, err := db.GetActiveProfile(conn)
	if err != nil {
		t.Fatalf("GetActiveProfile() error = %v", err)
	}
	if got.Name != "Main" || got.CVText != "Go developer" {
		t.Fatalf("unexpected active profile: %+v", got)
	}
}

func TestMarkSeenJobSkipsDuplicateWork(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "smarter-olj.db")
	conn, err := db.ConnectConfiguredDB(&db.DBConfig{
		Driver:     db.DriverSQLite,
		SQLitePath: dbPath,
	})
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

	if err := db.MarkSeenJob(conn, "job-1", "https://example.com/job-1", "Go Developer", "seen"); err != nil {
		t.Fatalf("MarkSeenJob() error = %v", err)
	}
	if err := db.MarkSeenJob(conn, "job-1", "https://example.com/job-1", "Go Developer Updated", "seen"); err != nil {
		t.Fatalf("MarkSeenJob() duplicate error = %v", err)
	}
	seen, err := db.HasSeenJob(conn, "job-1")
	if err != nil {
		t.Fatalf("HasSeenJob() error = %v", err)
	}
	if !seen {
		t.Fatalf("expected job-1 to be seen")
	}
}
