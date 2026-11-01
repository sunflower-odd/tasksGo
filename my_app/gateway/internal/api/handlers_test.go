package api

import (
	"encoding/json"
	"fmt"
	"ledger"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const jsonCT = "application/json; charset=utf-8"

func newTestMux() *http.ServeMux {
	mux := http.NewServeMux()
	RegisterRoutes(mux)
	return mux
}

func do(t *testing.T, h http.Handler, method, path, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec
}

// testCategory пропускает тест, если БД недоступна, и возвращает уникальную
// категорию, данные которой удаляются после теста.
func testCategory(t *testing.T) string {
	t.Helper()
	if _, err := ledger.ListBudgets(); err != nil {
		t.Skipf("database is not available: %v", err)
	}
	category := fmt.Sprintf("test-%d", time.Now().UnixNano())
	t.Cleanup(func() { _ = ledger.DeleteCategory(category) })
	return category
}

func countByCategory(t *testing.T, category string) int {
	t.Helper()
	list, err := ledger.ListTransactions()
	require.NoError(t, err)
	n := 0
	for _, tx := range list {
		if tx.Category == category {
			n++
		}
	}
	return n
}

func TestCreateAndListBudgets(t *testing.T) {
	ledger.Reset()
	t.Cleanup(ledger.Reset)
	category := testCategory(t)
	mux := newTestMux()

	// POST
	rec := do(t, mux, http.MethodPost, "/api/budgets", fmt.Sprintf(`{"category":%q,"limit":5000}`, category))
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

	var created BudgetResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	assert.Equal(t, category, created.Category)
	assert.Equal(t, 5000.0, created.Limit)

	// GET (period в БД не хранится, сравниваем категорию и лимит)
	rec = do(t, mux, http.MethodGet, "/api/budgets", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

	var list []BudgetResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&list))
	found := false
	for _, b := range list {
		if b.Category == category && b.Limit == 5000.0 {
			found = true
		}
	}
	assert.True(t, found, "budget %q not found in list", category)
}

func TestCreateBudgetBadRequest(t *testing.T) {
	tests := []struct {
		name string
		body string
	}{
		{"invalid json", `{"category":`},
		{"zero limit", `{"category":"еда","limit":0}`},
		{"negative limit", `{"category":"еда","limit":-5}`},
		{"empty category", `{"category":"","limit":100}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ledger.Reset()
			t.Cleanup(ledger.Reset)

			rec := do(t, newTestMux(), http.MethodPost, "/api/budgets", tt.body)
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

			var resp map[string]string
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			assert.NotEmpty(t, resp["error"])
		})
	}
}

func TestTransactionsFlow(t *testing.T) {
	ledger.Reset()
	t.Cleanup(ledger.Reset)
	category := testCategory(t)
	mux := newTestMux()

	rec := do(t, mux, http.MethodPost, "/api/budgets", fmt.Sprintf(`{"category":%q,"limit":5000}`, category))
	require.Equal(t, http.StatusCreated, rec.Code)

	t.Run("ok", func(t *testing.T) {
		rec := do(t, mux, http.MethodPost, "/api/transactions",
			fmt.Sprintf(`{"amount":3000,"category":%q,"description":"продукты","date":"2026-10-30"}`, category))
		assert.Equal(t, http.StatusCreated, rec.Code)
		assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

		var created TransactionResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
		assert.Equal(t, 3000.0, created.Amount)
		assert.Equal(t, "2026-10-30", created.Date)

		rec = do(t, mux, http.MethodGet, "/api/transactions", "")
		assert.Equal(t, http.StatusOK, rec.Code)
		assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

		var list []TransactionResponse
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&list))
		assert.Contains(t, list, created)
	})

	t.Run("exceeded", func(t *testing.T) {
		// 3000 + 2500 > 5000
		rec := do(t, mux, http.MethodPost, "/api/transactions",
			fmt.Sprintf(`{"amount":2500,"category":%q,"date":"2026-10-30"}`, category))
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

		var resp map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, "budget exceeded", resp["error"])
		assert.Equal(t, 1, countByCategory(t, category))
	})

	t.Run("bad_json", func(t *testing.T) {
		rec := do(t, mux, http.MethodPost, "/api/transactions", `{"amount":`)
		assert.Equal(t, http.StatusBadRequest, rec.Code)
		assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

		var resp map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.NotEmpty(t, resp["error"])
	})
}

func TestReportSummary(t *testing.T) {
	category := testCategory(t)
	mux := newTestMux()

	// уникальная дата, чтобы не попасть на закешированный ранее отчёт
	day := fmt.Sprintf("%04d-01-01", 2100+time.Now().UnixNano()%800)

	rec := do(t, mux, http.MethodPost, "/api/transactions",
		fmt.Sprintf(`{"amount":700,"category":%q,"date":%q}`, category, day))
	require.Equal(t, http.StatusCreated, rec.Code)

	rec = do(t, mux, http.MethodGet, "/api/reports/summary?from="+day+"&to="+day, "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

	var report ledger.Report
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&report))
	assert.Equal(t, day, report.StartDate)
	assert.Equal(t, day, report.EndDate)
	assert.Contains(t, report.Totals, category)
}

func TestReportSummaryBadRequest(t *testing.T) {
	tests := []struct {
		name  string
		query string
	}{
		{"no params", ""},
		{"bad format", "?from=2026/10/01&to=2026-10-31"},
		{"from after to", "?from=2026-10-31&to=2026-10-01"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rec := do(t, newTestMux(), http.MethodGet, "/api/reports/summary"+tt.query, "")
			assert.Equal(t, http.StatusBadRequest, rec.Code)
			assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

			var resp map[string]string
			require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
			assert.NotEmpty(t, resp["error"])
		})
	}
}

