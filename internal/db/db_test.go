package db_test

import (
	"testing"

	"log"
	"os"

	"github.com/joho/godotenv"
	"github.com/shinichikudo1st/job-scraper/internal/db"
)

func TestMain(m *testing.M) {
	err := godotenv.Load("../../.env")
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	os.Exit(m.Run())
}

func TestConnectDB(t *testing.T) {
	dbConn, err := db.ConnectDB()
	if err != nil {
		t.Fatalf("failed to connect to database: %v", err)
	}

	sqlDB, err := dbConn.DB()
	if err != nil {
		t.Fatalf("failed to get sql database: %v", err)
	}

	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("failed to ping database: %v", err)
	}

	t.Log("connected to database")
}
