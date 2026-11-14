package ledger

import (
	"context"

	"ledger/internal/app"
	"ledger/internal/service"
)

// интерфейс сервиса для gatewa
type Service = service.FinanceService

// сборка: БД → репозитории → сервис
func NewService(ctx context.Context) (Service, func() error, error) {
	return app.New(ctx)
}
