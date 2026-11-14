package api

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.HandleFunc("POST /api/transactions", h.createTransaction)
	mux.HandleFunc("GET /api/transactions", h.getTransactions)
	mux.HandleFunc("POST /api/budgets", h.createBudget)
	mux.HandleFunc("GET /api/budgets", h.getBudgets)
	mux.HandleFunc("GET /api/reports/summary", h.getReportSummary)
}
