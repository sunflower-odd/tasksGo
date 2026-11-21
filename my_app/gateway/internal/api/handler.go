package api

import (
	"context"
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

func writeContextError(w http.ResponseWriter, r *http.Request) bool {
	err := r.Context().Err()
	if errors.Is(err, context.DeadlineExceeded) ||
		errors.Is(err, context.Canceled) {
		writeError(w, http.StatusGatewayTimeout, "request canceled or timed out")
		return true
	}
	return false
}

// HTTP-обработчики
type Handler struct {
	svc ledger.Service
}

func NewHandler(svc ledger.Service) *Handler {
	return &Handler{svc: svc}
}

func (h *Handler) createTransaction(w http.ResponseWriter, r *http.Request) {
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

	created, err := h.svc.AddTransaction(r.Context(), tx)
	if writeContextError(w, r) {
		return
	}
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

func (h *Handler) createBudget(w http.ResponseWriter, r *http.Request) {
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

	err = h.svc.SetBudget(r.Context(), b)
	if writeContextError(w, r) {
		return
	}
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, BudgetFromDomain(b))
}

func (h *Handler) getTransactions(w http.ResponseWriter, r *http.Request) {
	transactions, err := h.svc.ListTransactions(r.Context())
	if writeContextError(w, r) {
		return
	}
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

func (h *Handler) getBudgets(w http.ResponseWriter, r *http.Request) {
	budgets, err := h.svc.ListBudgets(r.Context())
	if writeContextError(w, r) {
		return
	}
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
func (h *Handler) getReportSummary(w http.ResponseWriter, r *http.Request) {
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	if from == "" || to == "" {
		writeError(w, http.StatusBadRequest, "query parameters from and to are required (YYYY-MM-DD)")
		return
	}
	report, err := h.svc.GetReportSummary(r.Context(), from, to)
	if writeContextError(w, r) {
		return
	}
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
