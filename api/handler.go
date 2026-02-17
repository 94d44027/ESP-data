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

// GraphHandler returns an http.HandlerFunc for GET /api/graph (REQ-020).
// Queries Nebula for enriched asset connectivity and returns Cytoscape.js JSON.
// This satisfies REQ-122 (JSON output) and REQ-131 (JSON format for API responses).
func GraphHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: GET /api/graph from %s", r.RemoteAddr)

		// Query Nebula for enriched asset connectivity (REQ-020)
		rows, err := nebula.QueryAssets(pool, cfg)
		if err != nil {
			log.Printf("api: QueryAssets failed: %v", err)
			http.Error(w, "Failed to query database", http.StatusInternalServerError)
			return
		}

		// Build Cytoscape graph with enriched node data (UI-REQ-200)
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
		log.Printf("api: /api/graph response sent (%d bytes)", len(jsonData))
	}
}

// AssetsListHandler returns an http.HandlerFunc for GET /api/assets (REQ-021).
// Returns list of all assets with optional filtering for sidebar entity browser.
func AssetsListHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: GET /api/assets from %s", r.RemoteAddr)

		// Parse optional query parameters for filtering
		assetType := r.URL.Query().Get("type") // e.g., ?type=Server
		search := r.URL.Query().Get("search")  // e.g., ?search=CRM

		// Query Nebula for asset list (REQ-021)
		items, err := nebula.QueryAssetsList(pool, cfg, assetType, search)
		if err != nil {
			log.Printf("api: QueryAssetsList failed: %v", err)
			http.Error(w, "Failed to query assets", http.StatusInternalServerError)
			return
		}

		// Build response with counts (UI-REQ-120)
		// Total count would require a separate query; for PoC, use filtered count
		response := graph.BuildAssetList(items, len(items))
		log.Printf("api: returning %d assets (filtered=%d)", response.Total, response.Filtered)

		// Marshal and send JSON
		jsonData, err := json.Marshal(response)
		if err != nil {
			log.Printf("api: JSON marshal failed: %v", err)
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			log.Printf("api: failed to write response: %v", err)
		}
		log.Printf("api: /api/assets response sent (%d bytes)", len(jsonData))
	}
}

// AssetDetailHandler returns an http.HandlerFunc for GET /api/asset/{id} (REQ-022).
// Returns detailed info for a single asset for the inspector panel.
func AssetDetailHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract asset ID from URL path: /api/asset/{id}
		// Simple path parsing (for production, consider a router like gorilla/mux)
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 4 || pathParts[3] == "" {
			http.Error(w, "Asset ID required", http.StatusBadRequest)
			return
		}
		assetID := pathParts[3]

		log.Printf("api: GET /api/asset/%s from %s", assetID, r.RemoteAddr)

		// Validate asset ID format (REQ-025)
		if !nebula.ValidateAssetID(assetID) {
			log.Printf("api: invalid asset ID format: %s", assetID)
			http.Error(w, "Invalid asset ID format", http.StatusBadRequest)
			return
		}

		// Query Nebula for asset detail (REQ-022)
		detail, err := nebula.QueryAssetDetail(pool, cfg, assetID)
		if err != nil {
			log.Printf("api: QueryAssetDetail failed for %s: %v", assetID, err)
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, "Asset not found", http.StatusNotFound)
			} else {
				http.Error(w, "Failed to query asset detail", http.StatusInternalServerError)
			}
			return
		}

		// Build response (UI-REQ-210)
		response := graph.BuildAssetDetailResponse(detail)

		// Marshal and send JSON
		jsonData, err := json.Marshal(response)
		if err != nil {
			log.Printf("api: JSON marshal failed: %v", err)
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			log.Printf("api: failed to write response: %v", err)
		}
		log.Printf("api: /api/asset/%s response sent (%d bytes)", assetID, len(jsonData))
	}
}

// NeighborsHandler returns an http.HandlerFunc for GET /api/neighbors/{id} (REQ-023).
// Returns immediate neighbors of an asset with direction for inspector connections list.
func NeighborsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Extract asset ID from URL path: /api/neighbors/{id}
		pathParts := strings.Split(r.URL.Path, "/")
		if len(pathParts) < 4 || pathParts[3] == "" {
			http.Error(w, "Asset ID required", http.StatusBadRequest)
			return
		}
		assetID := pathParts[3]

		log.Printf("api: GET /api/neighbors/%s from %s", assetID, r.RemoteAddr)

		// Validate asset ID format (REQ-025)
		if !nebula.ValidateAssetID(assetID) {
			log.Printf("api: invalid asset ID format: %s", assetID)
			http.Error(w, "Invalid asset ID format", http.StatusBadRequest)
			return
		}

		// Query Nebula for neighbors (REQ-023)
		neighbors, err := nebula.QueryNeighbors(pool, cfg, assetID)
		if err != nil {
			log.Printf("api: QueryNeighbors failed for %s: %v", assetID, err)
			http.Error(w, "Failed to query neighbors", http.StatusInternalServerError)
			return
		}

		// Build response (UI-REQ-210 §3-4)
		response := graph.BuildNeighborsList(neighbors)

		// Marshal and send JSON
		jsonData, err := json.Marshal(response)
		if err != nil {
			log.Printf("api: JSON marshal failed: %v", err)
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			log.Printf("api: failed to write response: %v", err)
		}
		log.Printf("api: /api/neighbors/%s response sent (%d bytes)", assetID, len(jsonData))
	}
}

// AssetTypesHandler returns an http.HandlerFunc for GET /api/asset-types (REQ-024).
// Returns all distinct asset types for filter checkboxes in sidebar.
func AssetTypesHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Printf("api: GET /api/asset-types from %s", r.RemoteAddr)

		// Query Nebula for asset types (REQ-024)
		types, err := nebula.QueryAssetTypes(pool, cfg)
		if err != nil {
			log.Printf("api: QueryAssetTypes failed: %v", err)
			http.Error(w, "Failed to query asset types", http.StatusInternalServerError)
			return
		}

		// Build response (UI-REQ-122)
		response := graph.BuildAssetTypesList(types)
		log.Printf("api: returning %d asset types", response.Total)

		// Marshal and send JSON
		jsonData, err := json.Marshal(response)
		if err != nil {
			log.Printf("api: JSON marshal failed: %v", err)
			http.Error(w, "Failed to generate JSON", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write(jsonData); err != nil {
			log.Printf("api: failed to write response: %v", err)
		}
		log.Printf("api: /api/asset-types response sent (%d bytes)", len(jsonData))
	}
}
