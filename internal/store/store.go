package store

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// Store wraps the MariaDB connection pool and provides audit/cache operations.
// All write operations are designed for async (fire-and-forget) use (ADR-REQ-031).
type Store struct {
	db      *sql.DB
	enabled bool
}

// New creates a Store and runs schema migrations (ADR-REQ-003, ADR-REQ-081).
// If the connection fails and MARIA_ENABLED is true, returns an error.
// The caller may choose to proceed without the store (graceful degradation, ADR-REQ-033).
func New(host string, port int, user, pass, dbname string) (*Store, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true&charset=utf8mb4",
		user, pass, host, port, dbname)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("store: failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("store: failed to ping MariaDB at %s:%d: %w", host, port, err)
	}

	log.Printf("store: connected to MariaDB %s:%d/%s", host, port, dbname)

	if err := RunMigrations(db); err != nil {
		db.Close()
		return nil, err
	}

	return &Store{db: db, enabled: true}, nil
}

// Enabled returns whether the store is available for use.
func (s *Store) Enabled() bool {
	return s != nil && s.enabled
}

// Close shuts down the connection pool.
func (s *Store) Close() {
	if s != nil && s.db != nil {
		s.db.Close()
		log.Printf("store: connection closed")
	}
}

// FlushBatch writes the entire audit buffer to MariaDB in a single transaction (ADR-REQ-031).
// Designed to be called as: go store.FlushBatch(buf)
// If any step fails, the transaction is rolled back and the error is logged.
// No retry — the data is rebuildable (ADR-REQ-033).
func (s *Store) FlushBatch(buf *AuditBuffer) {
	if !s.Enabled() || buf == nil {
		return
	}

	flushStart := time.Now()

	tx, err := s.db.Begin()
	if err != nil {
		log.Printf("store: FlushBatch failed to begin tx: %v", err)
		return
	}

	defer func() {
		if err != nil {
			tx.Rollback()
			log.Printf("store: FlushBatch rolled back after %.3fs: %v",
				time.Since(flushStart).Seconds(), err)
		}
	}()

	// Layer 1: session
	res, err := tx.Exec(`INSERT INTO calc_sessions
		(entry_asset_id, target_asset_id, max_hops, orientation_time,
		 switchover_time, priority_tolerance, paths_found,
		 assets_recalculated, query_time_ms, total_time_ms)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		buf.Session.EntryAssetID, buf.Session.TargetAssetID,
		buf.Session.MaxHops, buf.Session.OrientationTime,
		buf.Session.SwitchoverTime, buf.Session.PriorityTolerance,
		buf.Session.PathsFound, buf.Session.AssetsRecalculated,
		buf.Session.QueryTimeMs, buf.Session.TotalTimeMs)
	if err != nil {
		return
	}
	sessionID, _ := res.LastInsertId()

	// Layer 2: paths (ADR-REQ-032 batch insert)
	for _, p := range buf.Paths {
		_, err = tx.Exec(`INSERT INTO calc_paths
			(session_id, path_seq, host_chain, hop_count, tta_hours)
			VALUES (?, ?, ?, ?, ?)`,
			sessionID, p.PathSeq, p.HostChain, p.HopCount, p.TTAHours)
		if err != nil {
			return
		}
	}

	// Layer 3: one breakdown per ComputeTTB call (one per asset per request) (ADR-REQ-012)
	breakdownIDs := make([]int64, len(buf.Breakdowns))
	for i, bd := range buf.Breakdowns {
		res, err = tx.Exec(`INSERT INTO calc_ttb_breakdown
			(session_id, asset_vid, chain_position, chain_vid,
			 ttb_total, orientation_time, tactic_count, technique_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			sessionID, bd.AssetVid, bd.ChainPosition, bd.ChainVid,
			bd.TTBTotal, bd.OrientationTime, bd.TacticCount, bd.TechniqueCount)
		if err != nil {
			return
		}
		breakdownIDs[i], _ = res.LastInsertId()
	}

	// Layer 3A: tactic steps — resolve BreakdownIdx → real breakdown_id (ADR-REQ-013)
	stepIDs := make([]int64, len(buf.TacticSteps))
	for j, ts := range buf.TacticSteps {
		var bdID int64
		if ts.BreakdownIdx >= 0 && ts.BreakdownIdx < len(breakdownIDs) {
			bdID = breakdownIDs[ts.BreakdownIdx]
		}
		res, err = tx.Exec(`INSERT INTO calc_ttb_tactic_steps
			(breakdown_id, tactic_seq, tactic_id, tactic_name,
			 technique_id, technique_name,
			 ttt_hours, switchover_added, candidates_count)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			bdID, ts.TacticSeq, ts.TacticID, ts.TacticName,
			sql.NullString{String: ts.TechniqueID, Valid: ts.TechniqueID != ""},
			sql.NullString{String: ts.TechniqueName, Valid: ts.TechniqueName != ""},
			ts.TTTHours, ts.SwitchoverAdded, ts.CandidatesCount)
		if err != nil {
			return
		}
		stepIDs[j], _ = res.LastInsertId()
	}

	// Layer 4: TTT detail — resolve StepIdx → real step_id (ADR-REQ-014)
	for _, td := range buf.TTTDetails {
		var sID int64
		if td.StepIdx >= 0 && td.StepIdx < len(stepIDs) {
			sID = stepIDs[td.StepIdx]
		}
		_, err = tx.Exec(`INSERT INTO calc_ttt_detail
			(step_id, technique_id, exec_min, exec_max,
			 possible_count, applied_count, maturity_factor,
			 formula_case, ttt_hours)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
			sID, td.TechniqueID,
			td.ExecMin, td.ExecMax,
			td.PossibleCount, td.AppliedCount, td.MaturityFactor,
			td.FormulaCase, td.TTTHours)
		if err != nil {
			return
		}
	}

	// Cache entries (ADR-REQ-022 — UPSERT via REPLACE)
	for _, ce := range buf.CacheEntries {
		_, err = tx.Exec(`REPLACE INTO asset_ttb_cache
			(asset_vid, chain_position, computed_at, nebula_hash,
			 ttb_total, orientation_time, breakdown_json, is_valid)
			VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
			ce.AssetVid, ce.ChainPosition, ce.ComputedAt, ce.NebulaHash,
			ce.TTBTotal, ce.OrientationTime, ce.BreakdownJSON, ce.IsValid)
		if err != nil {
			return
		}
	}

	err = tx.Commit()
	if err != nil {
		return
	}

	log.Printf("store: FlushBatch completed in %.3fs — session=%d paths=%d breakdowns=%d steps=%d details=%d cache=%d",
		time.Since(flushStart).Seconds(), sessionID,
		len(buf.Paths), len(buf.Breakdowns), len(buf.TacticSteps),
		len(buf.TTTDetails), len(buf.CacheEntries))
}

// InvalidateCache marks cached TTB breakdowns as stale for an asset (ADR-REQ-021).
// Called alongside InvalidateAssetHash when mitigations change.
func (s *Store) InvalidateCache(assetVid string) {
	if !s.Enabled() {
		return
	}
	_, err := s.db.Exec(
		`UPDATE asset_ttb_cache SET is_valid = FALSE WHERE asset_vid = ?`,
		assetVid)
	if err != nil {
		log.Printf("store: InvalidateCache failed for %s: %v", assetVid, err)
	}
}
