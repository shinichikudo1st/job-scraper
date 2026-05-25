package scraper

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

const listingFixture = `
<!doctype html>
<html>
  <body>
    <article class="job">
      <a href="/jobseekers/job/Go-Backend-Developer-12345">Go Backend Developer</a>
      <span>$1200/month</span>
      <p>Build APIs with PostgreSQL.</p>
    </article>
    <article class="job">
      <a href="https://www.onlinejobs.ph/jobseekers/job/Cold-Caller-98765">Cold Caller</a>
      <span>$500/month</span>
      <p>Phone sales.</p>
    </article>
  </body>
</html>`

func TestParseListingHTMLExtractsSummaries(t *testing.T) {
	jobs, err := ParseListingHTML(listingFixture, DefaultOnlineJobsPHBaseURL)
	if err != nil {
		t.Fatalf("ParseListingHTML() error = %v", err)
	}
	if len(jobs) != 2 {
		t.Fatalf("expected 2 jobs, got %d: %+v", len(jobs), jobs)
	}
	if jobs[0].ExternalID != "go-backend-developer-12345" {
		t.Fatalf("unexpected external id: %q", jobs[0].ExternalID)
	}
	if jobs[0].URL != "https://www.onlinejobs.ph/jobseekers/job/Go-Backend-Developer-12345" {
		t.Fatalf("unexpected URL: %q", jobs[0].URL)
	}
	if jobs[0].Title != "Go Backend Developer" {
		t.Fatalf("unexpected title: %q", jobs[0].Title)
	}
}

func TestFilterSummaries(t *testing.T) {
	jobs, err := ParseListingHTML(listingFixture, DefaultOnlineJobsPHBaseURL)
	if err != nil {
		t.Fatalf("ParseListingHTML() error = %v", err)
	}

	filtered := FilterSummaries(jobs, OnlineJobsPHConfig{
		Keywords:         []string{"backend"},
		ExcludedKeywords: []string{"caller"},
		MinSalary:        1000,
	})
	if len(filtered) != 1 {
		t.Fatalf("expected 1 filtered job, got %d: %+v", len(filtered), filtered)
	}
	if !strings.Contains(strings.ToLower(filtered[0].Title), "backend") {
		t.Fatalf("unexpected filtered job: %+v", filtered[0])
	}
}

func TestParseDetailHTML(t *testing.T) {
	detail, err := ParseDetailHTML(`
		<html>
			<body>
				<h1>Senior Go Engineer</h1>
				<div class="job-description">Design APIs and data pipelines.</div>
			</body>
		</html>`,
		JobSummary{Title: "Fallback", URL: "https://example.com/job", ExternalID: "job-1"},
	)
	if err != nil {
		t.Fatalf("ParseDetailHTML() error = %v", err)
	}
	if detail.Title != "Senior Go Engineer" {
		t.Fatalf("unexpected detail title: %q", detail.Title)
	}
	if !strings.Contains(detail.Description, "Design APIs") {
		t.Fatalf("unexpected description: %q", detail.Description)
	}
}

func TestOnlineJobsPHPageURLUsesOffsetPagination(t *testing.T) {
	s := NewOnlineJobsPHScraper(OnlineJobsPHConfig{
		SearchURL: "https://www.onlinejobs.ph/jobseekers/jobsearch/120?jobkeyword=developer",
	})

	tests := map[int]string{
		1: "https://www.onlinejobs.ph/jobseekers/jobsearch/120?jobkeyword=developer",
		2: "https://www.onlinejobs.ph/jobseekers/jobsearch/150?jobkeyword=developer",
		3: "https://www.onlinejobs.ph/jobseekers/jobsearch/180?jobkeyword=developer",
	}

	for page, want := range tests {
		got, err := s.pageURL(page)
		if err != nil {
			t.Fatalf("pageURL(%d) error = %v", page, err)
		}
		if got != want {
			t.Fatalf("pageURL(%d) = %q, want %q", page, got, want)
		}
	}
}

func TestOnlineJobsPHPageURLStartsAtFirstPage(t *testing.T) {
	s := NewOnlineJobsPHScraper(OnlineJobsPHConfig{
		SearchURL: "https://www.onlinejobs.ph/jobseekers/jobsearch",
	})

	got, err := s.pageURL(2)
	if err != nil {
		t.Fatalf("pageURL(2) error = %v", err)
	}
	if got != "https://www.onlinejobs.ph/jobseekers/jobsearch/30" {
		t.Fatalf("pageURL(2) = %q", got)
	}
}

type fakeSeenStore struct {
	seen map[string]bool
}

func (f *fakeSeenStore) IsSeen(externalID string) (bool, error) {
	return f.seen[externalID], nil
}

func (f *fakeSeenStore) MarkSeen(externalID, url, title string) error {
	f.seen[externalID] = true
	return nil
}

func TestScrapeNewDetailsFetchesOnlyNewCandidateDetails(t *testing.T) {
	var detailRequests int
	var serverURL string
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/search":
			fmt.Fprintf(w, `
				<article class="job">
					<a href="%s/jobseekers/job/Go-Backend-Developer-12345">Go Backend Developer</a>
					<span>$1200/month</span>
					<p>Build APIs.</p>
				</article>
				<article class="job">
					<a href="%s/jobseekers/job/Cold-Caller-98765">Cold Caller</a>
					<span>$500/month</span>
					<p>Phone sales.</p>
				</article>`, serverURL, serverURL)
		case "/jobseekers/job/Go-Backend-Developer-12345":
			detailRequests++
			fmt.Fprint(w, `<h1>Go Backend Developer</h1><main>Build APIs with Go.</main>`)
		case "/jobseekers/job/Cold-Caller-98765":
			detailRequests++
			fmt.Fprint(w, `<h1>Cold Caller</h1><main>Phone sales.</main>`)
		default:
			http.NotFound(w, r)
		}
	}))
	serverURL = ts.URL
	defer ts.Close()

	s := NewOnlineJobsPHScraper(OnlineJobsPHConfig{
		BaseURL:          ts.URL,
		SearchURL:        ts.URL + "/search",
		Keywords:         []string{"go"},
		ExcludedKeywords: []string{"caller"},
		MaxPages:         1,
	})
	store := &fakeSeenStore{seen: map[string]bool{}}

	details, err := s.ScrapeNewDetails(context.Background(), store)
	if err != nil {
		t.Fatalf("ScrapeNewDetails() error = %v", err)
	}
	if len(details) != 1 {
		t.Fatalf("expected 1 detail, got %d: %+v", len(details), details)
	}
	if detailRequests != 1 {
		t.Fatalf("expected only 1 detail request, got %d", detailRequests)
	}

	details, err = s.ScrapeNewDetails(context.Background(), store)
	if err != nil {
		t.Fatalf("second ScrapeNewDetails() error = %v", err)
	}
	if len(details) != 0 {
		t.Fatalf("expected duplicate to be skipped, got %+v", details)
	}
	if detailRequests != 1 {
		t.Fatalf("expected no duplicate detail request, got %d", detailRequests)
	}
}
