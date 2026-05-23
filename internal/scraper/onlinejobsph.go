package scraper

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	DefaultOnlineJobsPHBaseURL = "https://www.onlinejobs.ph"
	defaultUserAgent           = "SmarterOLJ/0.1 (+local job search assistant)"
)

type OnlineJobsPHConfig struct {
	BaseURL          string
	SearchURL        string
	Keywords         []string
	ExcludedKeywords []string
	MaxPages         int
	MinSalary        int
	RequestDelay     time.Duration
	UserAgent        string
}

type JobSummary struct {
	ExternalID string
	Title      string
	URL        string
	Summary    string
	Salary     string
}

type JobDetail struct {
	JobSummary
	Description string
}

type SeenStore interface {
	IsSeen(externalID string) (bool, error)
	MarkSeen(externalID, url, title string) error
}

type OnlineJobsPHScraper struct {
	Config     OnlineJobsPHConfig
	HTTPClient *http.Client
}

func NewOnlineJobsPHScraper(config OnlineJobsPHConfig) *OnlineJobsPHScraper {
	if strings.TrimSpace(config.BaseURL) == "" {
		config.BaseURL = DefaultOnlineJobsPHBaseURL
	}
	if config.MaxPages <= 0 {
		config.MaxPages = 1
	}
	if config.UserAgent == "" {
		config.UserAgent = defaultUserAgent
	}
	return &OnlineJobsPHScraper{
		Config:     config,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
	}
}

func (s *OnlineJobsPHScraper) ScrapeNewDetails(ctx context.Context, seen SeenStore) ([]JobDetail, error) {
	if s == nil {
		return nil, errors.New("scraper is nil")
	}
	if seen == nil {
		return nil, errors.New("seen store is required")
	}

	var details []JobDetail
	for page := 1; page <= s.maxPages(); page++ {
		listingURL, err := s.pageURL(page)
		if err != nil {
			return nil, err
		}
		htmlText, err := s.fetch(ctx, listingURL)
		if err != nil {
			return nil, err
		}
		summaries, err := ParseListingHTML(htmlText, s.baseURL())
		if err != nil {
			return nil, err
		}
		for _, summary := range FilterSummaries(summaries, s.Config) {
			alreadySeen, err := seen.IsSeen(summary.ExternalID)
			if err != nil {
				return nil, err
			}
			if alreadySeen {
				continue
			}
			detail, err := s.FetchDetail(ctx, summary)
			if err != nil {
				return nil, err
			}
			if err := seen.MarkSeen(summary.ExternalID, summary.URL, summary.Title); err != nil {
				return nil, err
			}
			details = append(details, detail)
		}
	}
	return details, nil
}

func (s *OnlineJobsPHScraper) FetchDetail(ctx context.Context, summary JobSummary) (JobDetail, error) {
	if s == nil {
		return JobDetail{}, errors.New("scraper is nil")
	}
	detailHTML, err := s.fetch(ctx, summary.URL)
	if err != nil {
		return JobDetail{}, err
	}
	return ParseDetailHTML(detailHTML, summary)
}

func (s *OnlineJobsPHScraper) fetch(ctx context.Context, targetURL string) (string, error) {
	if s.Config.RequestDelay > 0 {
		timer := time.NewTimer(s.Config.RequestDelay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return "", ctx.Err()
		case <-timer.C:
		}
	}

	client := s.HTTPClient
	if client == nil {
		client = &http.Client{Timeout: 30 * time.Second}
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, targetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", s.userAgent())
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetch %s: status %d", targetURL, resp.StatusCode)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

func (s *OnlineJobsPHScraper) pageURL(page int) (string, error) {
	raw := strings.TrimSpace(s.Config.SearchURL)
	if raw == "" {
		raw = s.baseURL() + "/jobseekers/jobsearch"
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", err
	}
	if !u.IsAbs() {
		base, err := url.Parse(s.baseURL())
		if err != nil {
			return "", err
		}
		u = base.ResolveReference(u)
	}
	q := u.Query()
	if page > 1 {
		q.Set("page", strconv.Itoa(page))
	}
	u.RawQuery = q.Encode()
	return u.String(), nil
}

func (s *OnlineJobsPHScraper) baseURL() string {
	baseURL := strings.TrimRight(strings.TrimSpace(s.Config.BaseURL), "/")
	if baseURL == "" {
		return DefaultOnlineJobsPHBaseURL
	}
	return baseURL
}

func (s *OnlineJobsPHScraper) maxPages() int {
	if s.Config.MaxPages <= 0 {
		return 1
	}
	return s.Config.MaxPages
}

func (s *OnlineJobsPHScraper) userAgent() string {
	if strings.TrimSpace(s.Config.UserAgent) == "" {
		return defaultUserAgent
	}
	return s.Config.UserAgent
}

func ParseListingHTML(htmlText, baseURL string) ([]JobSummary, error) {
	root, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return nil, err
	}

	var out []JobSummary
	seen := map[string]bool{}
	walk(root, func(n *html.Node) {
		if n.Type != html.ElementNode || n.Data != "a" {
			return
		}
		href := attr(n, "href")
		if !looksLikeJobURL(href) {
			return
		}
		fullURL := absoluteURL(baseURL, href)
		externalID := ExternalIDFromURL(fullURL)
		if externalID == "" || seen[externalID] {
			return
		}
		seen[externalID] = true

		title := cleanText(nodeText(n))
		blockText := cleanText(nodeText(bestSummaryBlock(n)))
		if title == "" {
			title = firstLine(blockText)
		}
		out = append(out, JobSummary{
			ExternalID: externalID,
			Title:      title,
			URL:        fullURL,
			Summary:    blockText,
			Salary:     extractSalary(blockText),
		})
	})

	return out, nil
}

func ParseDetailHTML(htmlText string, summary JobSummary) (JobDetail, error) {
	root, err := html.Parse(strings.NewReader(htmlText))
	if err != nil {
		return JobDetail{}, err
	}

	title := cleanText(firstElementText(root, "h1"))
	if title == "" {
		title = summary.Title
	}

	description := cleanText(firstByClassText(root, "job-description"))
	if description == "" {
		description = cleanText(firstElementText(root, "main"))
	}
	if description == "" {
		description = cleanText(nodeText(root))
	}

	summary.Title = title
	return JobDetail{
		JobSummary:  summary,
		Description: description,
	}, nil
}

func FilterSummaries(summaries []JobSummary, config OnlineJobsPHConfig) []JobSummary {
	var out []JobSummary
	for _, summary := range summaries {
		text := strings.ToLower(summary.Title + " " + summary.Summary)
		if len(config.Keywords) > 0 && !containsAny(text, config.Keywords) {
			continue
		}
		if containsAny(text, config.ExcludedKeywords) {
			continue
		}
		if config.MinSalary > 0 && maxNumber(summary.Salary) > 0 && maxNumber(summary.Salary) < config.MinSalary {
			continue
		}
		out = append(out, summary)
	}
	return out
}

func ExternalIDFromURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		rawURL = u.Path
	}
	rawURL = strings.Trim(strings.ToLower(rawURL), "/")
	re := regexp.MustCompile(`([a-z0-9]+(?:-[a-z0-9]+)*-\d+|\d+)$`)
	if match := re.FindString(rawURL); match != "" {
		return match
	}
	sum := sha1.Sum([]byte(rawURL))
	return "url-" + hex.EncodeToString(sum[:])[:16]
}

func looksLikeJobURL(href string) bool {
	href = strings.ToLower(strings.TrimSpace(href))
	return strings.Contains(href, "/jobseekers/job/") || strings.Contains(href, "/job/")
}

func absoluteURL(baseURL, href string) string {
	u, err := url.Parse(strings.TrimSpace(href))
	if err != nil {
		return strings.TrimSpace(href)
	}
	if u.IsAbs() {
		return u.String()
	}
	base, err := url.Parse(strings.TrimRight(baseURL, "/") + "/")
	if err != nil {
		return href
	}
	return base.ResolveReference(u).String()
}

func bestSummaryBlock(n *html.Node) *html.Node {
	cur := n
	for i := 0; i < 4 && cur.Parent != nil; i++ {
		cur = cur.Parent
		if cur.Type == html.ElementNode && (cur.Data == "article" || cur.Data == "li") {
			return cur
		}
		if cur.Type == html.ElementNode && cur.Data == "div" && strings.Contains(attr(cur, "class"), "job") {
			return cur
		}
	}
	return n.Parent
}

func firstElementText(n *html.Node, tag string) string {
	var found string
	walk(n, func(cur *html.Node) {
		if found != "" {
			return
		}
		if cur.Type == html.ElementNode && cur.Data == tag {
			found = nodeText(cur)
		}
	})
	return found
}

func firstByClassText(n *html.Node, className string) string {
	var found string
	walk(n, func(cur *html.Node) {
		if found != "" {
			return
		}
		if cur.Type == html.ElementNode && strings.Contains(attr(cur, "class"), className) {
			found = nodeText(cur)
		}
	})
	return found
}

func walk(n *html.Node, fn func(*html.Node)) {
	if n == nil {
		return
	}
	fn(n)
	for child := n.FirstChild; child != nil; child = child.NextSibling {
		walk(child, fn)
	}
}

func attr(n *html.Node, key string) string {
	for _, a := range n.Attr {
		if a.Key == key {
			return a.Val
		}
	}
	return ""
}

func nodeText(n *html.Node) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	walk(n, func(cur *html.Node) {
		if cur.Type == html.TextNode {
			b.WriteString(cur.Data)
			b.WriteString(" ")
		}
	})
	return b.String()
}

func cleanText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

func firstLine(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	for _, sep := range []string{"\n", " - ", " | "} {
		if idx := strings.Index(s, sep); idx >= 0 {
			return strings.TrimSpace(s[:idx])
		}
	}
	return s
}

func extractSalary(text string) string {
	for _, token := range strings.Split(text, " ") {
		lower := strings.ToLower(token)
		if strings.Contains(lower, "$") || strings.Contains(lower, "php") {
			return token
		}
	}
	return ""
}

func containsAny(text string, needles []string) bool {
	for _, needle := range needles {
		needle = strings.ToLower(strings.TrimSpace(needle))
		if needle != "" && strings.Contains(text, needle) {
			return true
		}
	}
	return false
}

func maxNumber(text string) int {
	re := regexp.MustCompile(`\d[\d,]*`)
	matches := re.FindAllString(text, -1)
	max := 0
	for _, match := range matches {
		n, err := strconv.Atoi(strings.ReplaceAll(match, ",", ""))
		if err == nil && n > max {
			max = n
		}
	}
	return max
}
