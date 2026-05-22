package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/shinichikudo1st/job-scraper/internal/ai"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/matcher"
	"github.com/shinichikudo1st/job-scraper/internal/server"
	"gorm.io/gorm"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Printf("config: .env not loaded, using environment/defaults: %v", err)
	}

	dbConfig, err := db.LoadDBConfig()
	if err != nil {
		log.Fatalf("database configuration error: %v", err)
	}

	var dbConn *gorm.DB
	switch dbConfig.Driver {
	case db.DriverPostgres:
		if err := db.RunMigrations(dbConfig.DatabaseURL); err != nil {
			log.Fatalf("migration error: %v", err)
		}
		dbConn, err = db.ConnectConfiguredDB(dbConfig)
	case db.DriverSQLite:
		dbConn, err = db.ConnectConfiguredDB(dbConfig)
		if err == nil {
			err = db.RunSQLiteMigrations(dbConn)
		}
	default:
		err = fmt.Errorf("unsupported DB_DRIVER %q", dbConfig.Driver)
	}
	if err != nil {
		log.Fatalf("database initialization error: %v", err)
	}
	log.Printf("database: connected using %s", dbConfig.Driver)

	aiStore, err := ai.NewConfigStore(ai.LoadConfigFromEnv())
	if err != nil {
		log.Fatalf("ai configuration error: %v", err)
	}
	aiConfig := aiStore.Get()
	matcherWorkers := getenvIntOrDefault("MATCHER_WORKERS", 2)
	matcherBatchSize := getenvIntOrDefault("MATCHER_BATCH_SIZE", 100)
	cvPath := getenvOrDefault("CV_PATH", "cv.text")

	aiClient := &ai.DynamicClient{Store: aiStore}
	cvProvider := &db.GormProfileCVProvider{DB: dbConn}
	analyzer, err := matcher.NewAnalyzerWithCVProvider(aiClient, cvPath, cvProvider)
	if err != nil {
		log.Fatalf("analyzer initialization error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go matcher.RunMatcher(ctx, dbConn, analyzer, matcherWorkers, matcherBatchSize)
	log.Printf("matcher: started (workers=%d, batch_size=%d, ai_provider=%s, model=%s, base_url=%s)", matcherWorkers, matcherBatchSize, aiConfig.Provider, aiConfig.Model, aiConfig.BaseURL)

	webRoot := os.Getenv("WEB_ROOT")
	router := server.NewRouter(dbConn, webRoot, aiStore)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer shutdownCancel()

		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("server shutdown error: %v", err)
		}
	}()

	log.Printf("http server: listening on :%s", port)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

func getenvOrDefault(key, fallback string) string {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}
	return value
}

func envBoolTrue(s string) bool {
	s = strings.TrimSpace(strings.ToLower(s))
	return s == "1" || s == "true" || s == "yes"
}

func getenvIntOrDefault(key string, fallback int) int {
	value := os.Getenv(key)
	if value == "" {
		return fallback
	}

	n, err := strconv.Atoi(value)
	if err != nil || n <= 0 {
		return fallback
	}
	return n
}
