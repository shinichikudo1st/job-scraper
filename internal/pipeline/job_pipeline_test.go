package pipeline

import (
	"context"
	"testing"
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/models"
	"github.com/shinichikudo1st/job-scraper/internal/scraper"
)

type fakeRepo struct {
	seen  map[string]bool
	marks map[string]string
	jobs  []models.Job
}

func (f *fakeRepo) IsSeen(externalID string) (bool, error) {
	return f.seen[externalID], nil
}

func (f *fakeRepo) MarkSeen(externalID, url, title, status string) error {
	if f.seen == nil {
		f.seen = map[string]bool{}
	}
	if f.marks == nil {
		f.marks = map[string]string{}
	}
	f.seen[externalID] = true
	f.marks[externalID] = status
	return nil
}

func (f *fakeRepo) SaveQueuedJob(job models.Job) error {
	f.jobs = append(f.jobs, job)
	return nil
}

type fakeFetcher struct {
	calls int
}

func (f *fakeFetcher) FetchDetail(ctx context.Context, summary scraper.JobSummary) (scraper.JobDetail, error) {
	f.calls++
	return scraper.JobDetail{
		JobSummary:  summary,
		Description: "Full description for " + summary.Title,
	}, nil
}

func TestPipelineSkipsFilteredJobsWithoutFetchingDetails(t *testing.T) {
	repo := &fakeRepo{seen: map[string]bool{}}
	fetcher := &fakeFetcher{}
	p := &Pipeline{
		Config: scraper.OnlineJobsPHConfig{
			Keywords:         []string{"go"},
			ExcludedKeywords: []string{"caller"},
		},
		Repo: repo,
		Now:  func() time.Time { return time.Date(2026, 5, 23, 1, 2, 3, 0, time.UTC) },
	}

	result, err := p.ProcessSummaries(context.Background(), []scraper.JobSummary{
		{ExternalID: "go-1", Title: "Go Developer", URL: "https://example.com/go-1", Summary: "Build APIs"},
		{ExternalID: "caller-1", Title: "Cold Caller", URL: "https://example.com/caller-1", Summary: "Phone sales"},
	}, fetcher)
	if err != nil {
		t.Fatalf("ProcessSummaries() error = %v", err)
	}
	if result.Queued != 1 || result.FilterSkipped != 1 {
		t.Fatalf("unexpected result: %+v", result)
	}
	if fetcher.calls != 1 {
		t.Fatalf("expected 1 detail fetch, got %d", fetcher.calls)
	}
	if repo.marks["caller-1"] != "skipped" {
		t.Fatalf("filtered job should be compactly marked skipped: %+v", repo.marks)
	}
	if len(repo.jobs) != 1 || repo.jobs[0].Description == nil {
		t.Fatalf("queued job should include full description: %+v", repo.jobs)
	}
}

func TestPipelineSkipsDuplicatesWithoutFetchingDetails(t *testing.T) {
	repo := &fakeRepo{seen: map[string]bool{"go-1": true}}
	fetcher := &fakeFetcher{}
	p := &Pipeline{
		Config: scraper.OnlineJobsPHConfig{Keywords: []string{"go"}},
		Repo:   repo,
	}

	result, err := p.ProcessSummaries(context.Background(), []scraper.JobSummary{
		{ExternalID: "go-1", Title: "Go Developer", URL: "https://example.com/go-1", Summary: "Build APIs"},
	}, fetcher)
	if err != nil {
		t.Fatalf("ProcessSummaries() error = %v", err)
	}
	if result.DuplicateSkipped != 1 {
		t.Fatalf("expected duplicate skip, got %+v", result)
	}
	if fetcher.calls != 0 {
		t.Fatalf("duplicate should not fetch detail, got %d calls", fetcher.calls)
	}
}
