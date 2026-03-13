# Auxiliary Database Requirements (ADR)
## ESP PoC — MariaDB Relational Store

**Version:** 0.1 (Draft)  
**Date:** March 12, 2026  
**Prepared by:** Konstantin Smirnov with the kind assistance of Perplexity AI  
**Project:** ESP PoC for Nebula Graph  
**Reference:** Derived from SRS (CMP004), ALGO (ALG-REQ-079), UIS  
**Document code:** ADR

---

## 1. Overview

### 1.1 Purpose

This document specifies the requirements for the Auxiliary Relational Database (RDBMS) component of the ESP PoC system. The RDBMS provides persistent storage for data that is **derived from** or **supplementary to** the primary graph database (NebulaGraph), but is not suitable for graph storage — specifically: calculation audit trails, cached computation results, application configuration, and session state.

### 1.2 Document Scope

This specification covers:
- MariaDB schema definition (tables, columns, relationships, indexes)
- TTB/TTT calculation audit trail storage (Layer 1–4 decomposition)
- Cached TTB breakdown data per asset (materialized computation results)
- Asynchronous write mechanism (write-behind pattern)
- Cache invalidation rules (tied to existing hash/stale mechanism in NebulaGraph)
- Configuration data storage (future)
- New and modified API endpoints

This specification does **not** cover:
- Graph data model — see ESP01_NebulaGraph_Schema.md (SCHEMA)
- TTB/TTT calculation algorithms — see AlgoSpecs.md (ALGO)
- UI presentation of audit data — see UI-Requirements.md (UIS), future amendments
- MariaDB installation and server administration

### 1.3 Relationship to Other Documents

| Document                             | Version | Relationship                                                                                                                                                 |
|--------------------------------------|---------|--------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Requirements.md (SRS)                | v1.14   | CMP004 defines the optional RDBMS component. REQ-040/041 define recalculation triggers that produce audit data. REQ-121 mandates Go for the APP layer.       |
| ESP01_NebulaGraph_Schema.md (SCHEMA) | v1.10   | TA001 (Asset.TTB, Asset.hash, Asset.hash_valid) drives cache invalidation. ED001 (applied_to) changes trigger stale marking.                                 |
| AlgoSpecs.md (ALGO)                  | v1.7    | ALG-REQ-060–066 (TTT formula), ALG-REQ-070–080 (TTB algorithm), ALG-REQ-079 (TTB log) define the data structures stored in RDBMS.                           |
| UI-Requirements.md (UIS)             | v1.14   | UI-REQ-207 (Path Inspector) will consume path detail data from RDBMS. Future UI amendments will add drill-down views.                                        |

### 1.4 Requirement ID Convention

All requirements in this document use the prefix `ADR-REQ-` followed by a three-digit number. Sections use `##` for chapters and `###` for individual requirements (as headers), matching the style of UI-Requirements.md and AlgoSpecs.md.

### 1.5 Design Principles

- **Non-blocking:** RDBMS writes SHALL NOT delay the HTTP response to the user. All writes are asynchronous (write-behind).
- **Rebuildable:** All data in the RDBMS is derived from NebulaGraph. If the RDBMS is lost or cleared, the system continues to function; data is rebuilt on next calculation.
- **Additive:** The RDBMS does not replace any NebulaGraph functionality. It adds capabilities (audit, cache, config) without modifying the existing graph data flow.
- **Graceful degradation:** If the RDBMS is unavailable, the application SHALL log a warning and continue operating without audit/cache functionality.

---

## 2. Definitions

| Term                    | Definition                                                                                                                                                             |
|-------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **RDBMS**               | The MariaDB relational database instance (SRS CMP004)                                                                                                                  |
| **Calculation Session** | A single invocation of `GET /api/paths` that produces a set of paths with TTA values. Each session has a unique ID and timestamp.                                       |
| **TTB Breakdown**       | The per-tactic decomposition of a TTB value: which technique was selected at each tactic step, what its TTT was, how many candidates existed (ALG-REQ-079)              |
| **TTT Detail**          | The per-technique decomposition of a TTT value: P, A, maturity_factor, exec_min, exec_max, and the formula case applied (ALG-REQ-060)                                  |
| **Audit Trail**         | The complete record of a calculation session: paths found, TTB values used, TTB breakdowns for recomputed assets, TTT details for each technique                        |
| **Cached Breakdown**    | The most recent TTB breakdown for an asset, stored persistently and served on demand without recomputation. Invalidated when the asset's NebulaGraph hash changes.       |
| **Write-Behind**        | A pattern where data is buffered in memory during computation and flushed to the RDBMS asynchronously after the HTTP response is sent                                   |

---

## 3. Infrastructure

### ADR-REQ-001: MariaDB Instance

The system SHALL use a MariaDB server (version 10.6 or later) as the RDBMS component (SRS CMP004). The database instance SHALL run on the same host as the application server (`nebble.m82`) or be network-accessible from it.

**Database name:** `esp_aux`  
**Character set:** `utf8mb4`  
**Collation:** `utf8mb4_unicode_ci`

```sql
CREATE DATABASE IF NOT EXISTS esp_aux
  CHARACTER SET utf8mb4
  COLLATE utf8mb4_unicode_ci;
```

### ADR-REQ-002: Connection Configuration

The APP layer SHALL read MariaDB connection parameters from environment variables, consistent with the existing configuration pattern (SRS REQ-002, `config/config.go`):

| Environment Variable | Default     | Description                        |
|----------------------|-------------|------------------------------------|
| `MARIA_HOST`         | `127.0.0.1` | MariaDB server hostname            |
| `MARIA_PORT`         | `3306`      | MariaDB server port                |
| `MARIA_USER`         | `esp`       | MariaDB user                       |
| `MARIA_PASS`         | `esp`       | MariaDB password                   |
| `MARIA_DB`           | `esp_aux`   | Database name                      |
| `MARIA_ENABLED`      | `true`      | Enable/disable RDBMS functionality |

When `MARIA_ENABLED` is `false` or the connection fails, the APP layer SHALL log a warning and disable all RDBMS-dependent functionality. The application SHALL continue to operate normally using NebulaGraph only (graceful degradation per §1.5).

### ADR-REQ-003: Go Driver and Connection Pool

The APP layer SHALL use the `go-sql-driver/mysql` package (compatible with MariaDB) for database connectivity. A connection pool SHALL be established at application startup and shared across all handlers.

```go
import "database/sql"
import _ "github.com/go-sql-driver/mysql"
```

The `sql.DB` pool handle SHALL be added to the application's dependency injection (currently passed as `pool *nebula.ConnectionPool` and `cfg *config.Config` to handlers).

---

## 4. Schema — Calculation Audit Trail

The audit trail is structured in four layers, each progressively more detailed. All layers are written together as part of a single calculation session's async flush.

### ADR-REQ-010: Layer 1 — Calculation Sessions

Each invocation of `GET /api/paths` creates one session record.

```sql
CREATE TABLE calc_sessions (
    session_id      BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    created_at      DATETIME(3)   NOT NULL DEFAULT CURRENT_TIMESTAMP(3),
    entry_asset_id  VARCHAR(64)   NOT NULL COMMENT 'Source asset VID (e.g. A00017)',
    target_asset_id VARCHAR(64)   NOT NULL COMMENT 'Target asset VID (e.g. A00011)',
    max_hops        INT           NOT NULL,
    orientation_time DOUBLE       NOT NULL COMMENT 'ALG-REQ-071 parameter used',
    switchover_time  DOUBLE       NOT NULL COMMENT 'ALG-REQ-072 parameter used',
    priority_tolerance INT        NOT NULL COMMENT 'ALG-REQ-075 parameter used',
    paths_found     INT           NOT NULL COMMENT 'Total paths returned',
    assets_recalculated INT       NOT NULL DEFAULT 0 COMMENT 'Stale assets recomputed (ALG-REQ-046)',
    query_time_ms   INT           NOT NULL COMMENT 'NebulaGraph path query duration',
    total_time_ms   INT           NOT NULL COMMENT 'Total API response time',

    INDEX idx_created (created_at),
    INDEX idx_entry_target (entry_asset_id, target_asset_id)
) ENGINE=InnoDB;
```

**Rationale:** Captures the "when, what, how" of each calculation run. The parameter values are recorded because they can be changed between runs (UI-REQ-2091), and the results are only meaningful in context of the parameters used.

### ADR-REQ-011: Layer 2 — Path Results

Each path within a session gets one record. This links to the session and records the TTA composition.

```sql
CREATE TABLE calc_paths (
    path_id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    session_id      BIGINT UNSIGNED NOT NULL,
    path_seq        INT            NOT NULL COMMENT 'P00001 = 1, P00002 = 2, etc.',
    host_chain      TEXT           NOT NULL COMMENT 'A00017 -> A00012 -> A00011',
    hop_count       INT            NOT NULL,
    tta_hours       DOUBLE         NOT NULL COMMENT 'Total TTA in hours',

    FOREIGN KEY (session_id) REFERENCES calc_sessions(session_id) ON DELETE CASCADE,
    INDEX idx_session (session_id),
    INDEX idx_tta (tta_hours)
) ENGINE=InnoDB;
```

>Design note: We store all paths or top-N paths per session. For PoC, storing top 50 is sufficient; the full set (up to ~5000) may be excessive. This is configurable via ADR-REQ-040.

### ADR-REQ-012: Layer 3 — TTB Breakdown per Asset

For each asset whose TTB was computed (on-the-fly entry/target or recalculated stale intermediate), record the per-tactic breakdown. This is the persistent form of ALG-REQ-079's TTBLogEntry.

```sql
CREATE TABLE calc_ttb_breakdown (
    breakdown_id    BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    session_id      BIGINT UNSIGNED NOT NULL,
    asset_vid       VARCHAR(64)    NOT NULL COMMENT 'Asset VID',
    chain_position  ENUM('entrance','intermediate','target') NOT NULL,
    chain_vid       VARCHAR(64)    NOT NULL COMMENT 'CHAIN_ENTRANCE / CHAIN_INTERMEDIATE / CHAIN_TARGET',
    ttb_total       DOUBLE         NOT NULL COMMENT 'Computed TTB for this asset+chain',
    orientation_time DOUBLE        NOT NULL,
    tactic_count    INT            NOT NULL COMMENT 'Number of tactics in chain',
    technique_count INT            NOT NULL COMMENT 'Tactics with a selected technique (non-empty)',

    FOREIGN KEY (session_id) REFERENCES calc_sessions(session_id) ON DELETE CASCADE,
    INDEX idx_session_asset (session_id, asset_vid),
    INDEX idx_asset (asset_vid)
) ENGINE=InnoDB;
```

### ADR-REQ-013: Layer 3A — TTB Tactic Steps

Each tactic step within a TTB breakdown. Directly maps to ALG-REQ-079 TTBLogEntry.

```sql
CREATE TABLE calc_ttb_tactic_steps (
    step_id         BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    breakdown_id    BIGINT UNSIGNED NOT NULL,
    tactic_seq      INT            NOT NULL COMMENT 'Order in chain (0-based)',
    tactic_id       VARCHAR(16)    NOT NULL COMMENT 'e.g. TA0002',
    tactic_name     VARCHAR(128)   NOT NULL,
    technique_vid   VARCHAR(64)    NULL     COMMENT 'Selected technique VID, NULL if empty set (ALG-REQ-080)',
    technique_id    VARCHAR(16)    NULL     COMMENT 'e.g. T1071.001',
    technique_name  VARCHAR(256)   NULL,
    ttt_hours       DOUBLE         NOT NULL COMMENT 'TTT of selected technique, 0.0 if empty set',
    switchover_added BOOLEAN       NOT NULL COMMENT 'Whether switchover time was added before this step',
    candidates_count INT           NOT NULL COMMENT 'Candidates after all filtering',

    FOREIGN KEY (breakdown_id) REFERENCES calc_ttb_breakdown(breakdown_id) ON DELETE CASCADE,
    INDEX idx_breakdown (breakdown_id)
) ENGINE=InnoDB;
```

### ADR-REQ-014: Layer 4 — TTT Detail

For each selected technique, record the full ALG-REQ-060 inputs and result. This is the deepest audit level — explains *why* a particular TTT value was computed.

```sql
CREATE TABLE calc_ttt_detail (
    detail_id       BIGINT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    step_id         BIGINT UNSIGNED NOT NULL,
    technique_vid   VARCHAR(64)    NOT NULL,
    exec_min        DOUBLE         NOT NULL COMMENT 'TA008 execution_min',
    exec_max        DOUBLE         NOT NULL COMMENT 'TA008 execution_max',
    possible_count  INT            NOT NULL COMMENT 'P — mitigations that can mitigate this technique',
    applied_count   INT            NOT NULL COMMENT 'A — active-applied mitigations on this asset',
    maturity_factor DOUBLE         NOT NULL COMMENT 'Σ(0.01 × M_i) for active-applied',
    formula_case    ENUM('no_mitigations','full_coverage','partial') NOT NULL COMMENT 'ALG-REQ-060 case applied',
    ttt_hours       DOUBLE         NOT NULL COMMENT 'Computed TTT',

    FOREIGN KEY (step_id) REFERENCES calc_ttb_tactic_steps(step_id) ON DELETE CASCADE,
    INDEX idx_step (step_id)
) ENGINE=InnoDB;
```

>Design note: `formula_case` explicitly records which branch of the ALG-REQ-060 formula was applied. This aids debugging — you can immediately see whether a technique hit the `exec_max` ceiling (Case 2) or was interpolated (Case 3).

---

## 5. Schema — Cached TTB Breakdowns

### ADR-REQ-020: Asset TTB Cache Table

The RDBMS SHALL maintain the most recent TTB breakdown for every asset that has been computed. This serves as a persistent cache — when a user requests path detail, the breakdown is served from here without recomputing against NebulaGraph.

```sql
CREATE TABLE asset_ttb_cache (
    asset_vid       VARCHAR(64)    NOT NULL,
    chain_position  ENUM('entrance','intermediate','target') NOT NULL,
    computed_at     DATETIME(3)    NOT NULL,
    nebula_hash     VARCHAR(64)    NOT NULL COMMENT 'Asset hash at computation time (ALG-REQ-040)',
    ttb_total       DOUBLE         NOT NULL,
    orientation_time DOUBLE        NOT NULL,
    breakdown_json  JSON           NOT NULL COMMENT 'Full tactic-step breakdown as JSON array',
    is_valid        BOOLEAN        NOT NULL DEFAULT TRUE,

    PRIMARY KEY (asset_vid, chain_position),
    INDEX idx_valid (is_valid)
) ENGINE=InnoDB;
```

**`breakdown_json` format:** A JSON array matching the TTBLogEntry structure from ALG-REQ-079, enriched with TTT detail:

```json
[
  {
    "tactic_seq": 0,
    "tactic_id": "TA0001",
    "tactic_name": "Initial Access",
    "technique_vid": "T1078",
    "technique_id": "T1078",
    "technique_name": "Valid Accounts",
    "ttt_hours": 0.1667,
    "candidates_count": 3,
    "ttt_detail": {
      "exec_min": 0.1667,
      "exec_max": 48.0,
      "P": 2,
      "A": 0,
      "maturity_factor": 0.0,
      "formula_case": "partial"
    }
  }
]
```

>Design note: JSON storage is chosen over additional relational tables for the cache because: (a) the cache is read as a single unit (entire breakdown for one asset), never queried by individual fields; (b) it simplifies the write path (single UPSERT instead of multi-table transaction); (c) MariaDB 10.6+ has native JSON support with extraction functions if field-level queries are ever needed.

### ADR-REQ-021: Cache Invalidation

The asset TTB cache SHALL be invalidated when the asset's state changes in NebulaGraph. The invalidation mechanism ties into the existing hash/stale system (ALG-REQ-040–042):

1. When a mitigation is added, modified, or removed on an asset (`PUT/DELETE /api/asset/{id}/mitigations`), the APP layer already calls `InvalidateAssetHash()` in NebulaGraph.
2. **Additionally**, the APP layer SHALL set `is_valid = FALSE` in `asset_ttb_cache` for all rows matching that `asset_vid`.
3. On next TTB computation for that asset, the new breakdown replaces the cached entry (UPSERT).

**Cache read logic:**
- If `is_valid == TRUE` and `nebula_hash` matches the asset's current hash in NebulaGraph → serve from cache.
- Otherwise → recompute from NebulaGraph, update cache.

>Design note: The hash comparison provides a defence-in-depth check beyond the `is_valid` flag. Even if the flag was not properly cleared (e.g., due to a code path omission), the hash mismatch will trigger recomputation.

### ADR-REQ-022: Cache Population

The asset TTB cache SHALL be populated as a side effect of TTB computation. Whenever `ComputeTTB` runs (whether for path-scoped recalculation, bulk recalculation, or on-the-fly entry/target computation), the resulting breakdown SHALL be written to the cache.

This happens during the async flush (ADR-REQ-030) — not inline with the computation.

---

## 6. Asynchronous Write Mechanism

### ADR-REQ-030: Write-Behind Buffer

During path calculation, the APP layer SHALL collect audit records into an in-memory buffer (Go slice) that is **local to the HTTP request goroutine**. No shared state, no mutexes, no channels are needed for the buffer itself.

**Buffer structure (Go):**

```go
type AuditBuffer struct {
    Session       SessionRecord
    Paths         []PathRecord
    Breakdowns    []BreakdownRecord
    TacticSteps   []TacticStepRecord
    TTTDetails    []TTTDetailRecord
    CacheEntries  []CacheEntry
}
```

The buffer is created at the start of the path calculation handler, passed through the computation functions (by pointer), and populated as a side effect of computation. The computation logic itself is unchanged — it still queries NebulaGraph and returns the same results. The only addition is appending records to the buffer at appropriate points.

### ADR-REQ-031: Async Flush Goroutine

After the HTTP response is sent to the client, the handler SHALL launch a background goroutine to flush the buffer to MariaDB:

```go
func (h *PathsHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    buf := &AuditBuffer{}

    // ... existing path calculation logic, populating buf ...

    // Send response to client (existing code)
    json.NewEncoder(w).Encode(response)

    // Async flush — non-blocking, fire-and-forget
    go h.auditStore.FlushBatch(buf)
}
```

**FlushBatch behaviour:**
1. Open a transaction on the MariaDB connection pool.
2. INSERT the session record; capture the auto-generated `session_id`.
3. Batch INSERT path records (with `session_id` FK).
4. Batch INSERT breakdown records (with `session_id` FK); capture `breakdown_id`s.
5. Batch INSERT tactic step records (with `breakdown_id` FKs); capture `step_id`s.
6. Batch INSERT TTT detail records (with `step_id` FKs).
7. UPSERT cache entries into `asset_ttb_cache`.
8. COMMIT the transaction.
9. If any step fails: ROLLBACK, log the error at warning level. No retry — the data is rebuildable.

>Design note: The goroutine has no deadline or timeout. In practice, the entire flush for a typical calculation (50 paths, 3 breakdowns, ~24 tactic steps, ~24 TTT details) should complete in < 100ms on a local MariaDB. If it takes longer, it doesn't matter — the user is already looking at results.

### ADR-REQ-032: Batch INSERT Efficiency

To minimize round-trips to MariaDB, the flush SHALL use multi-row INSERT statements:

```sql
INSERT INTO calc_paths (session_id, path_seq, host_chain, hop_count, tta_hours)
VALUES (?, ?, ?, ?, ?), (?, ?, ?, ?, ?), (?, ?, ?, ?, ?), ...;
```

The maximum batch size per INSERT statement SHALL be 500 rows. If a layer has more than 500 records, multiple INSERT statements are used within the same transaction.

### ADR-REQ-033: Graceful Degradation on RDBMS Failure

If the RDBMS is unavailable or the flush fails:
1. The error SHALL be logged at `WARNING` level (not `ERROR` — the primary function succeeded).
2. The audit buffer SHALL be discarded.
3. No retry mechanism is implemented for the PoC.
4. The next calculation will create a new buffer and attempt to flush again.
5. The user experience is unaffected — they received their path results before the flush was attempted.

---

## 7. Computation Instrumentation

### ADR-REQ-040: Audit Record Generation Points

The following functions SHALL be instrumented to populate the audit buffer:

| Function          | Records Generated                          | Layer |
|-------------------|--------------------------------------------|-------|
| `PathsHandler`    | `SessionRecord`                            | 1     |
| `PathsHandler`    | `PathRecord` (per path, top N)             | 2     |
| `ComputeTTB`      | `BreakdownRecord`                          | 3     |
| `ComputeTTB` loop | `TacticStepRecord` (per tactic)            | 3A    |
| `computeBatchTTT` | `TTTDetailRecord` (per selected technique) | 4     |

**Path record limit:** To avoid excessive storage, only the top N paths (by TTA ascending) SHALL be recorded in the audit trail. Default N = 100. Configurable via environment variable `AUDIT_MAX_PATHS` (ADR-REQ-002 extension).

>Design note: The instrumentation does NOT modify any computation logic or return values. It appends records to the buffer — a pointer-receiver append operation taking nanoseconds. The performance impact is negligible.

### ADR-REQ-041: Buffer Passing Convention

The audit buffer SHALL be passed through the computation stack as an optional parameter. To minimize changes to existing function signatures, the buffer pointer SHALL be stored in a lightweight context struct:

```go
type ComputeContext struct {
    Audit *AuditBuffer  // nil if RDBMS disabled
}
```

Functions that generate audit records accept a `*ComputeContext` parameter. If `ctx` is nil or `ctx.Audit` is nil, no records are generated (zero overhead when RDBMS is disabled).

>Design note: This is preferred over Go's `context.Context` value bag because: (a) it's typed and explicit; (b) it doesn't require interface assertions; (c) it's clear in the function signature that audit data may be collected.

---

## 8. New API Endpoints

### ADR-REQ-050: Path Detail Endpoint

The APP layer SHALL provide an API endpoint that returns the full TTB/TTT breakdown for a specific path from a calculation session.

**Endpoint:** `GET /api/path-detail?session={sessionId}&path={pathSeq}`

**Response:** JSON containing the path's host chain with, for each hop, the TTB breakdown (per-tactic technique selection and TTT values).

```json
{
  "session_id": 42,
  "path_seq": 1,
  "host_chain": "A00017 -> A00012 -> A00011",
  "tta_hours": 8.5842,
  "hops": [
    {
      "asset_vid": "A00017",
      "asset_name": "WS2",
      "chain_position": "entrance",
      "ttb_hours": 2.7503,
      "source": "computed",
      "breakdown": [
        {
          "tactic_seq": 0,
          "tactic_id": "TA0001",
          "tactic_name": "Initial Access",
          "technique_id": "T1078",
          "technique_name": "Valid Accounts",
          "ttt_hours": 0.1667,
          "candidates_count": 3,
          "ttt_detail": {
            "exec_min": 0.1667,
            "exec_max": 48.0,
            "P": 2,
            "A": 0,
            "maturity_factor": 0.0,
            "formula_case": "partial"
          }
        }
      ]
    }
  ]
}
```

**Data source priority:**
1. If audit data exists for this session+path → serve from `calc_*` tables.
2. If no audit data but cache exists for the asset → serve from `asset_ttb_cache`.
3. If neither exists → return `"source": "unavailable"` with the stored TTB value only (no breakdown).

> Design note: candidate for the future review.

### ADR-REQ-051: Calculation History Endpoint

The APP layer SHALL provide an API endpoint that returns recent calculation sessions.

**Endpoint:** `GET /api/calc-history?limit={n}`

**Response:** JSON array of recent sessions with summary data (no path details).

```json
{
  "sessions": [
    {
      "session_id": 42,
      "created_at": "2026-03-12T14:20:49.156Z",
      "entry_asset_id": "A00017",
      "target_asset_id": "A00011",
      "max_hops": 7,
      "paths_found": 4955,
      "total_time_ms": 4051,
      "params": {
        "orientation_time": 0.25,
        "switchover_time": 0.1667,
        "priority_tolerance": 1
      }
    }
  ]
}
```

### ADR-REQ-052: Asset TTB Cache Endpoint

The APP layer SHALL provide an API endpoint that returns the cached TTB breakdown for a specific asset.

**Endpoint:** `GET /api/asset/{id}/ttb-detail?position={entrance|intermediate|target}`

**Response:** The cached breakdown from `asset_ttb_cache`, or a 404 if no cache exists.

**Use case:** The VIS layer can call this endpoint when a user clicks on an asset in a highlighted path, showing the per-tactic TTB composition in the Asset Inspector panel without triggering any NebulaGraph queries.

---

## 9. Configuration Storage (Future)

### ADR-REQ-060: Configuration Table (Placeholder)

A future version of ADR SHALL define a `config_params` table for storing application configuration persistently, replacing or supplementing the current environment-variable-only approach (SRS REQ-002).

Candidate configuration items:
- TTB calculation parameters (currently env vars: `TTB_ORIENTATION_TIME`, `TTB_SWITCHOVER_TIME`, `TTB_PRIORITY_TOLERANCE`)
- UI preferences (theme, default hop count, default entry/target)
- Audit settings (max paths to store, retention period)

>Design note: For the PoC, environment variables remain the primary configuration mechanism. The RDBMS config table is a future enhancement that enables runtime configuration changes without application restart.

---

## 10. Data Retention

### ADR-REQ-070: Audit Trail Retention

Calculation session records and their associated path/breakdown/TTT detail records SHALL be retained for a configurable period. Default: **30 days**.

**Cleanup mechanism:** A scheduled task (or application startup routine) SHALL delete sessions older than the retention period:

```sql
DELETE FROM calc_sessions
WHERE created_at < DATE_SUB(NOW(), INTERVAL 30 DAY);
```

Cascade deletes on foreign keys automatically remove associated path, breakdown, step, and detail records.

> Design note: candidate for teh future review - must be a parameter for an admin interface.

### ADR-REQ-071: Cache Retention

Asset TTB cache entries have no time-based retention. They are:
- Overwritten when a new computation occurs for the same (asset, chain_position) pair.
- Invalidated (soft) when the asset's state changes.
- Deleted only if the asset is removed from NebulaGraph (manual cleanup, future).

---

## 11. Go Package Structure

### ADR-REQ-080: Package Layout

The RDBMS functionality SHALL be implemented in a new Go package within the existing project structure:

```
internal/
  store/
    store.go        — MariaDB connection pool, FlushBatch, query methods
    models.go       — AuditBuffer, SessionRecord, PathRecord, etc.
    migrations.go   — Schema creation (CREATE TABLE IF NOT EXISTS)
```

The `store` package SHALL be imported by `api/handler.go` and `cmd/asset-viz/main.go`. It SHALL NOT be imported by `internal/nebula/client.go` — the Nebula client remains independent of the RDBMS.

### ADR-REQ-081: Schema Migration

On application startup, the `store` package SHALL execute `CREATE TABLE IF NOT EXISTS` for all tables defined in this document. This ensures the schema is always up-to-date without requiring manual migration steps.

>Design note: For a PoC, `CREATE TABLE IF NOT EXISTS` is sufficient. Production systems would use a proper migration tool (e.g., golang-migrate). The current approach is simple and idempotent.

---

## 12. Cross-Reference Matrix

### 12.1 ADR-REQ to SRS

| ADR-REQ     | SRS Reference | Context                                     |
|-------------|---------------|---------------------------------------------|
| ADR-REQ-001 | CMP004        | RDBMS component definition                  |
| ADR-REQ-002 | REQ-002       | Environment variable configuration pattern  |
| ADR-REQ-003 | REQ-121       | Go APP layer uses go-sql-driver/mysql       |
| ADR-REQ-050 | —             | New endpoint, to be added to SRS Appendix C |
| ADR-REQ-051 | —             | New endpoint, to be added to SRS Appendix C |
| ADR-REQ-052 | —             | New endpoint, to be added to SRS Appendix C |

### 12.2 ADR-REQ to ALGO

| ADR-REQ     | ALGO Reference   | Context                                     |
|-------------|------------------|---------------------------------------------|
| ADR-REQ-010 | ALG-REQ-001      | Session maps to path calculation invocation |
| ADR-REQ-012 | ALG-REQ-079      | TTBLogEntry structure persisted             |
| ADR-REQ-013 | ALG-REQ-079      | Tactic step detail                          |
| ADR-REQ-014 | ALG-REQ-060      | TTT formula inputs persisted                |
| ADR-REQ-020 | ALG-REQ-040, 042 | Cache invalidation tied to asset hash       |
| ADR-REQ-040 | ALG-REQ-070, 060 | Instrumentation points in TTB/TTT flow      |

### 12.3 ADR-REQ to SCHEMA

| ADR-REQ     | SCHEMA Reference       | Context                                          |
|-------------|------------------------|--------------------------------------------------|
| ADR-REQ-020 | TA001 (Asset.hash)     | Hash comparison for cache validity               |
| ADR-REQ-021 | ED001 (applied_to)     | Mitigation changes trigger cache invalidation    |

### 12.4 ADR-REQ to UIS

| ADR-REQ     | UIS Reference          | Context                                          |
|-------------|------------------------|--------------------------------------------------|
| ADR-REQ-050 | UI-REQ-207 (future)    | Path detail drill-down from path table           |
| ADR-REQ-052 | UI-REQ-210 (future)    | TTB breakdown in Asset Inspector                 |

---

## 13. Entity-Relationship Diagram

```
┌──────────────────┐
│  calc_sessions   │
│  (Layer 1)       │
│                  │
│  session_id (PK) │
│  entry_asset_id  │
│  target_asset_id │
│  max_hops        │
│  params...       │
│  paths_found     │
│  timing...       │
└────────┬─────────┘
         │ 1:N
         ▼
┌──────────────────┐       ┌──────────────────────┐
│  calc_paths      │       │  calc_ttb_breakdown   │
│  (Layer 2)       │       │  (Layer 3)            │
│                  │       │                       │
│  path_id (PK)    │       │  breakdown_id (PK)    │
│  session_id (FK) │       │  session_id (FK)      │
│  path_seq        │       │  asset_vid            │
│  host_chain      │       │  chain_position       │
│  tta_hours       │       │  ttb_total            │
└──────────────────┘       └────────┬──────────────┘
                                    │ 1:N
                                    ▼
                           ┌──────────────────────┐
                           │ calc_ttb_tactic_steps │
                           │ (Layer 3A)            │
                           │                       │
                           │ step_id (PK)          │
                           │ breakdown_id (FK)     │
                           │ tactic_seq            │
                           │ technique_vid         │
                           │ ttt_hours             │
                           └────────┬──────────────┘
                                    │ 1:1
                                    ▼
                           ┌──────────────────────┐
                           │  calc_ttt_detail      │
                           │  (Layer 4)            │
                           │                       │
                           │  detail_id (PK)       │
                           │  step_id (FK)         │
                           │  P, A, maturity       │
                           │  formula_case         │
                           │  ttt_hours            │
                           └──────────────────────┘


┌──────────────────────┐
│  asset_ttb_cache     │
│  (Persistent Cache)  │
│                      │
│  (asset_vid,         │
│   chain_position) PK │
│  nebula_hash         │
│  breakdown_json      │
│  is_valid            │
└──────────────────────┘
```

---

## 14. Open Items

- [ ] ADR-REQ-060: Configuration table schema (deferred to future version)
- [ ] ADR-REQ-050: UI integration for path detail drill-down (requires UIS amendment)
- [ ] Determine if `calc_paths` should store all paths or top-N; current default N=100
- [ ] Consider adding `calc_ttb_tactic_steps.all_candidates_json` column to record rejected candidates (full transparency vs storage trade-off)
- [x] MariaDB installation and user setup on `nebble.m82` — practical deployment step
- [ ] Determine whether separate tactic step records for non-selected (rejected) candidates should be stored (Layer 3B — candidate detail)
- [ ] Storing UI parameters (like gravity for Cytoscape graph rendering)

---

## Change Log

| Version | Date         | Changes                                                        | Author          |
|---------|--------------|----------------------------------------------------------------|-----------------|
| 0.1     | Mar 12, 2026 | Initial draft — audit trail, cache, async write, API endpoints | AI + K. Smirnov |

---

**End of Auxiliary Database Requirements**
