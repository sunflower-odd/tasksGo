package main

import (
	"context"
	"fmt"
	"gateway/internal/api"
	"ledger"
	"net/http"
	"os"
)

func checkHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func main() {
	ctx := context.Background()

	svc, closeFn, err := ledger.NewService(ctx)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Could not init ledger: %v\n", err)
		os.Exit(1)
	}
	defer closeFn()
	fmt.Println("Connected to database")

	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", checkHealth)

	handler := api.NewHandler(svc)
	api.RegisterRoutes(mux, handler)

	logger := api.Logger(mux)
	err = http.ListenAndServe(":8083", logger)
	if err != nil {
		panic(err)
	}
}
