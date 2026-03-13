package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"ESP-data/config"
	"ESP-data/internal/graph"
	"ESP-data/internal/nebula"

	nebulago "github.com/vesoft-inc/nebula-go/v3"
)

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

	// REQ-042: invalidate asset hash after mitigation change (ALG-REQ-043)
	nebula.InvalidateAssetHash(pool, cfg, assetID)

	w.Header().Set("Content-Type", "application/json")
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

	// REQ-042: invalidate asset hash after mitigation removal (ALG-REQ-043)
	nebula.InvalidateAssetHash(pool, cfg, assetID)

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "ok"})

	requestDuration := time.Since(requestStart)
	log.Printf("[%s] api: DELETE %s -> %s completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), mitigationID, assetID, requestDuration.Seconds())
}
