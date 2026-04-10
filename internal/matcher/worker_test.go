package matcher

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/models"
)

type fakeRepo struct {
	pending []models.Job

	mu      sync.Mutex
	updated map[int]MatchResult
}

func (f *fakeRepo) FetchPendingJobs(limit int, postedAfter time.Time) ([]models.Job, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updated == nil {
		f.updated = make(map[int]MatchResult)
	}
	var out []models.Job
	for _, j := range f.pending {
		if len(out) >= limit {
			break
		}
		if _, done := f.updated[j.ID]; done {
			continue
		}
		out = append(out, j)
	}
	return out, nil
}

func (f *fakeRepo) UpdateJobMatch(id int, isMatch bool, score int, reason string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.updated == nil {
		f.updated = make(map[int]MatchResult)
	}
	f.updated[id] = MatchResult{
		Fit:    isMatch,
		Score:  score,
		Reason: reason,
	}
	return nil
}

type fakeJobAnalyzer struct {
	mu      sync.Mutex
	results map[int]MatchResult
	failIDs map[int]bool
	calls   int
}

func (f *fakeJobAnalyzer) AnalyzeJob(job *models.Job, cv string) (*MatchResult, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls++
	if f.failIDs[job.ID] {
		return nil, errors.New("analysis failed")
	}
	r := f.results[job.ID]
	return &MatchResult{
		Fit:    r.Fit,
		Score:  r.Score,
		Reason: r.Reason,
	}, nil
}

func TestRunMatcherBatchProcessesJobs(t *testing.T) {
	repo := &fakeRepo{
		pending: []models.Job{
			{ID: 1, Title: "Go Dev", URL: "https://example.com/1"},
			{ID: 2, Title: "Backend Dev", URL: "https://example.com/2"},
			{ID: 3, Title: "Fullstack Dev", URL: "https://example.com/3"},
		},
	}
	analyzer := &fakeJobAnalyzer{
		results: map[int]MatchResult{
			1: {Fit: true, Score: 90, Reason: "Strong fit"},
			2: {Fit: false, Score: 30, Reason: "Weak fit"},
			3: {Fit: true, Score: 80, Reason: "Good fit"},
		},
		failIDs: map[int]bool{},
	}

	runMatcherBatch(context.Background(), repo, analyzer, 2, 10)

	if len(repo.updated) != 3 {
		t.Fatalf("expected 3 updated jobs, got %d", len(repo.updated))
	}
	if repo.updated[1].Score != 90 || !repo.updated[1].Fit {
		t.Fatalf("unexpected update for job 1: %+v", repo.updated[1])
	}
}

func TestRunMatcherBatchContinuesOnAnalyzeError(t *testing.T) {
	repo := &fakeRepo{
		pending: []models.Job{
			{ID: 10, Title: "Go Dev", URL: "https://example.com/10"},
			{ID: 11, Title: "Backend Dev", URL: "https://example.com/11"},
		},
	}
	analyzer := &fakeJobAnalyzer{
		results: map[int]MatchResult{
			11: {Fit: true, Score: 85, Reason: "Relevant experience"},
		},
		failIDs: map[int]bool{
			10: true,
		},
	}

	runMatcherBatch(context.Background(), repo, analyzer, 2, 10)

	if len(repo.updated) != 1 {
		t.Fatalf("expected 1 updated job after one analyze error, got %d", len(repo.updated))
	}
	if _, exists := repo.updated[10]; exists {
		t.Fatalf("job 10 should not be updated when analyze fails")
	}
	if _, exists := repo.updated[11]; !exists {
		t.Fatalf("job 11 should still be updated")
	}
}

func TestRunMatcherBatchDrainsMultiplePages(t *testing.T) {
	var pending []models.Job
	for i := 1; i <= 25; i++ {
		pending = append(pending, models.Job{
			ID:    i,
			Title: "Role",
			URL:   "https://example.com/job",
		})
	}
	repo := &fakeRepo{pending: pending}
	analyzer := &fakeJobAnalyzer{
		results: make(map[int]MatchResult),
		failIDs: map[int]bool{},
	}
	for i := 1; i <= 25; i++ {
		analyzer.results[i] = MatchResult{Fit: true, Score: 80, Reason: "ok"}
	}

	runMatcherBatch(context.Background(), repo, analyzer, 2, 10)

	if len(repo.updated) != 25 {
		t.Fatalf("expected 25 updated jobs, got %d", len(repo.updated))
	}
}
