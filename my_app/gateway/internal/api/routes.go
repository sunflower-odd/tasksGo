package api

import (
	"net/http"
)

func RegisterRoutes(mux *http.ServeMux, h *Handler) {
	mux.Handle("POST /api/transactions", AddTimeout(http.HandlerFunc(h.createTransaction)))
	mux.Handle("GET /api/transactions", AddTimeout(http.HandlerFunc(h.getTransactions)))
	mux.Handle("POST /api/budgets", AddTimeout(http.HandlerFunc(h.createBudget)))
	mux.Handle("GET /api/budgets", AddTimeout(http.HandlerFunc(h.getBudgets)))
	mux.Handle("GET /api/reports/summary", AddTimeout(http.HandlerFunc(h.getReportSummary)))
}
