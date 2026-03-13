package nebula

import (
	"fmt"
	"log"
	"strings"
	"time"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// ============================================================
// Hash computation and TTB recalculation (ALG-REQ-042 through ALG-REQ-048)
// ============================================================

// StaleAssetHash holds the result of the hash computation query (ALG-REQ-042).
type StaleAssetHash struct {
	AssetID      string
	CurrentTTB   float64
	StoredHash   string
	ComputedHash int64
}

// PathResult holds one discovered path's ordered node IDs and their stored TTBs.
// Returned by QueryPaths (ALG-REQ-001 v1.3).

// QueryStaleHashes executes the hash computation query (ALG-REQ-042) for all
// assets with hash_valid == false. Hash is computed entirely in the database
// using hash() + concat_ws() + collect() + reduce() to minimise data transfer.
func QueryStaleHashes(pool *nebula.ConnectionPool, cfg *config.Config) ([]StaleAssetHash, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	query := `MATCH (a:Asset)
WHERE a.Asset.hash_valid == false
OPTIONAL MATCH (src:Asset)-[c:connects_to]->(a)
WITH a, src, c,
  src.Asset.Asset_ID AS src_id,
  c.Connection_Protocol AS c_proto,
  c.Connection_Port AS c_port
ORDER BY src_id, c_proto, c_port
WITH a, collect(concat_ws("|", src_id, c_proto, c_port)) AS conn_parts
OPTIONAL MATCH (m:tMitreMitigation)-[e:applied_to]->(a)
WITH a, conn_parts, m, e,
  m.tMitreMitigation.Mitigation_ID AS mit_id
ORDER BY mit_id
WITH a, conn_parts,
  collect(concat_ws("|", mit_id, toString(e.Maturity), toString(e.Active))) AS mit_parts
MATCH (a)-[:runs_on]->(os:OS_Type)
MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.TTB AS current_ttb,
  a.Asset.hash AS stored_hash,
  hash(concat_ws("##",
    reduce(s = "", x IN conn_parts | s + x + ";"),
    reduce(s = "", x IN mit_parts | s + x + ";"),
    toString(a.Asset.has_vulnerability),
    os.OS_Type.OS_Name,
    t.Asset_Type.Type_Name
  )) AS computed_hash;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryStaleHashes executing hash computation query",
		queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryStaleHashes completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	results := make([]StaleAssetHash, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}
		results = append(results, StaleAssetHash{
			AssetID:      safeString(record, 0),
			CurrentTTB:   safeFloat64(record, 1, 10),
			StoredHash:   safeString(record, 2),
			ComputedHash: safeInt64(record, 3),
		})
	}

	log.Printf("nebula: QueryStaleHashes returned %d stale assets", len(results))
	return results, nil
}

// QueryScopedStaleHashes runs the same hash computation as QueryStaleHashes
// but scoped to a specific set of asset IDs (ALG-REQ-046 step 4).
func QueryScopedStaleHashes(pool *nebula.ConnectionPool, cfg *config.Config, assetIDs []string) ([]StaleAssetHash, error) {
	if len(assetIDs) == 0 {
		return nil, nil
	}

	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	quoted := make([]string, len(assetIDs))
	for i, id := range assetIDs {
		quoted[i] = fmt.Sprintf(`"%s"`, id)
	}
	inList := strings.Join(quoted, ", ")

	query := fmt.Sprintf(`MATCH (a:Asset)
WHERE a.Asset.Asset_ID IN [%s] AND a.Asset.hash_valid == false
OPTIONAL MATCH (src:Asset)-[c:connects_to]->(a)
WITH a, src, c,
  src.Asset.Asset_ID AS src_id,
  c.Connection_Protocol AS c_proto,
  c.Connection_Port AS c_port
ORDER BY src_id, c_proto, c_port
WITH a, collect(concat_ws("|", src_id, c_proto, c_port)) AS conn_parts
OPTIONAL MATCH (m:tMitreMitigation)-[e:applied_to]->(a)
WITH a, conn_parts, m, e,
  m.tMitreMitigation.Mitigation_ID AS mit_id
ORDER BY mit_id
WITH a, conn_parts,
  collect(concat_ws("|", mit_id, toString(e.Maturity), toString(e.Active))) AS mit_parts
MATCH (a)-[:runs_on]->(os:OS_Type)
MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.TTB AS current_ttb,
  a.Asset.hash AS stored_hash,
  hash(concat_ws("##",
    reduce(s = "", x IN conn_parts | s + x + ";"),
    reduce(s = "", x IN mit_parts | s + x + ";"),
    toString(a.Asset.has_vulnerability),
    os.OS_Type.OS_Name,
    t.Asset_Type.Type_Name
  )) AS computed_hash;`, inList)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryScopedStaleHashes executing for %d assets",
		queryStart.Format("15:04:05.000"), len(assetIDs))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryScopedStaleHashes completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	results := make([]StaleAssetHash, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}
		results = append(results, StaleAssetHash{
			AssetID:      safeString(record, 0),
			CurrentTTB:   safeFloat64(record, 1, 10),
			StoredHash:   safeString(record, 2),
			ComputedHash: safeInt64(record, 3),
		})
	}

	log.Printf("nebula: QueryScopedStaleHashes returned %d stale assets", len(results))
	return results, nil
}

// UpdateAssetTTBAndHash writes the new TTB, hash, and sets hash_valid = true
// for a single asset (ALG-REQ-045 step 2b).
func UpdateAssetTTBAndHash(pool *nebula.ConnectionPool, cfg *config.Config, assetID string, newTTB float64, hashStr string) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	query := fmt.Sprintf(`UPDATE VERTEX ON Asset "%s" SET TTB = %f, hash = "%s", hash_valid = true;`,
		assetID, newTTB, hashStr)

	resultSet, err := session.Execute(query)
	if err != nil {
		return fmt.Errorf("update execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return fmt.Errorf("update failed: %s", resultSet.GetErrorMsg())
	}
	return nil
}

// DecrementStaleCount decrements stale_count on SystemState by the given amount
// after path-scoped recalculation (ALG-REQ-046 cleanup, UI-REQ-112A).
// Uses a WHEN guard to avoid negative values. Best-effort — errors are logged
// but do not propagate to the caller.
func DecrementStaleCount(pool *nebula.ConnectionPool, cfg *config.Config, count int) {
	if count <= 0 {
		return
	}
	session, err := openSession(pool, cfg)
	if err != nil {
		log.Printf("nebula: DecrementStaleCount failed to open session: %v", err)
		return
	}
	defer session.Release()

	// Decrement only when stale_count is large enough (CAS guard).
	query := fmt.Sprintf(
		`UPDATE VERTEX ON SystemState "SYS001" `+
			`SET stale_count = stale_count - %d `+
			`WHEN stale_count >= %d;`,
		count, count)

	resultSet, err := session.Execute(query)
	if err != nil {
		log.Printf("nebula: DecrementStaleCount failed: %v", err)
		return
	}
	if !resultSet.IsSucceed() {
		// WHEN condition was false (stale_count < count) — floor to 0.
		query2 := `UPDATE VERTEX ON SystemState "SYS001" SET stale_count = 0 WHEN stale_count > 0;`
		if rs2, err2 := session.Execute(query2); err2 != nil {
			log.Printf("nebula: DecrementStaleCount fallback failed: %v", err2)
		} else if !rs2.IsSucceed() {
			log.Printf("nebula: DecrementStaleCount fallback failed: %s", rs2.GetErrorMsg())
		}
		return
	}
	log.Printf("nebula: decremented stale_count by %d", count)
}

// InvalidateAssetHash sets hash_valid = false on an asset and increments
// stale_count on SystemState (ALG-REQ-043). Best-effort — errors are logged
// but do not propagate to the caller.
func InvalidateAssetHash(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) {
	session, err := openSession(pool, cfg)
	if err != nil {
		log.Printf("nebula: InvalidateAssetHash failed to open session: %v", err)
		return
	}
	defer session.Release()

	query := fmt.Sprintf(`UPDATE VERTEX ON Asset "%s" SET hash_valid = false;`, assetID)
	resultSet, err := session.Execute(query)
	if err != nil {
		log.Printf("nebula: InvalidateAssetHash (asset) failed for %s: %v", assetID, err)
		return
	}
	if !resultSet.IsSucceed() {
		log.Printf("nebula: InvalidateAssetHash (asset) failed for %s: %s", assetID, resultSet.GetErrorMsg())
		return
	}

	query2 := `UPDATE VERTEX ON SystemState "SYS001" SET stale_count = stale_count + 1;`
	resultSet2, err := session.Execute(query2)
	if err != nil {
		log.Printf("nebula: InvalidateAssetHash (SystemState) failed: %v", err)
		return
	}
	if !resultSet2.IsSucceed() {
		log.Printf("nebula: InvalidateAssetHash (SystemState) failed: %s", resultSet2.GetErrorMsg())
		return
	}

	log.Printf("nebula: invalidated hash for asset %s", assetID)
}

// QuerySystemState fetches the SystemState vertex (ALG-REQ-048).
func QuerySystemState(pool *nebula.ConnectionPool, cfg *config.Config) (map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	query := `FETCH PROP ON SystemState "SYS001"
YIELD SystemState.state_id AS state_id,
      SystemState.merkle_root AS merkle_root,
      SystemState.last_recalc_time AS last_recalc_time,
      SystemState.total_assets AS total_assets,
      SystemState.stale_count AS stale_count;`

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}
	if resultSet.GetRowSize() == 0 {
		return nil, fmt.Errorf("SystemState SYS001 not found")
	}

	record, err := resultSet.GetRowValuesByIndex(0)
	if err != nil {
		return nil, fmt.Errorf("failed to read result: %w", err)
	}

	return map[string]interface{}{
		"state_id":         safeString(record, 0),
		"merkle_root":      safeInt64(record, 1),
		"last_recalc_time": safeString(record, 2),
		"total_assets":     safeInt(record, 3, 0),
		"stale_count":      safeInt(record, 4, 0),
	}, nil
}

// UpdateSystemState writes the updated Merkle root and resets stale_count
// after bulk recalculation (ALG-REQ-045 step 3).
func UpdateSystemState(pool *nebula.ConnectionPool, cfg *config.Config, merkleRoot int64, totalAssets int) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	query := fmt.Sprintf(`UPDATE VERTEX ON SystemState "SYS001"
SET merkle_root = %d,
    last_recalc_time = datetime(),
    total_assets = %d,
    stale_count = 0;`, merkleRoot, totalAssets)

	resultSet, err := session.Execute(query)
	if err != nil {
		return fmt.Errorf("update execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return fmt.Errorf("update failed: %s", resultSet.GetErrorMsg())
	}
	return nil
}

// ComputeMerkleRoot computes the hash-of-hashes entirely in the database
// (ALG-REQ-047). Returns the Merkle root and the total asset count.
// All hashing is done by NebulaGraph's built-in hash() function —
// no hashing libraries needed in the APP layer.
func ComputeMerkleRoot(pool *nebula.ConnectionPool, cfg *config.Config) (int64, int, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return 0, 0, err
	}
	defer session.Release()

	query := `LOOKUP ON Asset
YIELD Asset.Asset_ID AS asset_id, Asset.hash AS hash
| ORDER BY $-.asset_id
| YIELD collect($-.hash) AS all_hashes, count(*) AS total
| YIELD hash(reduce(s = "", x IN $-.all_hashes | s + x + ";")) AS merkle_root,
         $-.total AS total;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: ComputeMerkleRoot executing",
		queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: ComputeMerkleRoot completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return 0, 0, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return 0, 0, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}
	if resultSet.GetRowSize() == 0 {
		return 0, 0, nil
	}

	record, err := resultSet.GetRowValuesByIndex(0)
	if err != nil {
		return 0, 0, fmt.Errorf("failed to read result: %w", err)
	}

	merkleRoot := safeInt64(record, 0)
	total := safeInt(record, 1, 0)

	log.Printf("nebula: ComputeMerkleRoot = %d (total assets: %d)", merkleRoot, total)
	return merkleRoot, total, nil
}

// QueryAssetHashValidity fetches hash_valid and TTB for a specific set of
// asset IDs (ALG-REQ-046 step 3). Uses FETCH PROP for direct VID lookup.
func QueryAssetHashValidity(pool *nebula.ConnectionPool, cfg *config.Config, assetIDs []string) (map[string]bool, map[string]float64, error) {
	if len(assetIDs) == 0 {
		return nil, nil, nil
	}

	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, nil, err
	}
	defer session.Release()

	quoted := make([]string, len(assetIDs))
	for i, id := range assetIDs {
		quoted[i] = fmt.Sprintf(`"%s"`, id)
	}
	vidList := strings.Join(quoted, ", ")

	query := fmt.Sprintf(`FETCH PROP ON Asset %s
YIELD Asset.Asset_ID AS asset_id,
      Asset.hash_valid AS hash_valid,
      Asset.TTB AS ttb;`, vidList)

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	validity := make(map[string]bool, resultSet.GetRowSize())
	ttbs := make(map[string]float64, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}
		aid := safeString(record, 0)
		validity[aid] = safeBool(record, 1)
		ttbs[aid] = safeFloat64(record, 2, 10)
	}

	return validity, ttbs, nil
}
