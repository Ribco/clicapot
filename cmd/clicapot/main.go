package main

import (
	"log"
	"net/http"
	"os"

	"github.com/Ribco/clicapot/internal/api"
	"github.com/Ribco/clicapot/internal/database"
)

func main() {
	dataDir := os.Getenv("CLICAPOT_DATA")
	if dataDir == "" {
		dataDir = "./data"
	}

	db, err := database.Open(dataDir)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	if err := database.Migrate(db); err != nil {
		log.Fatal(err)
	}

	router := api.NewRouter(db)

	log.Println("☁️ Clicapot v0.1.0")
	log.Println("🚀 Listening on :8000")

	if err := http.ListenAndServe(":8000", router); err != nil {
		log.Fatal(err)
	}
}
