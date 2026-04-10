package matcher

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/shinichikudo1st/job-scraper/internal/models"
)

type fakeGenerator struct {
	response   string
	err        error
	lastPrompt string
}

func (f *fakeGenerator) Generate(prompt string) (string, error) {
	f.lastPrompt = prompt
	if f.err != nil {
		return "", f.err
	}
	return f.response, nil
}

func TestLoadCVTextSuccess(t *testing.T) {
	tmpDir := t.TempDir()
	cvPath := filepath.Join(tmpDir, "cv.txt")
	if err := os.WriteFile(cvPath, []byte("My CV text"), 0o644); err != nil {
		t.Fatalf("write temp cv: %v", err)
	}

	got, err := LoadCVText(cvPath)
	if err != nil {
		t.Fatalf("LoadCVText() error = %v", err)
	}
	if got != "My CV text" {
		t.Fatalf("unexpected CV text: %q", got)
	}
}

func TestAnalyzeJobSuccessWithWrappedJSON(t *testing.T) {
	desc := "Need Go backend and PostgreSQL experience."
	job := &models.Job{
		ID:          10,
		Title:       "Backend Engineer",
		URL:         "https://www.onlinejobs.ph/jobseekers/job/foo-123",
		Description: &desc,
	}

	fake := &fakeGenerator{
		response: "Here is the result:\n{\"fit\":true,\"score\":88,\"reason\":\"Strong Go and backend alignment.\"}\nDone.",
	}
	analyzer := &Analyzer{
		client: fake,
		CVText: "Go developer with API and PostgreSQL experience",
	}

	res, err := analyzer.AnalyzeJob(job, "")
	if err != nil {
		t.Fatalf("AnalyzeJob() error = %v", err)
	}
	if !res.Fit || res.Score != 88 {
		t.Fatalf("unexpected result: %+v", res)
	}
	if res.Reason == "" {
		t.Fatalf("expected reason to be populated")
	}
	if !strings.Contains(fake.lastPrompt, "Backend Engineer") {
		t.Fatalf("prompt did not include job title")
	}
}

func TestAnalyzeJobReturnsErrorOnInvalidJSON(t *testing.T) {
	job := &models.Job{
		Title: "Backend Engineer",
		URL:   "https://example.com",
	}
	fake := &fakeGenerator{response: "no-json-here"}
	analyzer := &Analyzer{
		client: fake,
		CVText: "Some cv",
	}

	_, err := analyzer.AnalyzeJob(job, "")
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
}

func TestAnalyzeJobPropagatesGeneratorError(t *testing.T) {
	job := &models.Job{
		Title: "Backend Engineer",
		URL:   "https://example.com",
	}
	fake := &fakeGenerator{err: errors.New("network down")}
	analyzer := &Analyzer{
		client: fake,
		CVText: "Some cv",
	}

	_, err := analyzer.AnalyzeJob(job, "")
	if err == nil || !strings.Contains(err.Error(), "network down") {
		t.Fatalf("expected wrapped generator error, got %v", err)
	}
}

func TestNewAnalyzerLoadsCV(t *testing.T) {
	tmpDir := t.TempDir()
	cvPath := filepath.Join(tmpDir, "cv.txt")
	if err := os.WriteFile(cvPath, []byte("CV from file"), 0o644); err != nil {
		t.Fatalf("write temp cv: %v", err)
	}

	a, err := NewAnalyzer(&fakeGenerator{response: `{"fit":false,"score":10,"reason":"test"}`}, cvPath)
	if err != nil {
		t.Fatalf("NewAnalyzer() error = %v", err)
	}
	if a.CVText != "CV from file" {
		t.Fatalf("unexpected CV text loaded: %q", a.CVText)
	}
}
