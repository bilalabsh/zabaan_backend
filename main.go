package main

import (
	"log"
	"net/http"

	"github.com/bilalabsh/zabaan_backend/internal/config"
	"github.com/bilalabsh/zabaan_backend/internal/database"
	"github.com/bilalabsh/zabaan_backend/internal/handlers"
)

func main() {
	// Load configuration
	cfg := config.Load()

	// Initialize database
	database.Init()
	defer database.Close()

	// Setup routes
	mux := http.NewServeMux()
	mux.HandleFunc("/", handlers.Root)
	mux.HandleFunc("/health", handlers.HealthCheck)

	// Start server
	serverAddr := ":" + cfg.Port
	log.Printf("localhost running on %s", cfg.Port)
	
	if err := http.ListenAndServe(serverAddr, mux); err != nil {
		log.Fatal("Server failed to start:", err)
	}
}

