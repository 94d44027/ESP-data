package main

import (
	"log"
	"net/http"

	"ESP-data/api"
	"ESP-data/config"
	"ESP-data/internal/nebula"
	"ESP-data/internal/store"
)

func main() {
	// Load configuration from environment variables (REQ-002, ADR-REQ-002)
	cfg := config.Load()

	// Initialize Nebula connection pool (REQ-121)
	pool := nebula.NewPool(cfg)
	defer pool.Close()

	// Initialize MariaDB store (ADR-REQ-003, ADR-REQ-081)
	// Graceful degradation: if disabled or connection fails, auditStore is nil (ADR-REQ-033)
	var auditStore *store.Store
	if cfg.MariaEnabled {
		var err error
		auditStore, err = store.New(cfg.MariaHost, cfg.MariaPort, cfg.MariaUser, cfg.MariaPass, cfg.MariaDB)
		if err != nil {
			log.Printf("WARNING: MariaDB store unavailable — audit/cache disabled: %v", err)
			auditStore = nil
		} else {
			defer auditStore.Close()
		}
	} else {
		log.Printf("store: MariaDB disabled (MARIA_ENABLED=false)")
	}

	// Log store status for operator awareness
	if auditStore != nil && auditStore.Enabled() {
		log.Printf("store: audit trail and TTB cache active")
	} else {
		log.Printf("store: running without RDBMS — no audit trail or TTB cache")
	}

	// ---- suppress unused import until handlers are wired ----
	_ = auditStore

	// Register API endpoints

	// REQ-020: Enriched graph data for Cytoscape visualization
	http.HandleFunc("/api/graph", api.GraphHandler(pool, cfg))

	// REQ-021: Asset list for sidebar entity browser
	http.HandleFunc("/api/assets", api.AssetsHandler(pool, cfg))

	// REQ-022: Single asset detail for inspector panel
	// REQ-034 (GET), REQ-035 (PUT), REQ-036 (DELETE): Asset mitigations CRUD
	// AssetHandler dispatches based on URL path depth and HTTP method
	http.HandleFunc("/api/asset/", api.AssetHandler(pool, cfg))

	// REQ-023: Neighbor list for inspector connections summary
	http.HandleFunc("/api/neighbors/", api.NeighborsHandler(pool, cfg))

	// REQ-024: Asset types for filter checkboxes
	http.HandleFunc("/api/asset-types", api.AssetTypesHandler(pool, cfg))

	// REQ-026: Edge connections for edge inspector panel
	http.HandleFunc("/api/edges/", api.EdgesHandler(pool, cfg))

	// REQ-029: Path calculation for Path Inspector
	http.HandleFunc("/api/paths", api.PathsHandler(pool, cfg))

	// REQ-030: Entry points for Path Inspector dropdown
	http.HandleFunc("/api/entry-points", api.EntryPointsHandler(pool, cfg))

	// REQ-031: Targets for Path Inspector dropdown
	http.HandleFunc("/api/targets", api.TargetsHandler(pool, cfg))

	// REQ-033: All MITRE mitigations for editor dropdown
	http.HandleFunc("/api/mitigations", api.MitigationsListHandler(pool, cfg))

	// REQ-040: Bulk TTB recalculation
	http.HandleFunc("/api/recalculate-ttb", api.RecalculateTTBHandler(pool, cfg))

	// REQ-041: SystemState for UI badge
	http.HandleFunc("/api/system-state", api.SystemStateHandler(pool, cfg))

	// Serve static files (HTML, CSS, JS) from /static directory
	// This serves the VIS layer (REQ-123, UI-Requirements.MD)
	http.Handle("/", http.FileServer(http.Dir("static")))

	// Start HTTP server (REQ-130)
	addr := ":8080"
	log.Printf("ESP PoC starting on %s", addr)
	log.Printf("Configured Nebula: %s:%d, Space: %s", cfg.NebulaHost, cfg.NebulaPort, cfg.Space)
	log.Printf("API endpoints available:")
	log.Printf("  GET /api/graph         - Graph nodes and edges (REQ-020)")
	log.Printf("  GET /api/assets        - Asset list (REQ-021)")
	log.Printf("  GET /api/asset/{id}    - Asset detail (REQ-022)")
	log.Printf("  GET /api/neighbors/{id} - Neighbor list (REQ-023)")
	log.Printf("  GET /api/asset-types   - Asset types (REQ-024)")
	log.Printf("  GET /api/edges/{src}/{dst} - Edge connections (REQ-026)")
	log.Printf("  GET /api/paths         - Path calculation (REQ-029)")
	log.Printf("  GET /api/entry-points  - Entry points (REQ-030)")
	log.Printf("  GET /api/targets       - Targets (REQ-031)")
	log.Printf("  GET /api/mitigations   - All mitigations (REQ-033)")
	log.Printf("  GET /api/asset/{id}/mitigations    - Asset mitigations (REQ-034)")
	log.Printf("  PUT /api/asset/{id}/mitigations    - Upsert mitigation (REQ-035)")
	log.Printf("  DELETE /api/asset/{id}/mitigations/{mid} - Delete mitigation (REQ-036)")
	log.Printf("  POST /api/recalculate-ttb              - Bulk TTB recalculation (REQ-040)")
	log.Printf("  GET /api/system-state                   - System state (REQ-041)")
	log.Printf("Static files served from ./static/")
	log.Fatal(http.ListenAndServe(addr, nil))
}
