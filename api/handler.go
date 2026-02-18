package api

import (
	"ESP-data/config"
	"ESP-data/internal/graph"
	"ESP-data/internal/nebula"
	"encoding/json"
	"log"
	"net/http"

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

// AssetsHandler serves the asset list for the sidebar (REQ-021).
func AssetsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: assets list request %s %s", r.Method, r.URL.Path)

		items, err := nebula.QueryAssetsWithDetails(pool, cfg)
		if err != nil {
			log.Printf("api: QueryAssetsWithDetails failed: %v", err)
			http.Error(w, "Failed to query assets", http.StatusInternalServerError)
			return
		}

		resp := graph.BuildAssetsList(items, len(items))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("api: assets list JSON encode failed: %v", err)
		}
	}
}

// AssetDetailHandler serves a single asset detail (REQ-022).
func AssetDetailHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: asset detail request %s %s", r.Method, r.URL.Path)

		// URL is /api/asset/{id}
		id := r.URL.Path[len("/api/asset/"):]
		if id == "" {
			http.Error(w, "missing asset id", http.StatusBadRequest)
			return
		}

		detail, err := nebula.QueryAssetDetail(pool, cfg, id)
		if err != nil {
			log.Printf("api: QueryAssetDetail failed: %v", err)
			http.Error(w, "Failed to query asset detail", http.StatusInternalServerError)
			return
		}

		resp := graph.BuildAssetDetailResponse(detail)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("api: asset detail JSON encode failed: %v", err)
		}
	}
}

// NeighborsHandler serves neighbors for an asset (REQ-023).
func NeighborsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: neighbors request %s %s", r.Method, r.URL.Path)

		// URL is /api/neighbors/{id}
		id := r.URL.Path[len("/api/neighbors/"):]
		if id == "" {
			http.Error(w, "missing asset id", http.StatusBadRequest)
			return
		}

		neighbors, err := nebula.QueryNeighbors(pool, cfg, id)
		if err != nil {
			log.Printf("api: QueryNeighbors failed: %v", err)
			http.Error(w, "Failed to query neighbors", http.StatusInternalServerError)
			return
		}

		resp := graph.BuildNeighborsList(neighbors)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("api: neighbors JSON encode failed: %v", err)
		}
	}
}

// AssetTypesHandler serves asset types with counts (REQ-024).
func AssetTypesHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: asset types request %s %s", r.Method, r.URL.Path)

		types, err := nebula.QueryAssetTypes(pool, cfg)
		if err != nil {
			log.Printf("api: QueryAssetTypes failed: %v", err)
			http.Error(w, "Failed to query asset types", http.StatusInternalServerError)
			return
		}

		resp := graph.BuildAssetTypesList(types)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			log.Printf("api: asset types JSON encode failed: %v", err)
		}
	}
}
