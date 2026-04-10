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
		`id="filter-notified"`,
		`id="date-from"`,
		`id="date-to"`,
		`id="btn-csv"`,
		`id="btn-xlsx"`,
		"tailwindcss.com",
	}
	for _, sub := range required {
		if !strings.Contains(s, sub) {
			t.Errorf("index.html missing %q", sub)
		}
	}
}
