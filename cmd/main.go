package main

import (
	"AIChatMatrix/internal/api"
	"AIChatMatrix/internal/config"
	"log"
	"net/http"
	"os"
)

func main() {
	port := config.Get().Port
	if p := os.Getenv("PORT"); p != "" {
		port = p
	}

	router := api.NewRouter()

	// Serve static files from web directory
	fileServer := http.FileServer(http.Dir("web"))

	mux := http.NewServeMux()
	mux.Handle("/api/", router)
	mux.Handle("/", fileServer)

	log.Printf("🤖 AIChatMatrix server running on http://localhost:%s", port)
	if err := http.ListenAndServe(":"+port, mux); err != nil {
		log.Fatal(err)
	}
}
