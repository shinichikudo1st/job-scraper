package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/joho/godotenv"
	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/matcher"
	"github.com/shinichikudo1st/job-scraper/internal/server"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatalf("Error loading .env file: %v", err)
	}

	databaseURL := os.Getenv("DATABASE_URL")

	if databaseURL == "" {
		log.Fatal("DATABASE_URL environment variable is required")
	}

	if err := db.RunMigrations(databaseURL); err != nil {
		log.Fatalf("migration error: %v", err)
	}

	dbConn, err := db.ConnectDB()
	if err != nil {
		log.Fatalf("database connection error: %v", err)
	}

	ollamaBaseURL := getenvOrDefault("OLLAMA_BASE_URL", "http://127.0.0.1:11434")
	ollamaModel := getenvOrDefault("OLLAMA_MODEL", "llama3.2:3b")
	matcherWorkers := getenvIntOrDefault("MATCHER_WORKERS", 2)
	matcherBatchSize := getenvIntOrDefault("MATCHER_BATCH_SIZE", 100)
	cvPath := getenvOrDefault("CV_PATH", "cv.text")

	ollamaClient := matcher.NewOllamaClient(ollamaBaseURL, ollamaModel)
	ollamaClient.Think = envBoolTrue(os.Getenv("OLLAMA_THINK"))
	analyzer, err := matcher.NewAnalyzer(ollamaClient, cvPath)
	if err != nil {
		log.Fatalf("analyzer initialization error: %v", err)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	go matcher.RunMatcher(ctx, dbConn, analyzer, matcherWorkers, matcherBatchSize)
	log.Printf("matcher: started (workers=%d, batch_size=%d, model=%s, base_url=%s, ollama_think=%v)", matcherWorkers, matcherBatchSize, ollamaModel, ollamaBaseURL, ollamaClient.Think)

	webRoot := getenvOrDefault("WEB_ROOT", "web")
	router := server.NewRouter(dbConn, webRoot)

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
