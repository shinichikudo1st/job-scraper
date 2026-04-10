package export

import (
	"bytes"
	"encoding/csv"
	"strconv"

	"github.com/shinichikudo1st/job-scraper/internal/models"
)

func ExportCSV(jobs []models.Job) ([]byte, error) {
	buf := &bytes.Buffer{}
	w := csv.NewWriter(buf)
	header := []string{"id", "title", "company", "salary", "url", "match_score", "match_reason", "posted_at"}
	if err := w.Write(header); err != nil {
		return nil, err
	}

	for _, j := range jobs {
		row := []string{
			strconv.Itoa(j.ID),
			j.Title,
			ptrString(j.Company),
			ptrString(j.Salary),
			j.URL,
			ptrIntString(j.MatchScore),
			ptrString(j.MatchReason),
			formatTimePtr(j.PostedAt),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
