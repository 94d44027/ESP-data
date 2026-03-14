package api

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"ESP-data/config"
	"ESP-data/internal/graph"
	"ESP-data/internal/nebula"
	"ESP-data/internal/store"

	nebulago "github.com/vesoft-inc/nebula-go/v3"
)

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

// TargetsHandler returns targets for Path Inspector dropdown (ALG-REQ-003, migrated from REQ-031).
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
// (ALG-REQ-001, ALG-REQ-010, ALG-REQ-046, ALG-REQ-070..080 v1.5).
func PathsHandler(pool *nebulago.ConnectionPool, cfg *config.Config, auditStore *store.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		requestStart := time.Now()

		// ADR-REQ-030: create per-request audit buffer (nil if store disabled)
		var auditBuf *store.AuditBuffer
		if auditStore.Enabled() {
			auditBuf = &store.AuditBuffer{}
		}

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

		// A5: Parse optional TTB calculation parameters (ALG-REQ-071, 072, 075; UI-REQ-2091)
		orientationTime := cfg.OrientationTime
		if v := r.URL.Query().Get("orientationTime"); v != "" {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed >= 0 {
				orientationTime = parsed
			}
		}
		switchoverTime := cfg.SwitchoverTime
		if v := r.URL.Query().Get("switchoverTime"); v != "" {
			if parsed, err := strconv.ParseFloat(v, 64); err == nil && parsed >= 0 {
				switchoverTime = parsed
			}
		}
		priorityTolerance := cfg.PriorityTolerance
		if v := r.URL.Query().Get("priorityTolerance"); v != "" {
			if parsed, err := strconv.Atoi(v); err == nil && parsed >= 0 {
				priorityTolerance = parsed
			}
		}

		log.Printf("[%s] api: /api/paths?from=%s&to=%s&hops=%d (orient=%.4f switch=%.4f priTol=%d)",
			requestStart.Format("15:04:05.000"), fromID, toID, maxHops,
			orientationTime, switchoverTime, priorityTolerance)

		// Build TTBParams once — used by all ComputeTTB calls in this handler
		ttbParams := nebula.TTBParams{
			OrientationTime:   orientationTime,
			SwitchoverTime:    switchoverTime,
			PriorityTolerance: priorityTolerance,
		}

		// Timing buckets for /api/paths phase observability.
		var queryPathsDuration time.Duration
		var ttbRecalcDuration time.Duration
		var ttbEntryDuration time.Duration
		var ttbTargetDuration time.Duration
		var jsonEncodeDuration time.Duration

		// Step 1: Find paths — returns per-node IDs and stored TTBs (ALG-REQ-001 v1.3)
		qpStart := time.Now()
		pathResults, err := nebula.QueryPaths(pool, cfg, fromID, toID, maxHops)
		queryPathsDuration = time.Since(qpStart)
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
		freshTTBs := make(map[string]float64)

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

					ttbRecalcStart := time.Now()
					staleHashes, err := nebula.QueryScopedStaleHashes(pool, cfg, staleIDs)
					if err != nil {
						log.Printf("[%s] api: QueryScopedStaleHashes failed: %v",
							time.Now().Format("15:04:05.000"), err)
					} else {
						for _, asset := range staleHashes {
							hashStr := fmt.Sprintf("%d", asset.ComputedHash)
							chainVID := nebula.ChainVIDForPosition(1, 3) // intermediate position
							ttbResult, err := nebula.ComputeTTB(pool, cfg, asset.AssetID, chainVID, ttbParams, auditBuf)
							if auditBuf != nil && len(auditBuf.Breakdowns) > 0 {
								auditBuf.Breakdowns[len(auditBuf.Breakdowns)-1].ChainPosition = "intermediate"
							}
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
							freshTTBs[asset.AssetID] = ttbResult.TTB
							recalculatedAssets = append(recalculatedAssets, asset.AssetID)
							log.Printf("[%s] api: path-scoped recalc %s: TTB %.4f -> %.4f",
								time.Now().Format("15:04:05.000"), asset.AssetID, asset.CurrentTTB, ttbResult.TTB)
						}
						ttbRecalcDuration = time.Since(ttbRecalcStart)
					}
					// Decrement stale_count to reflect path-scoped recalculations (UI-REQ-112A)
					if len(recalculatedAssets) > 0 {
						nebula.DecrementStaleCount(pool, cfg, len(recalculatedAssets))
					}
				}
			}
		}

		// Step 5-6: Compute entry and target TTB with position-specific chains (ALG-REQ-046, ALG-REQ-070)
		// These are ephemeral — NOT written to the database.
		var allTTBLog []nebula.TTBLogEntry

		// Determine path length for chain selection (ALG-REQ-051)
		// Use the first path's length as representative; all paths share the same entry/target.
		pathLen := 2 // minimum: entry + target
		if len(pathResults) > 0 && len(pathResults[0].IDs) > pathLen {
			pathLen = len(pathResults[0].IDs)
		}

		entryChainVID := nebula.ChainVIDForPosition(0, pathLen) // entry position
		entryStart := time.Now()
		entryResult, err := nebula.ComputeTTB(pool, cfg, fromID, entryChainVID, ttbParams, auditBuf)
		if auditBuf != nil && len(auditBuf.Breakdowns) > 0 {
			auditBuf.Breakdowns[len(auditBuf.Breakdowns)-1].ChainPosition = "entrance"
		}
		ttbEntryDuration = time.Since(entryStart)
		var entryTTB float64
		if err != nil {
			log.Printf("[%s] api: ComputeTTB (entry %s) failed: %v, using fallback",
				time.Now().Format("15:04:05.000"), fromID, err)
			if ttb, ok := freshTTBs[fromID]; ok {
				entryTTB = ttb
			} else {
				entryTTB = 10.0
			}
		} else {
			entryTTB = entryResult.TTB
			allTTBLog = append(allTTBLog, entryResult.Log...)
		}

		targetChainVID := nebula.ChainVIDForPosition(pathLen-1, pathLen) // target position
		targetStart := time.Now()
		targetResult, err := nebula.ComputeTTB(pool, cfg, toID, targetChainVID, ttbParams, auditBuf)
		if auditBuf != nil && len(auditBuf.Breakdowns) > 0 {
			auditBuf.Breakdowns[len(auditBuf.Breakdowns)-1].ChainPosition = "target"
		}
		ttbTargetDuration = time.Since(targetStart)
		var targetTTB float64
		if err != nil {
			log.Printf("[%s] api: ComputeTTB (target %s) failed: %v, using fallback",
				time.Now().Format("15:04:05.000"), toID, err)
			if ttb, ok := freshTTBs[toID]; ok {
				targetTTB = ttb
			} else {
				targetTTB = 10.0
			}
		} else {
			targetTTB = targetResult.TTB
			allTTBLog = append(allTTBLog, targetResult.Log...)
		}

		log.Printf("[%s] api: position-aware TTB — entry %s=%.4f, target %s=%.4f",
			time.Now().Format("15:04:05.000"), fromID, entryTTB, toID, targetTTB)

		// Step 7: Compute TTA per path (ALG-REQ-010, ALG-REQ-078)
		pathItems := make([]graph.PathItem, 0, len(pathResults))
		for i, p := range pathResults {
			hosts := strings.Join(p.IDs, " -> ")
			var tta float64
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
						tta += 10.0
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

		// Step 8: Build response (ALG-REQ-046 step 8, ALG-REQ-079)
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
			TTBLog:             allTTBLog,
		}

		w.Header().Set("Content-Type", "application/json")
		jsonStart := time.Now()
		if err := json.NewEncoder(w).Encode(response); err != nil {
			log.Printf("[%s] api: JSON encode failed: %v", time.Now().Format("15:04:05.000"), err)
		}
		jsonEncodeDuration = time.Since(jsonStart)

		// ADR-REQ-031: populate session record and flush audit buffer async after response is sent
		if auditBuf != nil {
			totalMs := int(time.Since(requestStart).Milliseconds())
			auditBuf.Session = store.SessionRecord{
				EntryAssetID:       fromID,
				TargetAssetID:      toID,
				MaxHops:            maxHops,
				OrientationTime:    orientationTime,
				SwitchoverTime:     switchoverTime,
				PriorityTolerance:  priorityTolerance,
				PathsFound:         len(pathItems),
				AssetsRecalculated: len(recalculatedAssets),
				QueryTimeMs:        int(queryPathsDuration.Milliseconds()),
				TotalTimeMs:        totalMs,
			}
			for idx, p := range pathItems {
				hopCount := len(strings.Split(p.Hosts, " -> "))
				auditBuf.Paths = append(auditBuf.Paths, store.PathRecord{
					PathSeq:   idx + 1,
					HostChain: p.Hosts,
					HopCount:  hopCount,
					TTAHours:  p.TTA,
				})
			}
			go auditStore.FlushBatch(auditBuf)
		}

		requestDuration := time.Since(requestStart)
		log.Printf("[%s] api: returned %d paths for %s -> %s in %.3f seconds (recalculated: %d, qp=%.3f, recalc=%.3f, ttbEntry=%.3f, ttbTarget=%.3f, json=%.3f)",
			time.Now().Format("15:04:05.000"), len(pathItems), fromID, toID,
			requestDuration.Seconds(), len(recalculatedAssets),
			queryPathsDuration.Seconds(), ttbRecalcDuration.Seconds(),
			ttbEntryDuration.Seconds(), ttbTargetDuration.Seconds(), jsonEncodeDuration.Seconds())
	}
}
