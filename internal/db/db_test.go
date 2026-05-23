package db_test

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/models"
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

func TestQueuedJobLifecycleAndPruneSeenJobs(t *testing.T) {
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

	desc := "Build backend APIs."
	if err := db.UpsertQueuedJob(conn, models.Job{
		ExternalID:  "job-queued-1",
		Title:       "Go Developer",
		URL:         "https://example.com/job-queued-1",
		Description: &desc,
	}); err != nil {
		t.Fatalf("UpsertQueuedJob() error = %v", err)
	}

	pending, err := db.FetchPendingJobs(conn, 10, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("FetchPendingJobs() error = %v", err)
	}
	if len(pending) != 1 || pending[0].Status != "queued" {
		t.Fatalf("unexpected pending jobs: %+v", pending)
	}

	if err := db.MarkJobAnalysisFailed(conn, pending[0].ID, "model unavailable"); err != nil {
		t.Fatalf("MarkJobAnalysisFailed() error = %v", err)
	}
	pending, err = db.FetchPendingJobs(conn, 10, time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("FetchPendingJobs() after failure error = %v", err)
	}
	if len(pending) != 0 {
		t.Fatalf("failed job should not be pending again: %+v", pending)
	}

	if err := db.MarkSeenJob(conn, "old-skipped", "https://example.com/old", "Old", "skipped"); err != nil {
		t.Fatalf("MarkSeenJob() error = %v", err)
	}
	oldTime := time.Now().Add(-48 * time.Hour)
	if err := conn.Model(&models.SeenJob{}).Where("external_id = ?", "old-skipped").Update("last_seen_at", oldTime).Error; err != nil {
		t.Fatalf("age seen job: %v", err)
	}
	pruned, err := db.PruneSeenJobs(conn, "skipped", time.Now().Add(-24*time.Hour))
	if err != nil {
		t.Fatalf("PruneSeenJobs() error = %v", err)
	}
	if pruned != 1 {
		t.Fatalf("expected 1 pruned row, got %d", pruned)
	}
}
