package web

import (
	"os"
	"strings"
	"testing"
)

func TestIndexHTMLContainsRequiredHooks(t *testing.T) {
	data, err := os.ReadFile("index.html")
	if err != nil {
		t.Fatalf("read index.html: %v", err)
	}
	s := string(data)

	required := []string{
		"/api/jobs/matched",
		"/api/jobs/matched/export",
		`id="jobs-table"`,
		`id="jobs-tbody"`,
		`id="job-search"`,
		`id="job-sort"`,
		`id="job-tabs"`,
		`id="analysis-line"`,
		`id="btn-scrape"`,
		`id="btn-analyze"`,
		`id="btn-csv"`,
		`id="btn-xlsx"`,
		`id="profile-name"`,
		`id="profile-cv"`,
		`id="profile-upload"`,
		`id="btn-save-profile"`,
		"/api/profile/active",
		`id="ai-provider"`,
		`id="ai-model"`,
		`id="ai-base-url"`,
		`id="ai-api-key"`,
		`id="btn-save-ai"`,
		`id="btn-test-ai"`,
		"/api/ai/settings",
		"/api/ai/test",
		`id="setup-banner"`,
		`id="scraper-search-url"`,
		`id="scraper-keywords"`,
		`id="scraper-excluded"`,
		`id="scraper-preview"`,
		`id="btn-save-scraper"`,
		`id="btn-first-page"`,
		"Pages to scan",
		"Search URL",
		"First page",
		"/api/settings/scraper",
		"/api/scraper/run",
		"/api/jobs/stats",
		"/api/jobs/",
		"/status",
		"/reanalyze",
		"tailwindcss.com",
	}
	for _, sub := range required {
		if !strings.Contains(s, sub) {
			t.Errorf("index.html missing %q", sub)
		}
	}
}
