package main

import (
	"log"
	"net/http"

	"ESP-data/api"
	"ESP-data/config"
	"ESP-data/internal/nebula"
)

func main() {
	// Load configuration from environment variables (REQ-002)
	cfg := config.Load()

	// Initialize Nebula connection pool (REQ-121)
	pool := nebula.NewPool(cfg)
	defer pool.Close()

	// Register API endpoints

	// REQ-020: Enriched graph data for Cytoscape visualization
	http.HandleFunc("/api/graph", api.GraphHandler(pool, cfg))

	// REQ-021: Asset list for sidebar entity browser
	http.HandleFunc("/api/assets", api.AssetsHandler(pool, cfg))

	// REQ-022: Single asset detail for inspector panel
	http.HandleFunc("/api/asset/", api.AssetDetailHandler(pool, cfg))

	// REQ-023: Neighbor list for inspector connections summary
	http.HandleFunc("/api/neighbors/", api.NeighborsHandler(pool, cfg))

	// REQ-024: Asset types for filter checkboxes
	http.HandleFunc("/api/asset-types", api.AssetTypesHandler(pool, cfg))

	// Serve static files (HTML, CSS, JS) from /static directory
	// This serves the VIS layer (REQ-123, UI-Requirements.MD)
	http.Handle("/", http.FileServer(http.Dir("static")))

	// Start HTTP server (REQ-130)
	addr := ":8080"
	log.Printf("ESP PoC starting on %s", addr)
	log.Printf("Configured Nebula: %s:%d, Space: %s", cfg.NebulaHost, cfg.NebulaPort, cfg.Space)
	log.Printf("API endpoints available:")
	log.Printf("  GET /api/graph         - Graph nodes and edges (REQ-020)")
	log.Printf("  GET /api/assets        - Asset list (REQ-021)")
	log.Printf("  GET /api/asset/{id}    - Asset detail (REQ-022)")
	log.Printf("  GET /api/neighbors/{id} - Neighbor list (REQ-023)")
	log.Printf("  GET /api/asset-types   - Asset types (REQ-024)")
	log.Printf("Static files served from ./static/")
	log.Fatal(http.ListenAndServe(addr, nil))
}
