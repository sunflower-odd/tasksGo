package api

import (
	"context"
	"net/http"
	"time"
)

const defaultRequestTimeout = 2 * time.Second

func AddTimeout(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), defaultRequestTimeout)
		defer cancel()

		next.ServeHTTP(w, r.WithContext(ctx))
	})
}
