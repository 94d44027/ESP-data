package main

import (
	"log"
	"net/http"

	"ESP-data/api"
	"ESP-data/config"
	"ESP-data/internal/nebula"
)

func main() {
	cfg := config.Load()
	pool := nebula.NewPool(cfg)
	defer pool.Close()

	// Updated: pass both pool and cfg to GraphHandler
	http.HandleFunc("/api/graph", api.GraphHandler(pool, cfg))
	http.Handle("/", http.FileServer(http.Dir("static")))

	log.Printf("listening on :%d", cfg.AppPort)
	log.Fatal(http.ListenAndServe(":8080", nil))
}
