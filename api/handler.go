package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"

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

// AssetsHandler returns asset list with details for sidebar (REQ-021).
func AssetsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: /api/assets request")

		assets, err := nebula.QueryAssetsWithDetails(pool, cfg)
		if err != nil {
			log.Printf("api: QueryAssetsWithDetails failed: %v", err)
			http.Error(w, "Failed to query assets", http.StatusInternalServerError)
			return
		}

		response := graph.BuildAssetsList(assets, len(assets))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("api: JSON encode failed: %v", err)
		}

		log.Printf("api: returned %d assets", len(assets))
	}
}

// AssetDetailHandler returns detail for single asset (REQ-022).
func AssetDetailHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract asset ID from URL path: /api/asset/{id}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid asset ID", http.StatusBadRequest)
			return
		}
		assetID := parts[3]

		log.Printf("api: /api/asset/%s request", assetID)

		detail, err := nebula.QueryAssetDetail(pool, cfg, assetID)
		if err != nil {
			log.Printf("api: QueryAssetDetail failed: %v", err)
			http.Error(w, "Asset not found", http.StatusNotFound)
			return
		}

		response := graph.BuildAssetDetailResponse(detail)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("api: JSON encode failed: %v", err)
		}

		log.Printf("api: returned detail for %s", assetID)
	}
}

// NeighborsHandler returns neighbors for inspector panel (REQ-023).
func NeighborsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract asset ID from URL path: /api/neighbors/{id}
		parts := strings.Split(r.URL.Path, "/")
		if len(parts) < 4 {
			http.Error(w, "Invalid asset ID", http.StatusBadRequest)
			return
		}
		assetID := parts[3]

		log.Printf("api: /api/neighbors/%s request", assetID)

		neighbors, err := nebula.QueryNeighbors(pool, cfg, assetID)
		if err != nil {
			log.Printf("api: QueryNeighbors failed: %v", err)
			http.Error(w, "Failed to query neighbors", http.StatusInternalServerError)
			return
		}

		response := graph.BuildNeighborsList(neighbors)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("api: JSON encode failed: %v", err)
		}

		log.Printf("api: returned %d neighbors for %s", len(neighbors), assetID)
	}
}

// AssetTypesHandler returns asset types for filter dropdown (REQ-024).
func AssetTypesHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: /api/asset-types request")

		types, err := nebula.QueryAssetTypes(pool, cfg)
		if err != nil {
			log.Printf("api: QueryAssetTypes failed: %v", err)
			http.Error(w, "Failed to query asset types", http.StatusInternalServerError)
			return
		}

		response := graph.BuildAssetTypesList(types)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("api: JSON encode failed: %v", err)
		}

		log.Printf("api: returned %d asset types", len(types))
	}
}
