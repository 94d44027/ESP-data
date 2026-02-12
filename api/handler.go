package api

import (
	"encoding/json"
	"log"
	"net/http"

	"ESP-data/config"
	"ESP-data/internal/graph"
	"ESP-data/internal/nebula"
	nebulago "github.com/vesoft-inc/nebula-go/v3"
)

// GraphHandler returns an http.HandlerFunc that queries Nebula and writes CyGraph JSON.
// This satisfies REQ-122 (JSON output) and REQ-131 (JSON format for API responses).
func GraphHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: received request from %s %s", r.Method, r.URL.Path)

		// Query Nebula for asset connectivity
		rows, err := nebula.QueryAssets(pool, cfg)
		if err != nil {
			log.Printf("api: query failed: %v", err)
			http.Error(w, "Failed to query database", http.StatusInternalServerError)
			return
		}

		// Build Cytoscape graph from query results
		cyGraph := graph.BuildGraph(rows)
		log.Printf("api: built graph with %d nodes, %d edges", len(cyGraph.Nodes), len(cyGraph.Edges))

		// Marshal to JSON
		jsonData, err := json.Marshal(cyGraph)
		if err != nil {
			log.Printf("api: JSON marshal failed: %v", err)
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}

		// Write JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			log.Printf("api: failed to write response: %v", err)
		}

		log.Printf("api: response sent successfully (%d bytes)", len(jsonData))
	}
}
