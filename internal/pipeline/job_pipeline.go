package pipeline

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/shinichikudo1st/job-scraper/internal/models"
	"github.com/shinichikudo1st/job-scraper/internal/scraper"
)

type DetailFetcher interface {
	FetchDetail(ctx context.Context, summary scraper.JobSummary) (scraper.JobDetail, error)
}

type Repository interface {
	IsSeen(externalID string) (bool, error)
	MarkSeen(externalID, url, title, status string) error
	SaveQueuedJob(job models.Job) error
}

type Pipeline struct {
	Config scraper.OnlineJobsPHConfig
	Repo   Repository
	Now    func() time.Time
}

type Result struct {
	SeenSkipped      int
	FilterSkipped    int
	Queued           int
	DetailsFetched   int
	DuplicateSkipped int
}

func (p *Pipeline) ProcessSummaries(ctx context.Context, summaries []scraper.JobSummary, fetcher DetailFetcher) (Result, error) {
	if p == nil {
		return Result{}, errors.New("pipeline is nil")
	}
	if p.Repo == nil {
		return Result{}, errors.New("pipeline repository is required")
	}
	if fetcher == nil {
		return Result{}, errors.New("detail fetcher is required")
	}

	now := time.Now
	if p.Now != nil {
		now = p.Now
	}

	var result Result
	for _, summary := range summaries {
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		default:
		}

		if strings.TrimSpace(summary.ExternalID) == "" {
			continue
		}
		seen, err := p.Repo.IsSeen(summary.ExternalID)
		if err != nil {
			return result, err
		}
		if seen {
			result.DuplicateSkipped++
			continue
		}

		if !matchesLocalFilters(summary, p.Config) {
			if err := p.Repo.MarkSeen(summary.ExternalID, summary.URL, summary.Title, "skipped"); err != nil {
				return result, err
			}
			result.FilterSkipped++
			continue
		}

		detail, err := fetcher.FetchDetail(ctx, summary)
		if err != nil {
			return result, err
		}
		result.DetailsFetched++

		job := jobFromDetail(detail, now())
		if err := p.Repo.SaveQueuedJob(job); err != nil {
			return result, err
		}
		if err := p.Repo.MarkSeen(summary.ExternalID, summary.URL, summary.Title, "queued"); err != nil {
			return result, err
		}
		result.Queued++
	}

	return result, nil
}

func matchesLocalFilters(summary scraper.JobSummary, config scraper.OnlineJobsPHConfig) bool {
	return len(scraper.FilterSummaries([]scraper.JobSummary{summary}, config)) == 1
}

func jobFromDetail(detail scraper.JobDetail, scrapedAt time.Time) models.Job {
	description := strings.TrimSpace(detail.Description)
	salary := strings.TrimSpace(detail.Salary)
	job := models.Job{
		ExternalID:  detail.ExternalID,
		Title:       strings.TrimSpace(detail.Title),
		URL:         strings.TrimSpace(detail.URL),
		ScrapedAt:   scrapedAt.UTC(),
		Status:      "queued",
		RetryCount:  0,
		Description: nil,
		Salary:      nil,
	}
	if description != "" {
		job.Description = &description
	}
	if salary != "" {
		job.Salary = &salary
	}
	return job
}
