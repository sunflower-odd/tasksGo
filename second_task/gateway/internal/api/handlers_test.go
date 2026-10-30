package api

import (
	"encoding/json"
	"ledger"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

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

func TestCreateAndListBudgets(t *testing.T) {
	ledger.Reset()
	t.Cleanup(ledger.Reset)
	mux := newTestMux()

	// POST
	rec := do(t, mux, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
	assert.Equal(t, http.StatusCreated, rec.Code)
	assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

	var created BudgetResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&created))
	assert.Equal(t, "еда", created.Category)
	assert.Equal(t, 5000.0, created.Limit)

	// GET
	rec = do(t, mux, http.MethodGet, "/api/budgets", "")
	assert.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

	var list []BudgetResponse
	require.NoError(t, json.NewDecoder(rec.Body).Decode(&list))
	assert.Contains(t, list, created)
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
	mux := newTestMux()

	rec := do(t, mux, http.MethodPost, "/api/budgets", `{"category":"еда","limit":5000}`)
	require.Equal(t, http.StatusCreated, rec.Code)

	t.Run("ok", func(t *testing.T) {
		rec := do(t, mux, http.MethodPost, "/api/transactions",
			`{"amount":3000,"category":"еда","description":"продукты","date":"2026-10-30"}`)
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
			`{"amount":2500,"category":"еда","date":"2026-10-30"}`)
		assert.Equal(t, http.StatusConflict, rec.Code)
		assert.Equal(t, jsonCT, rec.Header().Get("Content-Type"))

		var resp map[string]string
		require.NoError(t, json.NewDecoder(rec.Body).Decode(&resp))
		assert.Equal(t, "budget exceeded", resp["error"])
		assert.Len(t, ledger.ListTransactions(), 1)
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
