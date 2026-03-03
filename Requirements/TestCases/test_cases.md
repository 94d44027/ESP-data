# ESP PoC — Test Cases
## Version 1.0 — March 3, 2026

---

## 1. Structure and Conventions

### 1.1 Test Case ID Format

| Prefix   | Layer                                   | Example       |
|----------|-----------------------------------------|---------------|
| `TC-API` | APP layer (Go handlers, HTTP API)       | TC-API-020-01 |
| `TC-DB`  | GRDB layer (nGQL queries via client.go) | TC-DB-042-01  |
| `TC-UI`  | VIS layer (JavaScript, HTML/CSS)        | TC-UI-112-01  |
| `TC-INT` | Integration (end-to-end across layers)  | TC-INT-046-01 |

The numeric part after the prefix maps to the **primary requirement** under test (e.g., `020` → REQ-020).

### 1.2 Columns

| Column              | Description                                           |
|---------------------|-------------------------------------------------------|
| **ID**              | Unique test case identifier                           |
| **Requirement(s)**  | Primary and related requirement IDs                   |
| **Title**           | Short description                                     |
| **Preconditions**   | State required before execution                       |
| **Steps**           | Numbered actions                                      |
| **Expected Result** | Observable outcome                                    |
| **Priority**        | P1 (must-pass) / P2 (should-pass) / P3 (nice-to-have) |

### 1.3 Test Data Assumptions

- Database: NebulaGraph 3.8 with the ESP01 schema fully applied
- Dataset: 63 assets per the demo dataset, with at least one entry point (is_entrance=true) and one target (is_target=true)
- SystemState vertex `"SYS001"` exists with initial values
- At least two assets connected via `connects_to` edges
- At least one `applied_to` edge exists between a tMitreMitigation and an Asset

---

## 2. APP Layer — API Endpoint Tests

### 2.1 Graph Data (REQ-020, REQ-027)

| ID            | Requirement(s) | Title                                  | Preconditions                               | Steps                                               | Expected Result                                                                                                                                           | Priority |
|---------------|----------------|----------------------------------------|---------------------------------------------|-----------------------------------------------------|-----------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| TC-API-020-01 | REQ-020        | Graph endpoint returns nodes and edges | Server running, DB connected                | 1. `GET /api/graph`                                 | 200 OK; JSON with `nodes[]` and `edges[]`; nodes have `asset_id`, `asset_name`, `is_entrance`, `is_target`, `priority`, `has_vulnerability`, `asset_type` | P1       |
| TC-API-020-02 | REQ-020        | Graph data reflects DB content         | Known dataset loaded                        | 1. `GET /api/graph` 2. Count nodes                  | Node count matches LOOKUP ON Asset count                                                                                                                  | P1       |
| TC-API-027-01 | REQ-027        | Edge de-duplication                    | Two assets with multiple ranked connections | 1. `GET /api/graph` 2. Count edges between the pair | Exactly one edge per (source, target) pair in response                                                                                                    | P1       |

### 2.2 Asset List and Detail (REQ-021, REQ-022, REQ-023, REQ-024)

| ID            | Requirement(s)   | Title                                | Preconditions                | Steps                            | Expected Result                                                                                                                                            | Priority |
|---------------|------------------|--------------------------------------|------------------------------|----------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------|----------|
| TC-API-021-01 | REQ-021          | Assets list returns all assets       | DB connected                 | 1. `GET /api/assets`             | 200 OK; `assets[]` with `asset_id`, `asset_name`, `asset_type` for each; `total` matches dataset size                                                      | P1       |
| TC-API-021-02 | REQ-021          | Assets list supports type filter     | Multiple asset types exist   | 1. `GET /api/assets?type=Server` | Only assets of type "Server" returned; `filtered` count matches                                                                                            | P2       |
| TC-API-022-01 | REQ-022          | Asset detail returns full properties | Asset "A00001" exists        | 1. `GET /api/asset/A00001`       | 200 OK; response includes `asset_id`, `asset_name`, `asset_description`, `priority`, `has_vulnerability`, `TTB`, `os_name`, `network_segment`, `type_name` | P1       |
| TC-API-022-02 | REQ-022, REQ-025 | Asset detail rejects invalid ID      | —                            | 1. `GET /api/asset/INVALID`      | 400 Bad Request with descriptive error                                                                                                                     | P1       |
| TC-API-023-01 | REQ-023          | Neighbors returns connected assets   | Asset "A00001" has neighbors | 1. `GET /api/neighbors/A00001`   | 200 OK; array of neighbors with `neighbor_id`, `direction` ("inbound" or "outbound")                                                                       | P1       |
| TC-API-024-01 | REQ-024          | Asset types returns distinct types   | Types exist in DB            | 1. `GET /api/asset-types`        | 200 OK; array of `{ type_id, type_name }` objects                                                                                                          | P1       |

### 2.3 Edge Connections (REQ-026)

| ID            | Requirement(s)   | Title                                 | Preconditions                 | Steps                              | Expected Result                                                                                         | Priority |
|---------------|------------------|---------------------------------------|-------------------------------|------------------------------------|---------------------------------------------------------------------------------------------------------|----------|
| TC-API-026-01 | REQ-026          | Edge detail returns all connections   | A00001→A00003 has connections | 1. `GET /api/edges/A00001/A00003`  | 200 OK; `connections[]` with `protocol`, `port`, `rank`; `source` and `target` asset summaries included | P1       |
| TC-API-026-02 | REQ-026, REQ-025 | Edge detail rejects invalid source ID | —                             | 1. `GET /api/edges/INVALID/A00003` | 400 Bad Request                                                                                         | P1       |

### 2.4 Path Calculation (ALG-REQ-001, ALG-REQ-010, ALG-REQ-011)

| ID             | Requirement(s) | Title                                | Preconditions                                          | Steps                                                                         | Expected Result                                                               | Priority |
|----------------|----------------|--------------------------------------|--------------------------------------------------------|-------------------------------------------------------------------------------|-------------------------------------------------------------------------------|----------|
| TC-API-PATH-01 | ALG-REQ-001    | Path calculation returns paths       | Entry A00013 and target A00011 exist and are connected | 1. `GET /api/paths?from=A00013&to=A00011&hops=6`                              | 200 OK; `paths[]` non-empty; `entry_point`, `target`, `hops`, `total` present | P1       |
| TC-API-PATH-02 | ALG-REQ-010    | TTA excludes entry point TTB         | Same as above                                          | 1. Run path query 2. Verify TTA for a path                                    | TTA = sum of TTB for nodes 2..k (entry point's TTB excluded)                  | P1       |
| TC-API-PATH-03 | ALG-REQ-011    | Path IDs follow Pxxxxx format        | Same as above                                          | 1. Run path query 2. Check path_id values                                     | Each `path_id` matches `^P\d{5}$` (e.g., P00001)                              | P2       |
| TC-API-PATH-04 | ALG-REQ-031    | Paths are loop-free                  | Same as above                                          | 1. Run path query 2. For each path, parse hosts                               | No asset_id appears more than once in any single path                         | P1       |
| TC-API-PATH-05 | ALG-REQ-033    | Hops validation rejects out-of-range | —                                                      | 1. `GET /api/paths?from=A00013&to=A00011&hops=1` 2. `hops=10` 3. `hops=abc`   | 400 Bad Request for all three                                                 | P1       |
| TC-API-PATH-06 | ALG-REQ-030    | No paths returns empty array         | Entry and target are disconnected                      | 1. `GET /api/paths?from={disconnected_entry}&to={disconnected_target}&hops=6` | 200 OK; `paths: [], total: 0`                                                 | P1       |
| TC-API-PATH-07 | REQ-025        | Path rejects invalid asset IDs       | —                                                      | 1. `GET /api/paths?from=INVALID&to=A00011&hops=6`                             | 400 Bad Request                                                               | P1       |

### 2.5 Entry Points and Targets (ALG-REQ-002, ALG-REQ-003)

| ID            | Requirement(s) | Title                                | Preconditions                            | Steps                      | Expected Result                                                                 | Priority |
|---------------|----------------|--------------------------------------|------------------------------------------|----------------------------|---------------------------------------------------------------------------------|----------|
| TC-API-EP-01  | ALG-REQ-002    | Entry points returns entrance assets | At least one asset with is_entrance=true | 1. `GET /api/entry-points` | 200 OK; `entry_points[]` with `asset_id`, `asset_name`; all are entrance assets | P1       |
| TC-API-TGT-01 | ALG-REQ-003    | Targets returns target assets        | At least one asset with is_target=true   | 1. `GET /api/targets`      | 200 OK; `targets[]` with `asset_id`, `asset_name`; all are target assets        | P1       |

### 2.6 Mitigations CRUD (REQ-033 through REQ-039)

| ID            | Requirement(s) | Title                                          | Preconditions                                    | Steps                                                                                                                                       | Expected Result                                                                       | Priority |
|---------------|----------------|------------------------------------------------|--------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------------------------------------|----------|
| TC-API-MIT-01 | REQ-033        | Mitigations list returns all MITRE mitigations | Mitigations exist in DB                          | 1. `GET /api/mitigations`                                                                                                                   | 200 OK; `mitigations[]` with `mitigation_id`, `mitigation_name`; `total` matches      | P1       |
| TC-API-MIT-02 | REQ-034        | Asset mitigations returns applied mitigations  | Asset A00001 has at least one applied mitigation | 1. `GET /api/asset/A00001/mitigations`                                                                                                      | 200 OK; `mitigations[]` with `mitigation_id`, `mitigation_name`, `maturity`, `active` | P1       |
| TC-API-MIT-03 | REQ-035        | Upsert mitigation creates new edge             | Asset A00005 has no applied mitigation M0001     | 1. `PUT /api/asset/A00005/mitigations` with `{"mitigation_id":"M0001","maturity":50,"active":true}` 2. `GET /api/asset/A00005/mitigations`  | Step 1: 200 OK `{"status":"ok"}`; Step 2: M0001 appears with maturity=50              | P1       |
| TC-API-MIT-04 | REQ-035        | Upsert mitigation updates existing edge        | M0001 already applied to A00005                  | 1. `PUT /api/asset/A00005/mitigations` with `{"mitigation_id":"M0001","maturity":80,"active":false}` 2. `GET /api/asset/A00005/mitigations` | Step 2: M0001 now has maturity=80, active=false                                       | P1       |
| TC-API-MIT-05 | REQ-036        | Delete mitigation removes edge                 | M0001 applied to A00005                          | 1. `DELETE /api/asset/A00005/mitigations/M0001` 2. `GET /api/asset/A00005/mitigations`                                                      | Step 1: 200 OK; Step 2: M0001 no longer in list                                       | P1       |
| TC-API-MIT-06 | REQ-038        | Upsert rejects invalid mitigation ID           | —                                                | 1. `PUT /api/asset/A00005/mitigations` with `{"mitigation_id":"INVALID","maturity":50,"active":true}`                                       | 400 Bad Request                                                                       | P1       |
| TC-API-MIT-07 | REQ-039        | Upsert rejects invalid maturity                | —                                                | 1. `PUT /api/asset/A00005/mitigations` with `{"mitigation_id":"M0001","maturity":33,"active":true}`                                         | 400 Bad Request; only {25, 50, 80, 100} accepted                                      | P1       |

### 2.7 Recalculate TTB (REQ-040, ALG-REQ-045)

| ID               | Requirement(s)       | Title                                | Preconditions                                     | Steps                                                                         | Expected Result                                                                    | Priority |
|------------------|----------------------|--------------------------------------|---------------------------------------------------|-------------------------------------------------------------------------------|------------------------------------------------------------------------------------|----------|
| TC-API-RECALC-01 | REQ-040, ALG-REQ-045 | Bulk recalculation endpoint succeeds | At least one asset with hash_valid=false          | 1. `POST /api/recalculate-ttb`                                                | 200 OK; response contains `recalculated` (>0), `unchanged`, `total`, `merkle_root` | P1       |
| TC-API-RECALC-02 | REQ-040, ALG-REQ-045 | Recalculation with no stale assets   | All assets have hash_valid=true                   | 1. `POST /api/recalculate-ttb`                                                | 200 OK; `recalculated: 0`, `unchanged: 0`                                          | P1       |
| TC-API-RECALC-03 | REQ-040, ALG-REQ-044 | TTB increases after recalculation    | Asset A00005 has hash_valid=false, current TTB=10 | 1. Note current TTB 2. `POST /api/recalculate-ttb` 3. `GET /api/asset/A00005` | New TTB = old TTB + [1..10]                                                        | P1       |
| TC-API-RECALC-04 | REQ-040              | Rejects non-POST method              | —                                                 | 1. `GET /api/recalculate-ttb`                                                 | 405 Method Not Allowed                                                             | P2       |

### 2.8 System State (REQ-041, ALG-REQ-048)

| ID              | Requirement(s)       | Title                                     | Preconditions              | Steps                                                     | Expected Result                                                                                   | Priority |
|-----------------|----------------------|-------------------------------------------|----------------------------|-----------------------------------------------------------|---------------------------------------------------------------------------------------------------|----------|
| TC-API-STATE-01 | REQ-041, ALG-REQ-048 | System state returns SYS001 data          | SystemState vertex exists  | 1. `GET /api/system-state`                                | 200 OK; response has `state_id`, `merkle_root`, `last_recalc_time`, `total_assets`, `stale_count` | P1       |
| TC-API-STATE-02 | REQ-041              | stale_count reflects invalidated assets   | One asset just invalidated | 1. `PUT` a mitigation 2. `GET /api/system-state`          | `stale_count` > 0                                                                                 | P1       |
| TC-API-STATE-03 | REQ-041, ALG-REQ-045 | stale_count resets to 0 after bulk recalc | stale_count > 0            | 1. `POST /api/recalculate-ttb` 2. `GET /api/system-state` | `stale_count: 0`                                                                                  | P1       |

### 2.9 Hash Invalidation on Mitigation Write (REQ-042, ALG-REQ-043)

| ID              | Requirement(s)       | Title                                          | Preconditions                                       | Steps                                                                                                                     | Expected Result                                         | Priority |
|-----------------|----------------------|------------------------------------------------|-----------------------------------------------------|---------------------------------------------------------------------------------------------------------------------------|---------------------------------------------------------|----------|
| TC-API-INVAL-01 | REQ-042, ALG-REQ-043 | Upsert invalidates asset hash                  | Asset A00005 has hash_valid=true                    | 1. `PUT /api/asset/A00005/mitigations` with valid body 2. Query DB: `FETCH PROP ON Asset "A00005" YIELD Asset.hash_valid` | hash_valid = false                                      | P1       |
| TC-API-INVAL-02 | REQ-042, ALG-REQ-043 | Delete invalidates asset hash                  | A00005 has hash_valid=true and a mitigation applied | 1. `DELETE /api/asset/A00005/mitigations/M0001` 2. Query DB: `FETCH PROP ON Asset "A00005" YIELD Asset.hash_valid`        | hash_valid = false                                      | P1       |
| TC-API-INVAL-03 | REQ-042, ALG-REQ-043 | Invalidation increments stale_count            | stale_count = N                                     | 1. `PUT` a mitigation on any asset 2. `GET /api/system-state`                                                             | stale_count = N + 1                                     | P1       |
| TC-API-INVAL-04 | REQ-042              | Mitigation succeeds even if invalidation fails | SystemState vertex missing (edge case)              | 1. Remove SYS001 vertex 2. `PUT` a mitigation                                                                             | 200 OK `{"status":"ok"}`; error logged in server output | P2       |

---

## 3. GRDB Layer — Database Query Tests

### 3.1 Hash Computation (ALG-REQ-042)

| ID           | Requirement(s)           | Title                                               | Preconditions                            | Steps                                                                                             | Expected Result                                                                | Priority |
|--------------|--------------------------|-----------------------------------------------------|------------------------------------------|---------------------------------------------------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| TC-DB-042-01 | ALG-REQ-042              | Hash query returns computed hashes for stale assets | At least one asset with hash_valid=false | 1. Run hash computation query (ALG-REQ-042 reference)                                             | Returns asset_id, current_ttb, stored_hash, computed_hash for each stale asset | P1       |
| TC-DB-042-02 | ALG-REQ-042              | Hash is deterministic                               | Same DB state                            | 1. Run hash query twice                                                                           | Same computed_hash for each asset in both runs                                 | P1       |
| TC-DB-042-03 | ALG-REQ-042, ALG-REQ-041 | Hash changes when mitigation added                  | Asset X has known hash H1                | 1. Note computed_hash 2. Add a mitigation to asset X 3. Set hash_valid=false 4. Re-run hash query | computed_hash differs from H1                                                  | P1       |
| TC-DB-042-04 | ALG-REQ-042              | Hash query returns empty for no stale assets        | All hash_valid=true                      | 1. Run hash computation query                                                                     | Empty result set (0 rows)                                                      | P2       |

### 3.2 Scoped Hash Computation (ALG-REQ-046)

| ID           | Requirement(s) | Title                                            | Preconditions                    | Steps                                                      | Expected Result                                   | Priority |
|--------------|----------------|--------------------------------------------------|----------------------------------|------------------------------------------------------------|---------------------------------------------------|----------|
| TC-DB-046-01 | ALG-REQ-046    | Scoped query returns only requested stale assets | A00005 is stale, A00001 is valid | 1. Run scoped query with `IN ["A00005", "A00001"]`         | Only A00005 returned (A00001 has hash_valid=true) | P1       |
| TC-DB-046-02 | ALG-REQ-046    | Scoped query produces same hash as full query    | A00005 is stale                  | 1. Run full hash query 2. Run scoped query for A00005 only | computed_hash matches in both                     | P1       |

### 3.3 Merkle Root (ALG-REQ-047)

| ID           | Requirement(s) | Title                                           | Preconditions                 | Steps                                                                                                                                                                                      | Expected Result            | Priority |
|--------------|----------------|-------------------------------------------------|-------------------------------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|----------------------------|----------|
| TC-DB-047-01 | ALG-REQ-047    | Merkle root query returns int64                 | Assets exist with hash values | 1. Run: `LOOKUP ON Asset YIELD Asset.Asset_ID AS aid, Asset.hash AS hash \| ORDER BY $-.aid \| YIELD collect($-.hash) AS all \| YIELD hash(reduce(s="",x IN $-.all \| s+x+";")) AS merkle` | Returns single int64 value | P1       |
| TC-DB-047-02 | ALG-REQ-047    | Merkle root changes when any asset hash changes | Known Merkle root M1          | 1. Update one asset's hash 2. Re-run Merkle query                                                                                                                                          | New Merkle root ≠ M1       | P1       |
| TC-DB-047-03 | ALG-REQ-047    | Merkle root is deterministic                    | Same DB state                 | 1. Run Merkle query twice                                                                                                                                                                  | Same value both times      | P2       |

### 3.4 SystemState CRUD

| ID           | Requirement(s) | Title                                 | Preconditions   | Steps                                                                                                                                | Expected Result                                                            | Priority |
|--------------|----------------|---------------------------------------|-----------------|--------------------------------------------------------------------------------------------------------------------------------------|----------------------------------------------------------------------------|----------|
| TC-DB-SYS-01 | ALG-REQ-048    | FETCH PROP returns SYS001             | Vertex exists   | 1. `FETCH PROP ON SystemState "SYS001" YIELD ...`                                                                                    | Returns state_id, merkle_root, last_recalc_time, total_assets, stale_count | P1       |
| TC-DB-SYS-02 | ALG-REQ-043    | stale_count increment is atomic       | stale_count = N | 1. `UPDATE VERTEX ON SystemState "SYS001" SET stale_count = stale_count + 1` 2. Fetch                                                | stale_count = N + 1                                                        | P1       |
| TC-DB-SYS-03 | ALG-REQ-045    | SystemState update resets stale_count | stale_count > 0 | 1. `UPDATE VERTEX ON SystemState "SYS001" SET merkle_root=123, last_recalc_time=datetime(), total_assets=63, stale_count=0` 2. Fetch | stale_count = 0, merkle_root = 123                                         | P1       |

---

## 4. VIS Layer — UI Tests

### 4.1 Application Shell (UI-REQ-100, UI-REQ-110)

| ID           | Requirement(s) | Title                          | Preconditions | Steps                  | Expected Result                                                                             | Priority |
|--------------|----------------|--------------------------------|---------------|------------------------|---------------------------------------------------------------------------------------------|----------|
| TC-UI-100-01 | UI-REQ-100     | Three-panel layout renders     | App loaded    | 1. Open app in browser | Left sidebar, center canvas, right inspector visible                                        | P1       |
| TC-UI-110-01 | UI-REQ-110     | Top bar shows node/edge counts | Graph loaded  | 1. Check stats area    | Nodes and Edges counts match API data                                                       | P1       |
| TC-UI-110-02 | UI-REQ-110     | All topbar buttons present     | App loaded    | 1. Inspect topbar      | Sidebar toggle, Inspector toggle, Path Inspector, Recalculate TTBs, Refresh buttons visible | P1       |

### 4.2 Recalculate TTBs Button (UI-REQ-112)

| ID           | Requirement(s) | Title                                | Preconditions                  | Steps                                                  | Expected Result                                                                                                        | Priority |
|--------------|----------------|--------------------------------------|--------------------------------|--------------------------------------------------------|------------------------------------------------------------------------------------------------------------------------|----------|
| TC-UI-112-01 | UI-REQ-112     | Badge shows stale count on load      | stale_count > 0 in SystemState | 1. Load app                                            | Red badge on Recalculate button shows the stale count number                                                           | P1       |
| TC-UI-112-02 | UI-REQ-112     | Badge hidden when stale_count = 0    | stale_count = 0                | 1. Load app                                            | No badge visible on Recalculate button                                                                                 | P1       |
| TC-UI-112-03 | UI-REQ-112     | Click triggers recalculation         | stale_count > 0                | 1. Click Recalculate TTBs button                       | Button shows loading spinner; toast appears with "Recalculated N asset(s)"; badge updates (goes to 0 or reduced count) | P1       |
| TC-UI-112-04 | UI-REQ-112     | Button disabled during recalculation | —                              | 1. Click Recalculate 2. Immediately try clicking again | Second click has no effect (button disabled)                                                                           | P2       |
| TC-UI-112-05 | UI-REQ-112     | Toast auto-dismisses                 | —                              | 1. Click Recalculate 2. Wait 3 seconds                 | Toast fades out                                                                                                        | P2       |
| TC-UI-112-06 | UI-REQ-112     | Error toast on failure               | Server down or endpoint error  | 1. Disconnect server 2. Click Recalculate              | Red-bordered toast "TTB recalculation failed"                                                                          | P2       |

### 4.3 Stale Path Warning (UI-REQ-113)

| ID           | Requirement(s) | Title                                        | Preconditions                                 | Steps                                                                | Expected Result                                                                | Priority |
|--------------|----------------|----------------------------------------------|-----------------------------------------------|----------------------------------------------------------------------|--------------------------------------------------------------------------------|----------|
| TC-UI-113-01 | UI-REQ-113     | Warning appears when paths have stale assets | At least one path member has hash_valid=false | 1. Open Path Inspector 2. Select entry and target 3. Click Calculate | Orange warning bar appears below results: "⚠️ TTB recalculated for N asset(s)" | P1       |
| TC-UI-113-02 | UI-REQ-113     | Warning tooltip shows asset list             | Same as above                                 | 1. Hover over warning bar                                            | Tooltip lists recalculated asset IDs                                           | P2       |
| TC-UI-113-03 | UI-REQ-113     | Warning absent when no recalculation needed  | All path members have hash_valid=true         | 1. Calculate paths                                                   | No warning bar visible                                                         | P1       |
| TC-UI-113-04 | UI-REQ-113     | Badge updates after path-scoped recalc       | Path triggered recalculation                  | 1. Calculate paths with stale members                                | Stale badge in topbar decrements                                               | P2       |

### 4.4 Sidebar (UI-REQ-120, UI-REQ-121, UI-REQ-122, UI-REQ-123, UI-REQ-124)

| ID           | Requirement(s) | Title                            | Preconditions          | Steps                          | Expected Result                                              | Priority |
|--------------|----------------|----------------------------------|------------------------|--------------------------------|--------------------------------------------------------------|----------|
| TC-UI-120-01 | UI-REQ-120     | Asset list populates on load     | App loaded             | 1. Check sidebar               | All assets listed with name and type badge                   | P1       |
| TC-UI-122-01 | UI-REQ-122     | Search filters asset list        | Assets loaded          | 1. Type "Server" in search box | Only assets matching "Server" in name/type remain            | P1       |
| TC-UI-122-02 | UI-REQ-122     | Type checkboxes filter list      | Multiple types         | 1. Check "Server" type filter  | Only Server-type assets shown                                | P1       |
| TC-UI-123-01 | UI-REQ-123     | Sidebar toggle collapses/expands | Sidebar visible        | 1. Click sidebar toggle button | Sidebar collapses; click again restores                      | P1       |
| TC-UI-124-01 | UI-REQ-124     | Graph selection scrolls sidebar  | Asset visible on graph | 1. Click a node on graph       | Sidebar scrolls to show corresponding asset item highlighted | P2       |

### 4.5 Canvas and Graph (UI-REQ-200 through UI-REQ-204)

| ID | Requirement(s) | Title | Preconditions | Steps | Expected Result | Priority |
|---|---|---|---|---|---|---|
| TC-UI-200-01 | UI-REQ-200 | Graph renders all nodes | App loaded | 1. Check canvas | All nodes visible; count matches stat counter | P1 |
| TC-UI-201-01 | UI-REQ-201, REQ-013 | Nodes display labels | Nodes rendered | 1. Zoom to readable level | Each node shows Asset_Name (or Asset_ID fallback) | P1 |
| TC-UI-201-02 | UI-REQ-201, REQ-011 | Node colors distinguish types | Multiple types | 1. Compare nodes | Different asset types have different colors; sufficient contrast | P1 |
| TC-UI-202-01 | UI-REQ-202, REQ-012 | Edges show direction | Edges rendered | 1. Inspect edges | Arrowheads indicate direction | P1 |
| TC-UI-204-01 | UI-REQ-204 | Pan and zoom work | Graph visible | 1. Mouse-wheel zoom 2. Click-drag pan | Graph zooms and pans smoothly | P1 |

### 4.6 Path Inspector (UI-REQ-206 through UI-REQ-209)

| ID           | Requirement(s)         | Title                                      | Preconditions                   | Steps                                                     | Expected Result                                                                        | Priority |
|--------------|------------------------|--------------------------------------------|---------------------------------|-----------------------------------------------------------|----------------------------------------------------------------------------------------|----------|
| TC-UI-206-01 | UI-REQ-206             | Path Inspector opens as side panel         | App loaded                      | 1. Click Path Inspector button (⛓)                        | Side panel slides in from the right; canvas contracts                                  | P1       |
| TC-UI-207-01 | UI-REQ-207             | Dropdowns populate with entry/target lists | Panel open                      | 1. Check Entry Point dropdown 2. Check Target dropdown    | Entry points and targets loaded with asset_id + asset_name                             | P1       |
| TC-UI-207-02 | UI-REQ-207             | Calculate produces results table           | Valid entry and target selected | 1. Select entry 2. Select target 3. Click Calculate Paths | Status shows "Found N path(s)"; table appears with Path ID, Hosts, TTA columns         | P1       |
| TC-UI-208-01 | UI-REQ-208, UI-REQ-332 | Path row click highlights on graph         | Results showing                 | 1. Click a path row                                       | Nodes and edges in that path highlighted on canvas; graph fits to highlighted elements | P1       |
| TC-UI-209-01 | UI-REQ-209             | Close button restores layout               | Panel open                      | 1. Click × button                                         | Panel slides out; canvas expands; path highlights cleared                              | P1       |

### 4.7 Inspector Panel (UI-REQ-210, UI-REQ-212)

| ID           | Requirement(s) | Title                             | Preconditions        | Steps                        | Expected Result                                                                                                | Priority |
|--------------|----------------|-----------------------------------|----------------------|------------------------------|----------------------------------------------------------------------------------------------------------------|----------|
| TC-UI-210-01 | UI-REQ-210     | Node click shows asset detail     | Graph loaded         | 1. Click a node on graph     | Inspector panel shows asset properties: ID, name, description, priority, vulnerability, TTB, OS, segment, type | P1       |
| TC-UI-210-02 | UI-REQ-210     | Inspector shows neighbor list     | Node selected        | 1. Check connections section | Lists inbound and outbound neighbors with direction                                                            | P1       |
| TC-UI-212-01 | UI-REQ-212     | Edge click shows edge connections | Two connected assets | 1. Click an edge on graph    | Edge inspector shows all connections (protocol, port) between the pair, plus source/target summaries           | P1       |

### 4.8 Mitigations Editor (UI-REQ-250 through UI-REQ-258)

| ID           | Requirement(s) | Title                           | Preconditions               | Steps                                                                             | Expected Result                                               | Priority |
|--------------|----------------|---------------------------------|-----------------------------|-----------------------------------------------------------------------------------|---------------------------------------------------------------|----------|
| TC-UI-250-01 | UI-REQ-250     | Shield button opens modal       | Asset selected in inspector | 1. Click shield icon button                                                       | Mitigations Editor modal opens with asset name in title       | P1       |
| TC-UI-252-01 | UI-REQ-252     | Table shows applied mitigations | Asset has mitigations       | 1. Open editor for asset                                                          | Table rows show Mitigation ID, Name, Maturity, Active columns | P1       |
| TC-UI-254-01 | UI-REQ-254     | Inline edit maturity            | Row selected                | 1. Click row 2. Change maturity dropdown to 80 3. Press Enter or Save             | Row updates; green flash; server confirms                     | P1       |
| TC-UI-255-01 | UI-REQ-255     | Add new mitigation              | Editor open                 | 1. Click ➕ Add 2. Select mitigation from dropdown 3. Set maturity, active 4. Save | New row appears; server confirms                              | P1       |
| TC-UI-257-01 | UI-REQ-257     | Delete mitigation               | Row selected                | 1. Click Delete 2. Confirm                                                        | Row removed; server confirms                                  | P1       |
| TC-UI-258-01 | UI-REQ-258     | Error handling in editor        | Server returns error        | 1. Attempt save with server error                                                 | Error message shown; row not updated                          | P2       |

### 4.9 Dark Theme and Accessibility (UI-REQ-300, UI-REQ-301, UI-REQ-350)

| ID           | Requirement(s) | Title                   | Preconditions | Steps                                        | Expected Result                                                                     | Priority |
|--------------|----------------|-------------------------|---------------|----------------------------------------------|-------------------------------------------------------------------------------------|----------|
| TC-UI-300-01 | UI-REQ-300     | Dark theme applied      | App loaded    | 1. Visual inspection                         | All panels use dark background per color spec; text is light                        | P1       |
| TC-UI-301-01 | UI-REQ-301     | Contrast meets WCAG AA  | App loaded    | 1. Test primary text/background combinations | Contrast ratio ≥ 4.5:1 for normal text                                              | P2       |
| TC-UI-350-01 | UI-REQ-350     | Semantic HTML structure | App loaded    | 1. Inspect DOM                               | Buttons use `<button>`, inputs use `<input>`, meaningful `title` attributes present | P3       |

### 4.10 Performance (UI-REQ-320, UI-REQ-321, UI-REQ-412)

| ID           | Requirement(s)         | Title                         | Preconditions    | Steps                                           | Expected Result           | Priority |
|--------------|------------------------|-------------------------------|------------------|-------------------------------------------------|---------------------------|----------|
| TC-UI-320-01 | UI-REQ-320, UI-REQ-412 | Graph renders under 2 seconds | 300-node dataset | 1. Measure time from page load to graph visible | < 2 seconds               | P2       |
| TC-UI-321-01 | UI-REQ-321             | Animations are smooth         | App loaded       | 1. Pan/zoom graph 2. Open/close panels          | 60fps maintained, no jank | P3       |

---

## 5. Integration Tests (End-to-End)

| ID            | Requirement(s)                | Title                                                   | Preconditions                                   | Steps                                                                                                     | Expected Result                                        | Priority |
|---------------|-------------------------------|---------------------------------------------------------|-------------------------------------------------|-----------------------------------------------------------------------------------------------------------|--------------------------------------------------------|----------|
| TC-INT-E2E-01 | REQ-035, REQ-042, REQ-041     | Mitigation upsert → invalidation → badge update         | App loaded; asset visible                       | 1. Select asset 2. Open Mitigations Editor 3. Add mitigation 4. Close modal 5. Check topbar badge         | Badge increments by 1                                  | P1       |
| TC-INT-E2E-02 | REQ-040, REQ-041, UI-REQ-112  | Bulk recalculation → badge reset                        | stale_count > 0                                 | 1. Click Recalculate TTBs 2. Observe toast 3. Check badge                                                 | Toast shows count; badge goes to 0                     | P1       |
| TC-INT-E2E-03 | ALG-REQ-046, UI-REQ-113       | Path with stale members → inline recalc → warning       | At least one path member stale                  | 1. Open Path Inspector 2. Select entry/target with stale member 3. Calculate                              | Results show; orange warning appears; badge decrements | P1       |
| TC-INT-E2E-04 | REQ-036, REQ-042, ALG-REQ-043 | Delete mitigation → invalidation → recalc → TTB changes | Asset with mitigation applied                   | 1. Delete mitigation 2. Note stale_count incremented 3. Recalculate 4. Check asset TTB                    | TTB has changed from original value                    | P1       |
| TC-INT-E2E-05 | ALG-REQ-042, ALG-REQ-047      | Hash changes after mitigation → Merkle root changes     | Known merkle_root M1                            | 1. Note merkle_root 2. Add mitigation 3. Recalculate 4. Check merkle_root                                 | New merkle_root ≠ M1                                   | P1       |
| TC-INT-E2E-06 | ALG-REQ-046                   | Second path calculation — no redundant recalc           | Path members just recalculated in TC-INT-E2E-03 | 1. Run same path calculation again                                                                        | `recalculated_assets: []` (empty); no warning shown    | P1       |
| TC-INT-E2E-07 | REQ-002                       | Config from environment                                 | Env vars set                                    | 1. Start server with NEBULA_HOST, NEBULA_PORT, NEBULA_USER, NEBULA_PASS, NEBULA_SPACE 2. `GET /api/graph` | Connection succeeds; graph data returned               | P1       |

---

## 6. Traceability Matrix

| Requirement | Test Case(s)                                                           |
|-------------|------------------------------------------------------------------------|
| REQ-001     | (no auth by design — verified by any successful test)                  |
| REQ-002     | TC-INT-E2E-07                                                          |
| REQ-010     | TC-UI-200-01                                                           |
| REQ-011     | TC-UI-201-02                                                           |
| REQ-012     | TC-UI-202-01                                                           |
| REQ-013     | TC-UI-201-01                                                           |
| REQ-020     | TC-API-020-01, TC-API-020-02                                           |
| REQ-021     | TC-API-021-01, TC-API-021-02                                           |
| REQ-022     | TC-API-022-01, TC-API-022-02                                           |
| REQ-023     | TC-API-023-01                                                          |
| REQ-024     | TC-API-024-01                                                          |
| REQ-025     | TC-API-022-02, TC-API-026-02, TC-API-PATH-07                           |
| REQ-026     | TC-API-026-01, TC-API-026-02                                           |
| REQ-027     | TC-API-027-01                                                          |
| REQ-028     | (data insertion convention — verified via TC-API-026-01 rank values)   |
| REQ-033     | TC-API-MIT-01                                                          |
| REQ-034     | TC-API-MIT-02                                                          |
| REQ-035     | TC-API-MIT-03, TC-API-MIT-04                                           |
| REQ-036     | TC-API-MIT-05                                                          |
| REQ-038     | TC-API-MIT-06                                                          |
| REQ-039     | TC-API-MIT-07                                                          |
| REQ-040     | TC-API-RECALC-01, TC-API-RECALC-02, TC-API-RECALC-03, TC-API-RECALC-04 |
| REQ-041     | TC-API-STATE-01, TC-API-STATE-02, TC-API-STATE-03                      |
| REQ-042     | TC-API-INVAL-01, TC-API-INVAL-02, TC-API-INVAL-03, TC-API-INVAL-04     |
| ALG-REQ-001 | TC-API-PATH-01                                                         |
| ALG-REQ-002 | TC-API-EP-01                                                           |
| ALG-REQ-003 | TC-API-TGT-01                                                          |
| ALG-REQ-010 | TC-API-PATH-02                                                         |
| ALG-REQ-011 | TC-API-PATH-03                                                         |
| ALG-REQ-030 | TC-API-PATH-06                                                         |
| ALG-REQ-031 | TC-API-PATH-04                                                         |
| ALG-REQ-033 | TC-API-PATH-05                                                         |
| ALG-REQ-042 | TC-DB-042-01, TC-DB-042-02, TC-DB-042-03, TC-DB-042-04                 |
| ALG-REQ-043 | TC-API-INVAL-01, TC-API-INVAL-02, TC-API-INVAL-03                      |
| ALG-REQ-044 | TC-API-RECALC-03                                                       |
| ALG-REQ-045 | TC-API-RECALC-01, TC-API-RECALC-02, TC-INT-E2E-02                      |
| ALG-REQ-046 | TC-DB-046-01, TC-DB-046-02, TC-INT-E2E-03, TC-INT-E2E-06               |
| ALG-REQ-047 | TC-DB-047-01, TC-DB-047-02, TC-DB-047-03, TC-INT-E2E-05                |
| ALG-REQ-048 | TC-API-STATE-01, TC-DB-SYS-01                                          |
| UI-REQ-100  | TC-UI-100-01                                                           |
| UI-REQ-110  | TC-UI-110-01, TC-UI-110-02                                             |
| UI-REQ-112  | TC-UI-112-01 through TC-UI-112-06                                      |
| UI-REQ-113  | TC-UI-113-01 through TC-UI-113-04                                      |
| UI-REQ-120  | TC-UI-120-01                                                           |
| UI-REQ-122  | TC-UI-122-01, TC-UI-122-02                                             |
| UI-REQ-123  | TC-UI-123-01                                                           |
| UI-REQ-124  | TC-UI-124-01                                                           |
| UI-REQ-200  | TC-UI-200-01                                                           |
| UI-REQ-201  | TC-UI-201-01, TC-UI-201-02                                             |
| UI-REQ-202  | TC-UI-202-01                                                           |
| UI-REQ-204  | TC-UI-204-01                                                           |
| UI-REQ-206  | TC-UI-206-01                                                           |
| UI-REQ-207  | TC-UI-207-01, TC-UI-207-02                                             |
| UI-REQ-208  | TC-UI-208-01                                                           |
| UI-REQ-209  | TC-UI-209-01                                                           |
| UI-REQ-210  | TC-UI-210-01, TC-UI-210-02                                             |
| UI-REQ-212  | TC-UI-212-01                                                           |
| UI-REQ-250  | TC-UI-250-01                                                           |
| UI-REQ-252  | TC-UI-252-01                                                           |
| UI-REQ-254  | TC-UI-254-01                                                           |
| UI-REQ-255  | TC-UI-255-01                                                           |
| UI-REQ-257  | TC-UI-257-01                                                           |
| UI-REQ-258  | TC-UI-258-01                                                           |
| UI-REQ-300  | TC-UI-300-01                                                           |
| UI-REQ-301  | TC-UI-301-01                                                           |
| UI-REQ-320  | TC-UI-320-01                                                           |
| UI-REQ-350  | TC-UI-350-01                                                           |

---

## 7. Test Execution Summary Template

| Metric               | Count |
|----------------------|-------|
| Total test cases     | 82    |
| P1 (must-pass)       | 60    |
| P2 (should-pass)     | 17    |
| P3 (nice-to-have)    | 5     |
| APP layer (TC-API)   | 33    |
| GRDB layer (TC-DB)   | 10    |
| VIS layer (TC-UI)    | 32    |
| Integration (TC-INT) | 7     |

---

*Document generated: March 3, 2026*
*Covers: Requirements.md v1.x, AlgoSpecs.md v1.1, UI-Requirements.md v1.12, ESP01_NebulaGraph_Schema.md*
