package api

import (
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"regexp"
	"sort"
	"strconv"
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

// validMitigationID matches the Mitigation ID format (e.g. "M1020").
// Used by REQ-038 to reject malformed input before it reaches nGQL.
var validMitigationID = regexp.MustCompile(`^M\d{4}$`)

// validMaturity defines the allowed maturity values per REQ-039.
var validMaturity = map[int]bool{25: true, 50: true, 80: true, 100: true}

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

// AssetHandler dispatches /api/asset/{id}[/mitigations[/{mid}]] requests.
// It routes to asset detail (REQ-022) or mitigations CRUD (REQ-034/035/036)
// based on the URL path structure.
func AssetHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		parts := strings.Split(strings.TrimRight(r.URL.Path, "/"), "/")
		// /api/asset/{id}                       → len 4
		// /api/asset/{id}/mitigations           → len 5
		// /api/asset/{id}/mitigations/{mid}     → len 6

		switch {
		case len(parts) == 4:
			handleAssetDetail(pool, cfg, w, r)
		case len(parts) >= 5 && parts[4] == "mitigations":
			switch r.Method {
			case http.MethodGet:
				handleGetAssetMitigations(pool, cfg, w, r)
			case http.MethodPut:
				handleUpsertAssetMitigation(pool, cfg, w, r)
			case http.MethodDelete:
				if len(parts) < 6 {
					http.Error(w, "Missing mitigation ID for DELETE", http.StatusBadRequest)
					return
				}
				handleDeleteAssetMitigation(pool, cfg, w, r)
			default:
				http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			}
		default:
			http.Error(w, "Not found", http.StatusNotFound)
		}
	}
}

// handleAssetDetail returns detail for single asset (REQ-022).
func handleAssetDetail(pool *nebulago.ConnectionPool, cfg *config.Config, w http.ResponseWriter, r *http.Request) {
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

// EntryPointsHandler returns assets with is_entrance == true (ALG-REQ-002, migrated from REQ-030).
func EntryPointsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: /api/entry-points request", requestStart.Format("15:04:05.000"))

		entries, err := nebula.QueryEntryPoints(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QueryEntryPoints failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query entry points", http.StatusInternalServerError)
			return
		}

		response := graph.BuildEntryPointsList(entries)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d entry points in %.3f seconds", time.Now().Format("15:04:05.000"), len(entries), requestDuration.Seconds())
	}
}

// TargetsHandler returns assets with is_target == true (ALG-REQ-003, migrated from REQ-031).
func TargetsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: /api/targets request", requestStart.Format("15:04:05.000"))

		targets, err := nebula.QueryTargets(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QueryTargets failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query targets", http.StatusInternalServerError)
			return
		}

		response := graph.BuildTargetsList(targets)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d targets in %.3f seconds", time.Now().Format("15:04:05.000"), len(targets), requestDuration.Seconds())
	}
}

// PathsHandler calculates loop-free paths with position-aware TTB
// (ALG-REQ-001, ALG-REQ-010, ALG-REQ-046 v1.3).
func PathsHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		fromID := r.URL.Query().Get("from")
		toID := r.URL.Query().Get("to")
		hopsStr := r.URL.Query().Get("hops")

		if !validAssetID.MatchString(fromID) {
			http.Error(w, fmt.Sprintf("Invalid entry point ID: %q", fromID), http.StatusBadRequest)
			return
		}
		if !validAssetID.MatchString(toID) {
			http.Error(w, fmt.Sprintf("Invalid target ID: %q", toID), http.StatusBadRequest)
			return
		}

		maxHops := 6
		if hopsStr != "" {
			n, err := strconv.Atoi(hopsStr)
			if err != nil || n < 2 || n > 9 {
				http.Error(w, "hops must be an integer between 2 and 9", http.StatusBadRequest)
				return
			}
			maxHops = n
		}

		log.Printf("[%s] api: /api/paths?from=%s&to=%s&hops=%d request",
			requestStart.Format("15:04:05.000"), fromID, toID, maxHops)

		// Step 1: Find paths — returns per-node IDs and stored TTBs (ALG-REQ-001 v1.3)
		pathResults, err := nebula.QueryPaths(pool, cfg, fromID, toID, maxHops)
		if err != nil {
			log.Printf("[%s] api: QueryPaths failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to calculate paths", http.StatusInternalServerError)
			return
		}

		// Step 2: Extract unique asset IDs from all paths (ALG-REQ-046 step 2)
		assetIDSet := make(map[string]bool)
		for _, p := range pathResults {
			for _, id := range p.IDs {
				assetIDSet[id] = true
			}
		}
		uniqueIDs := make([]string, 0, len(assetIDSet))
		for id := range assetIDSet {
			uniqueIDs = append(uniqueIDs, id)
		}

		// Step 3-4: Check hash validity and recalculate stale intermediates (ALG-REQ-046)
		var recalculatedAssets []string
		freshTTBs := make(map[string]int) // asset_id -> latest TTB (Regular_chain)

		if len(uniqueIDs) > 0 {
			validity, fetchedTTBs, err := nebula.QueryAssetHashValidity(pool, cfg, uniqueIDs)
			if err != nil {
				log.Printf("[%s] api: QueryAssetHashValidity failed: %v",
					time.Now().Format("15:04:05.000"), err)
			} else {
				freshTTBs = fetchedTTBs

				// Collect stale IDs — exclude entry and target (they get ephemeral TTB)
				var staleIDs []string
				for _, id := range uniqueIDs {
					if !validity[id] && id != fromID && id != toID {
						staleIDs = append(staleIDs, id)
					}
				}

				if len(staleIDs) > 0 {
					log.Printf("[%s] api: %d intermediate(s) have stale hashes, recalculating",
						requestStart.Format("15:04:05.000"), len(staleIDs))

					staleHashes, err := nebula.QueryScopedStaleHashes(pool, cfg, staleIDs)
					if err != nil {
						log.Printf("[%s] api: QueryScopedStaleHashes failed: %v",
							time.Now().Format("15:04:05.000"), err)
					} else {
						for _, asset := range staleHashes {
							hashStr := fmt.Sprintf("%d", asset.ComputedHash)
							// ALG-REQ-044 v1.3: stub with CHAIN_INTERMEDIATE for intermediates
							newTTB := nebula.ComputeTTBStub(asset.CurrentTTB, "CHAIN_INTERMEDIATE")
							if err := nebula.UpdateAssetTTBAndHash(pool, cfg, asset.AssetID, newTTB, hashStr); err != nil {
								log.Printf("[%s] api: UpdateAssetTTBAndHash failed for %s: %v",
									time.Now().Format("15:04:05.000"), asset.AssetID, err)
								continue
							}
							freshTTBs[asset.AssetID] = newTTB
							recalculatedAssets = append(recalculatedAssets, asset.AssetID)
							log.Printf("[%s] api: path-scoped recalc %s: TTB %d -> %d",
								time.Now().Format("15:04:05.000"), asset.AssetID, asset.CurrentTTB, newTTB)
						}
					}
				}
			}
		}

		// Step 5-6: Compute entry and target TTB with position-specific chains (ALG-REQ-046 v1.3)
		// These are ephemeral — NOT written to the database.
		entryCurrentTTB := 10
		if ttb, ok := freshTTBs[fromID]; ok {
			entryCurrentTTB = ttb
		}
		entryTTB := nebula.ComputeTTBStub(entryCurrentTTB, "CHAIN_ENTRANCE")

		targetCurrentTTB := 10
		if ttb, ok := freshTTBs[toID]; ok {
			targetCurrentTTB = ttb
		}
		targetTTB := nebula.ComputeTTBStub(targetCurrentTTB, "CHAIN_TARGET")

		log.Printf("[%s] api: position-aware TTB — entry %s=%d, target %s=%d",
			time.Now().Format("15:04:05.000"), fromID, entryTTB, toID, targetTTB)

		// Step 7: Compute TTA per path (ALG-REQ-010 v1.3)
		pathItems := make([]graph.PathItem, 0, len(pathResults))
		for i, p := range pathResults {
			hosts := strings.Join(p.IDs, " -> ")
			tta := 0
			for j, id := range p.IDs {
				switch {
				case j == 0:
					tta += entryTTB
				case j == len(p.IDs)-1:
					tta += targetTTB
				default:
					if ttb, ok := freshTTBs[id]; ok {
						tta += ttb
					} else if j < len(p.TTBs) {
						tta += p.TTBs[j]
					} else {
						tta += 10
					}
				}
			}
			pathItems = append(pathItems, graph.PathItem{
				PathID: fmt.Sprintf("P%05d", i+1),
				Hosts:  hosts,
				TTA:    tta,
			})
		}

		// Sort by TTA ascending (ALG-REQ-001: response ordered by TTA)
		sort.Slice(pathItems, func(i, j int) bool {
			return pathItems[i].TTA < pathItems[j].TTA
		})

		// Re-assign path IDs after sorting
		for i := range pathItems {
			pathItems[i].PathID = fmt.Sprintf("P%05d", i+1)
		}

		// Step 8: Build response (ALG-REQ-046 step 8)
		if recalculatedAssets == nil {
			recalculatedAssets = []string{}
		}

		response := graph.PathsResponseWithRecalc{
			Paths:              pathItems,
			EntryPoint:         fromID,
			Target:             toID,
			Hops:               maxHops,
			Total:              len(pathItems),
			RecalculatedAssets: recalculatedAssets,
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d paths for %s -> %s in %.3f seconds (recalculated: %d)",
			time.Now().Format("15:04:05.000"), len(pathItems), fromID, toID,
			requestDuration.Seconds(), len(recalculatedAssets))
	}
}

// ============================================================
// Mitigations API handlers (REQ-033 through REQ-036)
// ============================================================

// MitigationsListHandler returns all MITRE mitigations for the editor dropdown (REQ-033).
func MitigationsListHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: /api/mitigations request", requestStart.Format("15:04:05.000"))

		mitigations, err := nebula.QueryMitigations(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QueryMitigations failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query mitigations", http.StatusInternalServerError)
			return
		}

		response := graph.BuildMitigationsList(mitigations)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d mitigations in %.3f seconds", time.Now().Format("15:04:05.000"), len(mitigations), requestDuration.Seconds())
	}
}

// handleGetAssetMitigations returns mitigations applied to an asset (REQ-034).
func handleGetAssetMitigations(pool *nebulago.ConnectionPool, cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	// URL: /api/asset/{id}/mitigations — asset ID is segment 3
	assetID, err := extractAssetID(r.URL.Path, 3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[%s] api: GET /api/asset/%s/mitigations request", requestStart.Format("15:04:05.000"), assetID)

	mitigations, err := nebula.QueryAssetMitigations(pool, cfg, assetID)
	if err != nil {
		log.Printf("[%s] api: QueryAssetMitigations failed: %v", time.Now().Format("15:04:05.000"), err)
		http.Error(w, "Failed to query asset mitigations", http.StatusInternalServerError)
		return
	}

	response := graph.BuildAssetMitigationsResponse(assetID, mitigations)

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
	}

	requestDuration := time.Since(requestStart)
	log.Printf("[%s] api: returned %d mitigations for asset %s in %.3f seconds",
		time.Now().Format("15:04:05.000"), len(mitigations), assetID, requestDuration.Seconds())
}

// MitigationUpsertRequest is the JSON body for PUT /api/asset/{id}/mitigations (REQ-035).
type MitigationUpsertRequest struct {
	MitigationID string `json:"mitigation_id"`
	Maturity     int    `json:"maturity"`
	Active       bool   `json:"active"`
}

// handleUpsertAssetMitigation adds or updates an applied_to edge (REQ-035).
func handleUpsertAssetMitigation(pool *nebulago.ConnectionPool, cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	assetID, err := extractAssetID(r.URL.Path, 3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var req MitigationUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	// REQ-038: validate mitigation ID format
	if !validMitigationID.MatchString(req.MitigationID) {
		http.Error(w, fmt.Sprintf("Invalid mitigation ID format: %q (expected pattern like M1020)", req.MitigationID), http.StatusBadRequest)
		return
	}

	// REQ-039: validate maturity is in the fixed set {25, 50, 80, 100}
	if !validMaturity[req.Maturity] {
		http.Error(w, fmt.Sprintf("Invalid maturity value: %d (allowed: 25, 50, 80, 100)", req.Maturity), http.StatusBadRequest)
		return
	}

	log.Printf("[%s] api: PUT /api/asset/%s/mitigations {%s, maturity=%d, active=%v}",
		requestStart.Format("15:04:05.000"), assetID, req.MitigationID, req.Maturity, req.Active)

	err = nebula.UpsertMitigation(pool, cfg, req.MitigationID, assetID, req.Maturity, req.Active)
	if err != nil {
		log.Printf("[%s] api: UpsertMitigation failed: %v", time.Now().Format("15:04:05.000"), err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// REQ-042: invalidate asset hash after mitigation change (ALG-REQ-043)
	nebula.InvalidateAssetHash(pool, cfg, assetID)

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	requestDuration := time.Since(requestStart)
	log.Printf("[%s] api: UPSERT %s -> %s completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), req.MitigationID, assetID, requestDuration.Seconds())
}

// handleDeleteAssetMitigation removes an applied_to edge (REQ-036).
func handleDeleteAssetMitigation(pool *nebulago.ConnectionPool, cfg *config.Config, w http.ResponseWriter, r *http.Request) {
	requestStart := time.Now()

	// URL: /api/asset/{id}/mitigations/{mid}
	assetID, err := extractAssetID(r.URL.Path, 3)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	mitigationID, err := extractMitigationID(r.URL.Path, 5)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	log.Printf("[%s] api: DELETE /api/asset/%s/mitigations/%s request",
		requestStart.Format("15:04:05.000"), assetID, mitigationID)

	err = nebula.DeleteMitigation(pool, cfg, mitigationID, assetID)
	if err != nil {
		log.Printf("[%s] api: DeleteMitigation failed: %v", time.Now().Format("15:04:05.000"), err)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	// REQ-042: invalidate asset hash after mitigation removal (ALG-REQ-043)
	nebula.InvalidateAssetHash(pool, cfg, assetID)

	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	requestDuration := time.Since(requestStart)
	log.Printf("[%s] api: DELETE %s -> %s completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), mitigationID, assetID, requestDuration.Seconds())
}

// ============================================================
// URL path helpers
// ============================================================

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

// extractMitigationID pulls the mitigation ID from the given URL path segment,
// validates it against the expected format (REQ-038), and returns it or a
// descriptive error for an HTTP 400 response.
func extractMitigationID(urlPath string, segmentIndex int) (string, error) {
	parts := strings.Split(urlPath, "/")
	if len(parts) <= segmentIndex {
		return "", fmt.Errorf("missing mitigation ID in path")
	}

	mitigationID := parts[segmentIndex]
	if mitigationID == "" {
		return "", fmt.Errorf("mitigation ID cannot be empty")
	}

	if !validMitigationID.MatchString(mitigationID) {
		return "", fmt.Errorf("invalid mitigation ID format: %q (expected pattern like M1020)", mitigationID)
	}

	return mitigationID, nil
}

// ============================================================
// Hash and TTB recalculation handlers (REQ-040, REQ-041)
// ============================================================

// RecalculateTTBHandler triggers bulk TTB recalculation for stale assets (REQ-040, ALG-REQ-045).
func RecalculateTTBHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		requestStart := time.Now()
		log.Printf("[%s] api: POST /api/recalculate-ttb request", requestStart.Format("15:04:05.000"))

		staleAssets, err := nebula.QueryStaleHashes(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QueryStaleHashes failed: %v", time.Now().Format("15:04:05.000"), err)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(map[string]string{"error": "Failed to compute hashes"})
			return
		}

		recalculated := 0
		for _, asset := range staleAssets {
			hashStr := fmt.Sprintf("%d", asset.ComputedHash)
			if hashStr == asset.StoredHash {
				if err := nebula.UpdateAssetTTBAndHash(pool, cfg, asset.AssetID, asset.CurrentTTB, hashStr); err != nil {
					log.Printf("[%s] api: UpdateAssetTTBAndHash (unchanged) failed for %s: %v",
						time.Now().Format("15:04:05.000"), asset.AssetID, err)
				}
				continue
			}

			newTTB := asset.CurrentTTB + rand.Intn(10) + 1
			if err := nebula.UpdateAssetTTBAndHash(pool, cfg, asset.AssetID, newTTB, hashStr); err != nil {
				log.Printf("[%s] api: UpdateAssetTTBAndHash failed for %s: %v",
					time.Now().Format("15:04:05.000"), asset.AssetID, err)
				continue
			}
			recalculated++
			log.Printf("[%s] api: recalculated TTB for %s: %d -> %d",
				time.Now().Format("15:04:05.000"), asset.AssetID, asset.CurrentTTB, newTTB)
		}

		merkleRoot, totalAssets, err := nebula.ComputeMerkleRoot(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: ComputeMerkleRoot failed: %v", time.Now().Format("15:04:05.000"), err)
		}
		if err := nebula.UpdateSystemState(pool, cfg, merkleRoot, totalAssets); err != nil {
			log.Printf("[%s] api: UpdateSystemState failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		response := graph.RecalculateResponse{
			Recalculated: recalculated,
			Unchanged:    len(staleAssets) - recalculated,
			Total:        totalAssets,
			MerkleRoot:   fmt.Sprintf("%d", merkleRoot),
		}

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: recalculated %d/%d assets in %.3f seconds",
			time.Now().Format("15:04:05.000"), recalculated, len(staleAssets), requestDuration.Seconds())
	}
}

// SystemStateHandler returns the current SystemState (REQ-041, ALG-REQ-048).
func SystemStateHandler(pool *nebulago.ConnectionPool, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()
		log.Printf("[%s] api: GET /api/system-state request", requestStart.Format("15:04:05.000"))

		data, err := nebula.QuerySystemState(pool, cfg)
		if err != nil {
			log.Printf("[%s] api: QuerySystemState failed: %v", time.Now().Format("15:04:05.000"), err)
			http.Error(w, "Failed to query system state", http.StatusInternalServerError)
			return
		}

		response := graph.BuildSystemStateResponse(data)

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned system state in %.3f seconds",
			time.Now().Format("15:04:05.000"), requestDuration.Seconds())
	}
}
