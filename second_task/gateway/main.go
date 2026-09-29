package main

import "net/http"

func checkHealth(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("pong"))
}

func main() {
	http.HandleFunc("/ping", checkHealth)

	http.ListenAndServe(":8080", nil)
}
