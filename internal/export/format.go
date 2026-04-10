package export

import (
	"strconv"
	"time"
)

func formatTimePtr(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}

func ptrString(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func ptrIntString(p *int) string {
	if p == nil {
		return ""
	}
	return strconv.Itoa(*p)
}
