package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	"ESP-data/config"
	"ESP-data/internal/graph"
	"ESP-data/internal/nebula"

	nebulago "github.com/vesoft-inc/nebula-go/v3"
)

// validAssetID matches the Asset ID format defined in the schema (e.g. "A00012").
// Used by REQ-025 to reject malformed or injected input before it reaches nGQL.
var validAssetID = regexp.MustCompile(`^A\d{4,5}$`)

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

		// Extract and validate asset ID from URL path: /api/asset/{id}
		assetID, err := extractAssetID(r.URL.Path, 3)
		if err != nil {
			log.Printf("[%s] api: /api/asset/ bad request: %v", requestStart.Format("15:04:05.000"), err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("[%s] api: /api/asset/%s request", requestStart.Format("15:04:05.000"), assetID)

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

		// Extract and validate asset ID from URL path: /api/neighbors/{id}
		assetID, err := extractAssetID(r.URL.Path, 3)
		if err != nil {
			log.Printf("[%s] api: /api/neighbors/ bad request: %v", requestStart.Format("15:04:05.000"), err)
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("[%s] api: /api/neighbors/%s request", requestStart.Format("15:04:05.000"), assetID)

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

// EdgesHandler returns all connects_to edge properties between two assets
// for the edge inspector panel (REQ-026, UI-REQ-212).
func EdgesHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		// Extract and validate both asset IDs from URL path: /api/edges/{sourceId}/{targetId}
		// REQ-025: validate before query execution
		sourceID, err := extractAssetID(r.URL.Path, 3)
		if err != nil {
			log.Printf("[%s] api: /api/edges/ bad source: %v", requestStart.Format("15:04:05.000"), err)
			http.Error(w, "Invalid source asset ID: "+err.Error(), http.StatusBadRequest)
			return
		}
		targetID, err := extractAssetID(r.URL.Path, 4)
		if err != nil {
			log.Printf("[%s] api: /api/edges/ bad target: %v", requestStart.Format("15:04:05.000"), err)
			http.Error(w, "Invalid target asset ID: "+err.Error(), http.StatusBadRequest)
			return
		}

		log.Printf("[%s] api: /api/edges/%s/%s request", requestStart.Format("15:04:05.000"), sourceID, targetID)

		// Fetch edge connections and both asset details in parallel concept,
		// but sequential here for simplicity — three fast queries.
		connections, err := nebula.QueryEdgeConnections(pool, cfg, sourceID, targetID)
		if err != nil {
			log.Printf("[%s] api: QueryEdgeConnections failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query edge connections", http.StatusInternalServerError)
			return
		}

		srcDetail, err := nebula.QueryAssetDetail(pool, cfg, sourceID)
		if err != nil {
			log.Printf("[%s] api: QueryAssetDetail(source) failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Source asset not found", http.StatusNotFound)
			return
		}

		dstDetail, err := nebula.QueryAssetDetail(pool, cfg, targetID)
		if err != nil {
			log.Printf("[%s] api: QueryAssetDetail(target) failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Target asset not found", http.StatusNotFound)
			return
		}

		response := graph.BuildEdgeDetailResponse(srcDetail, dstDetail, connections)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d connections for %s -> %s in %.3f seconds",
			time.Now().Format("15:04:05.000"), len(connections), sourceID, targetID, requestDuration.Seconds())
	}
}

// extractAssetID pulls the asset ID from the given URL path segment,
// validates it against the expected format (REQ-025), and returns
// it or a descriptive error for an HTTP 400 response.
func extractAssetID(urlPath string, segmentIndex int) (string, error) {
	parts := strings.Split(urlPath, "/")
	if len(parts) <= segmentIndex {
		return "", fmt.Errorf("missing asset ID in path")
	}

	assetID := parts[segmentIndex]
	if assetID == "" {
		return "", fmt.Errorf("asset ID cannot be empty")
	}

	if !validAssetID.MatchString(assetID) {
		return "", fmt.Errorf("invalid asset ID format: %q (expected pattern like A00012)", assetID)
	}

	return assetID, nil
}
