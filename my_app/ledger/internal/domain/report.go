package domain

import (
	"context"
	"errors"
)

// {"from":"2026-10-01","to":"2026-10-31","totals":{"food":3000}}
type Report struct {
	StartDate string             `json:"from"`
	EndDate   string             `json:"to"`
	Totals    map[string]float64 `json:"totals"`
}

var ErrInvalidPeriod = errors.New("invalid report period")

type ReportCache interface {
	Get(ctx context.Context, from, to string) (Report, bool) // bool = нашли в кеше
	Set(ctx context.Context, r Report)
}
