package api

import "net/http"

func RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/transactions", createTransaction)
	mux.HandleFunc("GET /api/transactions", getTransactions)
	mux.HandleFunc("POST /api/budgets", createBudget)
	mux.HandleFunc("GET /api/budgets", getBudgets)
	mux.HandleFunc("GET /api/reports/summary", getReportSummary)
}
