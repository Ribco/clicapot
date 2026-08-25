package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type HealthResponse struct {
	Service string `json:"service"`
	Status  string `json:"status"`
	Version string `json:"version"`
}

func main() {
	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		_ = json.NewEncoder(w).Encode(HealthResponse{
			Service: "clicapot",
			Status:  "ok",
			Version: "0.1.0",
		})
	})

	log.Println("☁️ Clicapot v0.1.0 listening on :8000")

	if err := http.ListenAndServe(":8000", nil); err != nil {
		log.Fatal(err)
	}
}
