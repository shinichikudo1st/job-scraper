package matcher

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/db"
	"github.com/shinichikudo1st/job-scraper/internal/models"
	"gorm.io/gorm"
)

const (
	defaultWorkerCount  = 2
	defaultBatchSize    = 100
	defaultPollInterval = 5 * time.Minute
)

type JobRepository interface {
	FetchPendingJobs(limit int, postedAfter time.Time) ([]models.Job, error)
	UpdateJobMatch(id int, isMatch bool, score int, reason string) error
}

type gormJobRepository struct {
	conn *gorm.DB
}

func NewGormJobRepository(conn *gorm.DB) JobRepository {
	return &gormJobRepository{conn: conn}
}

func (r *gormJobRepository) FetchPendingJobs(limit int, postedAfter time.Time) ([]models.Job, error) {
	return db.FetchPendingJobs(r.conn, limit, postedAfter)
}

func (r *gormJobRepository) UpdateJobMatch(id int, isMatch bool, score int, reason string) error {
	return db.UpdateJobMatch(r.conn, id, isMatch, score, reason)
}

// RunMatcher continuously polls pending jobs, analyzes them with a worker pool,
// and updates match results. Each poll drains every job posted today that is still
// pending (match_score IS NULL), fetching up to batchSize rows per DB round-trip.
// batchSize <= 0 uses defaultBatchSize. It stops gracefully when ctx is canceled.
func RunMatcher(ctx context.Context, conn *gorm.DB, analyzer *Analyzer, workers, batchSize int) {
	repo := NewGormJobRepository(conn)
	runMatcherLoop(ctx, repo, analyzer, workers, batchSize, defaultPollInterval)
}

func runMatcherLoop(
	ctx context.Context,
	repo JobRepository,
	analyzer JobAnalyzer,
	workers int,
	batchSize int,
	pollInterval time.Duration,
) {
	if workers <= 0 {
		workers = defaultWorkerCount
	}
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}
	if pollInterval <= 0 {
		pollInterval = defaultPollInterval
	}

	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			log.Println("matcher: shutdown requested")
			return
		default:
		}

		runMatcherBatch(ctx, repo, analyzer, workers, batchSize)

		select {
		case <-ctx.Done():
			log.Println("matcher: shutdown requested")
			return
		case <-ticker.C:
		}
	}
}

func runMatcherBatch(
	ctx context.Context,
	repo JobRepository,
	analyzer JobAnalyzer,
	workers int,
	batchSize int,
) {
	if workers <= 0 {
		workers = defaultWorkerCount
	}
	if batchSize <= 0 {
		batchSize = defaultBatchSize
	}

	postedAfter := startOfToday(time.Now())
	sawAny := false
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		jobs, err := repo.FetchPendingJobs(batchSize, postedAfter)
		if err != nil {
			log.Printf("matcher: fetch pending jobs failed: %v", err)
			return
		}
		if len(jobs) == 0 {
			if !sawAny {
				log.Println("matcher: no pending jobs")
			}
			return
		}
		sawAny = true

		nOK := processMatcherJobBatch(ctx, repo, analyzer, workers, jobs)
		if nOK == 0 {
			log.Printf("matcher: no successful updates in batch (%d job(s)); stopping drain until next poll", len(jobs))
			return
		}
		if len(jobs) < batchSize {
			return
		}
	}
}

// processMatcherJobBatch runs the worker pool over jobs and returns how many rows were updated successfully.
func processMatcherJobBatch(
	ctx context.Context,
	repo JobRepository,
	analyzer JobAnalyzer,
	workers int,
	jobs []models.Job,
) int {
	var successCount int
	var mu sync.Mutex

	jobCh := make(chan models.Job)
	var wg sync.WaitGroup

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			for job := range jobCh {
				select {
				case <-ctx.Done():
					return
				default:
				}

				result, err := analyzer.AnalyzeJob(&job, "")
				if err != nil {
					log.Printf("matcher: worker=%d analyze failed for job id=%d: %v", workerID, job.ID, err)
					continue
				}

				if err := repo.UpdateJobMatch(job.ID, result.Fit, result.Score, result.Reason); err != nil {
					log.Printf("matcher: worker=%d update failed for job id=%d: %v", workerID, job.ID, err)
					continue
				}

				mu.Lock()
				successCount++
				mu.Unlock()

				out, err := json.Marshal(map[string]any{
					"fit":    result.Fit,
					"score":  result.Score,
					"reason": result.Reason,
				})
				if err != nil {
					log.Printf("matcher: worker=%d job id=%d saved (fit=%v score=%d) json marshal: %v", workerID, job.ID, result.Fit, result.Score, err)
					continue
				}
				log.Printf("matcher: worker=%d analyzed job id=%d title=%q saved result=%s", workerID, job.ID, truncateRunes(job.Title, 72), string(out))
			}
		}(i + 1)
	}

	for _, job := range jobs {
		select {
		case <-ctx.Done():
			close(jobCh)
			wg.Wait()
			return successCount
		case jobCh <- job:
		}
	}

	close(jobCh)
	wg.Wait()
	return successCount
}

func startOfToday(now time.Time) time.Time {
	y, m, d := now.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, now.Location())
}

func truncateRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
