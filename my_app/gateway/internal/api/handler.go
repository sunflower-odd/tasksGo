package api

import (
	"encoding/json"
	"errors"
	"ledger"
	"log"
	"net/http"
	"time"
)

// Content-Type: application/json; charset=utf-8 для ЛЮБОГО JSON-ответа
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func Logger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		next.ServeHTTP(w, r) // передаём запрос дальше

		duration := time.Since(start)
		log.Printf("%s %s %s", r.Method, r.URL.Path, duration)
	})
}

func createTransaction(w http.ResponseWriter, r *http.Request) {
	// читаем
	var req CreateTransactionRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// DTO → доменная модель
	tx, err := req.TransactionToDomain()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	// валидация
	if err := tx.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	created, err := ledger.CreateTransaction(tx)
	if err != nil {
		if errors.Is(err, ledger.ErrBudgetExceeded) {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, TransactionFromDomain(created))
}

func createBudget(w http.ResponseWriter, r *http.Request) {
	var req CreateBudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	b, err := req.BudgetToDomain()
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := b.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := ledger.SetBudget(b); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, BudgetFromDomain(b))
}

func getTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := ledger.ListTransactions()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := []TransactionResponse{}
	for _, tx := range transactions {
		resp = append(resp, TransactionFromDomain(tx))
	}

	writeJSON(w, http.StatusOK, resp)
}

func getBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := ledger.ListBudgets()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	resp := []BudgetResponse{}
	for _, b := range budgets {
		resp = append(resp, BudgetFromDomain(b))
	}

	writeJSON(w, http.StatusOK, resp)
}

// GET /api/reports/summary?from=YYYY-MM-DD&to=YYYY-MM-DD
// Суммы трат по категориям за период. Результат кешируется в Redis внутри ledger.
func getReportSummary(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "query parameters from and to are required (YYYY-MM-DD)")
		return
	}

	report, err := ledger.GetReportSummary(r.Context(), from, to)
	if err != nil {
		if errors.Is(err, ledger.ErrInvalidPeriod) {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, report)
}
