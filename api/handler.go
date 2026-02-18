package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"

	"ESP-data/config"
	"ESP-data/internal/graph"
	"ESP-data/internal/nebula"

	nebulago "github.com/vesoft-inc/nebula-go/v3"
)

// GraphHandler returns an http.HandlerFunc that queries Nebula and writes CyGraph JSON.
// This satisfies REQ-122 (JSON output) and REQ-131 (JSON format for API responses).
func GraphHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: received request from %s %s", requestStart.Format("15:04:05.000"), r.Method, r.URL.Path)

		// Query Nebula for asset connectivity
		rows, err := nebula.QueryAssets(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: query failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query database", http.StatusInternalServerError)
			return
		}

		// Build Cytoscape graph from query results
		cyGraph := graph.BuildGraph(rows)
		log.Printf("[%s] api: built graph with %d nodes, %d edges", time.Now().Format("15:04:05.000"), len(cyGraph.Nodes), len(cyGraph.Edges))

		// Marshal to JSON
		jsonData, err := json.Marshal(cyGraph)
		if err != nil {
			log.Printf("[%s] api: JSON marshal failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}

		// Write JSON response
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			log.Printf("[%s] api: failed to write response: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: response sent successfully (%d bytes) in %.3f seconds", time.Now().Format("15:04:05.000"), len(jsonData), requestDuration.Seconds())
	}
}

// AssetsHandler returns asset list with details for sidebar (REQ-021).
func AssetsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: /api/assets request", requestStart.Format("15:04:05.000"))

		assets, err := nebula.QueryAssetsWithDetails(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QueryAssetsWithDetails failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query assets", http.StatusInternalServerError)
			return
		}

		response := graph.BuildAssetsList(assets, len(assets))

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d assets in %.3f seconds", time.Now().Format("15:04:05.000"), len(assets), requestDuration.Seconds())
	}
}

// AssetDetailHandler returns detail for single asset (REQ-022).
func AssetDetailHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		// Extract asset ID from URL path: /api/asset/{id}
		parts := strings.Split(r.URL.Path, "/")
		log.Printf("[%s] api: URL path: %s, parts: %v, len: %d", requestStart.Format("15:04:05.000"), r.URL.Path, parts, len(parts))

		if len(parts) < 4 {
			http.Error(w, "Invalid asset ID", http.StatusBadRequest)
			return
		}
		assetID := parts[3]

		// Validate asset ID is not empty
		if assetID == "" {
			log.Printf("[%s] api: ERROR - empty assetID extracted from path", requestStart.Format("15:04:05.000"))
			http.Error(w, "Asset ID cannot be empty", http.StatusBadRequest)
			return
		}

		log.Printf("[%s] api: /api/asset/%s request (extracted assetID: '%s')", requestStart.Format("15:04:05.000"), assetID, assetID)

		detail, err := nebula.QueryAssetDetail(pool, cfg, assetID)
		if err != nil {
			log.Printf("[%s] api: QueryAssetDetail failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Asset not found", http.StatusNotFound)
			return
		}

		response := graph.BuildAssetDetailResponse(detail)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned detail for %s in %.3f seconds", time.Now().Format("15:04:05.000"), assetID, requestDuration.Seconds())
	}
}

// NeighborsHandler returns neighbors for inspector panel (REQ-023).
func NeighborsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		// Extract asset ID from URL path: /api/neighbors/{id}
		parts := strings.Split(r.URL.Path, "/")
		log.Printf("[%s] api: URL path: %s, parts: %v, len: %d", requestStart.Format("15:04:05.000"), r.URL.Path, parts, len(parts))

		if len(parts) < 4 {
			http.Error(w, "Invalid asset ID", http.StatusBadRequest)
			return
		}
		assetID := parts[3]

		// Validate asset ID is not empty
		if assetID == "" {
			log.Printf("[%s] api: ERROR - empty assetID extracted from path", requestStart.Format("15:04:05.000"))
			http.Error(w, "Asset ID cannot be empty", http.StatusBadRequest)
			return
		}

		log.Printf("[%s] api: /api/neighbors/%s request (extracted assetID: '%s')", requestStart.Format("15:04:05.000"), assetID, assetID)

		neighbors, err := nebula.QueryNeighbors(pool, cfg, assetID)
		if err != nil {
			log.Printf("[%s] api: QueryNeighbors failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query neighbors", http.StatusInternalServerError)
			return
		}

		response := graph.BuildNeighborsList(neighbors)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d neighbors for %s in %.3f seconds", time.Now().Format("15:04:05.000"), len(neighbors), assetID, requestDuration.Seconds())
	}
}

// AssetTypesHandler returns asset types for filter dropdown (REQ-024).
func AssetTypesHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: /api/asset-types request", requestStart.Format("15:04:05.000"))

		types, err := nebula.QueryAssetTypes(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QueryAssetTypes failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query asset types", http.StatusInternalServerError)
			return
		}

		response := graph.BuildAssetTypesList(types)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d asset types in %.3f seconds", time.Now().Format("15:04:05.000"), len(types), requestDuration.Seconds())
	}
}
