package api

import (
	"testing"
	"time"
)

func TestScrapeTimeoutScalesWithPages(t *testing.T) {
	tests := []struct {
		pages int
		want  time.Duration
	}{
		{pages: 0, want: 5 * time.Minute},
		{pages: 1, want: 5 * time.Minute},
		{pages: 10, want: 10 * time.Minute},
		{pages: 25, want: 20 * time.Minute},
	}

	for _, tt := range tests {
		if got := scrapeTimeout(tt.pages); got != tt.want {
			t.Fatalf("scrapeTimeout(%d) = %s, want %s", tt.pages, got, tt.want)
		}
	}
}
