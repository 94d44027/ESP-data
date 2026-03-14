package store

import (
	"database/sql"
	"fmt"
	"log"
)

// tables is the ordered list of CREATE TABLE statements (ADR-REQ-081).
// Order matters: parent tables first, child tables after (FK dependencies).
var tables = []struct {
	name string
	ddl  string
}{
	{
		name: "calc_sessions",
		ddl: `CREATE TABLE IF NOT EXISTS calc_sessions (
    session_id        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at        DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    entry_asset_id    VARCHAR(64)   NOT NULL,
    target_asset_id   VARCHAR(64)   NOT NULL,
    max_hops          INT           NOT NULL,
    orientation_time  DOUBLE        NOT NULL,
    switchover_time   DOUBLE        NOT NULL,
    priority_tolerance INT          NOT NULL,
    paths_found       INT           NOT NULL,
    assets_recalculated INT        NOT NULL DEFAULT 0,
    query_time_ms     INT           NOT NULL,
    total_time_ms     INT           NOT NULL,
    INDEX idx_created (created_at),
    INDEX idx_entry_target (entry_asset_id, target_asset_id)
) ENGINE=InnoDB`,
	},
	{
		name: "calc_paths",
		ddl: `CREATE TABLE IF NOT EXISTS calc_paths (
    path_id     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    session_id  BIGINT UNSIGNED NOT NULL,
    path_seq    INT            NOT NULL,
    host_chain  TEXT           NOT NULL,
    hop_count   INT            NOT NULL,
    tta_hours   DOUBLE         NOT NULL,
    FOREIGN KEY (session_id) REFERENCES calc_sessions(session_id) ON DELETE CASCADE,
    INDEX idx_session (session_id),
    INDEX idx_tta (tta_hours)
) ENGINE=InnoDB`,
	},
	{
		name: "calc_ttb_breakdown",
		ddl: `CREATE TABLE IF NOT EXISTS calc_ttb_breakdown (
    breakdown_id     BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    session_id       BIGINT UNSIGNED NOT NULL,
    asset_vid        VARCHAR(64)    NOT NULL,
    chain_position   ENUM('entrance','intermediate','target') NOT NULL,
    chain_vid        VARCHAR(64)    NOT NULL,
    ttb_total        DOUBLE         NOT NULL,
    orientation_time DOUBLE         NOT NULL,
    tactic_count     INT            NOT NULL,
    technique_count  INT            NOT NULL,
    FOREIGN KEY (session_id) REFERENCES calc_sessions(session_id) ON DELETE CASCADE,
    INDEX idx_session_asset (session_id, asset_vid),
    INDEX idx_asset (asset_vid)
) ENGINE=InnoDB`,
	},
	{
		name: "calc_ttb_tactic_steps",
		ddl: `CREATE TABLE IF NOT EXISTS calc_ttb_tactic_steps (
    step_id          BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    breakdown_id     BIGINT UNSIGNED NOT NULL,
    tactic_seq       INT            NOT NULL,
    tactic_id        VARCHAR(16)    NOT NULL,
    tactic_name      VARCHAR(128)   NOT NULL,
    technique_id     VARCHAR(16)    NULL,
    technique_name   VARCHAR(256)   NULL,
    ttt_hours        DOUBLE         NOT NULL,
    switchover_added BOOLEAN        NOT NULL,
    candidates_count INT            NOT NULL,
    FOREIGN KEY (breakdown_id) REFERENCES calc_ttb_breakdown(breakdown_id) ON DELETE CASCADE,
    INDEX idx_breakdown (breakdown_id)
) ENGINE=InnoDB`,
	},
	{
		name: "calc_ttt_detail",
		ddl: `CREATE TABLE IF NOT EXISTS calc_ttt_detail (
    detail_id        BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    step_id          BIGINT UNSIGNED NOT NULL,
    technique_id     VARCHAR(16)    NOT NULL,
    exec_min         DOUBLE         NOT NULL,
    exec_max         DOUBLE         NOT NULL,
    possible_count   INT            NOT NULL,
    applied_count    INT            NOT NULL,
    maturity_factor  DOUBLE         NOT NULL,
    formula_case     ENUM('no_mitigations','full_coverage','partial') NOT NULL,
    ttt_hours        DOUBLE         NOT NULL,
    FOREIGN KEY (step_id) REFERENCES calc_ttb_tactic_steps(step_id) ON DELETE CASCADE,
    INDEX idx_step (step_id)
) ENGINE=InnoDB`,
	},
	{
		name: "asset_ttb_cache",
		ddl: `CREATE TABLE IF NOT EXISTS asset_ttb_cache (
    asset_vid        VARCHAR(64)    NOT NULL,
    chain_position   ENUM('entrance','intermediate','target') NOT NULL,
    computed_at      DATETIME(3)    NOT NULL,
    nebula_hash      VARCHAR(64)    NOT NULL,
    ttb_total        DOUBLE         NOT NULL,
    orientation_time DOUBLE         NOT NULL,
    breakdown_json   JSON           NOT NULL,
    is_valid         BOOLEAN        NOT NULL DEFAULT TRUE,
    PRIMARY KEY (asset_vid, chain_position),
    INDEX idx_valid (is_valid)
) ENGINE=InnoDB`,
	},
}

// RunMigrations executes CREATE TABLE IF NOT EXISTS for all ADR tables (ADR-REQ-081).
// Idempotent — safe to call on every application startup.
func RunMigrations(db *sql.DB) error {
	for _, t := range tables {
		_, err := db.Exec(t.ddl)
		if err != nil {
			return fmt.Errorf("store: migration failed for %s: %w", t.name, err)
		}
		log.Printf("store: table %s — ready", t.name)
	}
	log.Printf("store: all %d tables migrated", len(tables))
	return nil
}
