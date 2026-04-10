package matcher

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/shinichikudo1st/job-scraper/internal/models"
)

type promptGenerator interface {
	Generate(prompt string) (string, error)
}

type Analyzer struct {
	client promptGenerator
	CVText string
}

type JobAnalyzer interface {
	AnalyzeJob(job *models.Job, cv string) (*MatchResult, error)
}

type MatchResult struct {
	Fit    bool   `json:"fit"`
	Score  int    `json:"score"`
	Reason string `json:"reason"`
}

func LoadCVText(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read cv file: %w", err)
	}
	cv := strings.TrimSpace(string(b))
	if cv == "" {
		return "", errors.New("cv text is empty")
	}
	return cv, nil
}

func NewAnalyzer(client promptGenerator, cvPath string) (*Analyzer, error) {
	if client == nil {
		return nil, errors.New("analyzer client is required")
	}
	cv, err := LoadCVText(cvPath)
	if err != nil {
		return nil, err
	}
	return &Analyzer{
		client: client,
		CVText: cv,
	}, nil
}

func (a *Analyzer) AnalyzeJob(job *models.Job, cv string) (*MatchResult, error) {
	if a == nil || a.client == nil {
		return nil, errors.New("analyzer is not initialized")
	}
	if job == nil {
		return nil, errors.New("job is required")
	}

	cvText := strings.TrimSpace(cv)
	if cvText == "" {
		cvText = strings.TrimSpace(a.CVText)
	}
	if cvText == "" {
		return nil, errors.New("cv text is required")
	}

	prompt := buildAnalysisPrompt(job, cvText)
	raw, err := a.client.Generate(prompt)
	if err != nil {
		return nil, fmt.Errorf("generate analysis: %w", err)
	}

	if envBool("MATCHER_LOG_MODEL_RAW") {
		preview := raw
		const maxPreview = 800
		if len(preview) > maxPreview {
			preview = preview[:maxPreview] + "…(truncated)"
		}
		log.Printf("matcher: job id=%d model_output_len=%d preview=%q", job.ID, len(raw), preview)
	}

	parsed, err := parseMatchResult(raw)
	if err != nil {
		return nil, fmt.Errorf("parse model output (job_id=%d, raw_len=%d): %w", job.ID, len(raw), err)
	}
	return parsed, nil
}

func buildAnalysisPrompt(job *models.Job, cv string) string {
	description := ""
	if job.Description != nil {
		description = *job.Description
	}

	return fmt.Sprintf(
		`You are a job-matching evaluator. Analyze if this candidate is a good fit for the job.

Your response must be ONLY a valid JSON object with these fields:
- "fit": true or false (boolean)
- "score": 0 to 100 (integer)
- "reason": brief explanation in 1-2 sentences (string)

Example response format:
{"fit": true, "score": 85, "reason": "Candidate has strong Go and PostgreSQL experience matching job requirements."}

Rules:
- Use actual boolean values (true/false), not the word "boolean"
- Use actual numbers for score, not the word "number"
- Write in English only
- No markdown code blocks, just raw JSON

Candidate CV:
%s

Job to evaluate:
Title: %s
URL: %s
Description: %s

Return your JSON evaluation now:`,
		cv, job.Title, job.URL, description,
	)
}

func envBool(key string) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(key)))
	return v == "1" || v == "true" || v == "yes"
}

func parseMatchResult(raw string) (*MatchResult, error) {
	jsonText, err := extractJSONObject(raw)
	if err != nil {
		return nil, fmt.Errorf("extract json result: %w (raw preview: %.200s)", err, raw)
	}

	var result MatchResult
	if err := json.Unmarshal([]byte(jsonText), &result); err != nil {
		return nil, fmt.Errorf("parse match result json: %w (extracted: %.200s)", err, jsonText)
	}

	result.Reason = strings.TrimSpace(result.Reason)
	if result.Score < 0 {
		result.Score = 0
	}
	if result.Score > 100 {
		result.Score = 100
	}
	if result.Reason == "" {
		return nil, errors.New("match reason is required")
	}

	return &result, nil
}

func extractJSONObject(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return "", errors.New("empty model response")
	}

	start := strings.IndexByte(s, '{')
	if start < 0 {
		return "", errors.New("no json object start found")
	}

	depth := 0
	for i := start; i < len(s); i++ {
		switch s[i] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1], nil
			}
		}
	}

	return "", errors.New("no complete json object found")
}
