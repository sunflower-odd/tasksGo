package main

import (
	"gateway/internal/api"
	"net/http"
)

func checkHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /ping", checkHealth)
	api.RegisterRoutes(mux)

	handler := api.Logger(mux) // логирование

	err := http.ListenAndServe(":8083", handler)
	if err != nil {
		panic(err)
	}
}
