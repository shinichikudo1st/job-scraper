package export

import (
	"fmt"

	"github.com/shinichikudo1st/job-scraper/internal/models"
	"github.com/xuri/excelize/v2"
)

func ExportExcel(jobs []models.Job) ([]byte, error) {
	f := excelize.NewFile()
	defer func() { _ = f.Close() }()

	sheet := "Sheet1"
	const headerRow = 1
	headers := []string{"id", "title", "company", "salary", "url", "match_score", "match_reason", "posted_at"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, headerRow)
		if err != nil {
			return nil, err
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return nil, err
		}
	}

	for r, j := range jobs {
		rowIdx := headerRow + 1 + r
		values := []any{
			j.ID,
			j.Title,
			ptrString(j.Company),
			ptrString(j.Salary),
			j.URL,
			ptrIntExcel(j.MatchScore),
			ptrString(j.MatchReason),
			formatTimePtr(j.PostedAt),
		}
		for c, v := range values {
			cell, err := excelize.CoordinatesToCellName(c+1, rowIdx)
			if err != nil {
				return nil, err
			}
			if err := f.SetCellValue(sheet, cell, v); err != nil {
				return nil, err
			}
		}
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("write excel buffer: %w", err)
	}
	return buf.Bytes(), nil
}

func ptrIntExcel(p *int) any {
	if p == nil {
		return ""
	}
	return *p
}
