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
// Hash and TTB recalculation handlers (REQ-040, REQ-041)
// ============================================================

// RecalculateTTBHandler triggers bulk TTB recalculation for stale assets
// (REQ-040, ALG-REQ-045, ALG-REQ-070).
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

		ttbParams := nebula.TTBParams{
			OrientationTime:   cfg.OrientationTime,
			SwitchoverTime:    cfg.SwitchoverTime,
			PriorityTolerance: cfg.PriorityTolerance,
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

			// ALG-REQ-070: real TTB computation replaces stub
			chainVID := nebula.ChainVIDForPosition(1, 3) // default intermediate
			ttbResult, err := nebula.ComputeTTB(pool, cfg, asset.AssetID, chainVID, ttbParams, nil)
			if err != nil {
				log.Printf("[%s] api: ComputeTTB failed for %s: %v",
					time.Now().Format("15:04:05.000"), asset.AssetID, err)
				continue
			}

			if err := nebula.UpdateAssetTTBAndHash(pool, cfg, asset.AssetID, ttbResult.TTB, hashStr); err != nil {
				log.Printf("[%s] api: UpdateAssetTTBAndHash failed for %s: %v",
					time.Now().Format("15:04:05.000"), asset.AssetID, err)
				continue
			}
			recalculated++
			log.Printf("[%s] api: recalculated TTB for %s: %.4f -> %.4f (%d log entries)",
				time.Now().Format("15:04:05.000"), asset.AssetID, asset.CurrentTTB, ttbResult.TTB, len(ttbResult.Log))
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
