package nebula

import (
	"fmt"
	"log"
	"time"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

type PathResult struct {
	IDs  []string  // ordered Asset_IDs: [entry, ..intermediates.., target]
	TTBs []float64 // stored TTB per node (hours, float64 since v1.5)
}

// ChainVIDForPosition returns the TacticChain vertex ID for a node's
// position in the attack path (ALG-REQ-051).
func ChainVIDForPosition(index, pathLength int) string {
	switch {
	case index == 0:
		return "CHAIN_ENTRANCE"
	case index == pathLength-1:
		return "CHAIN_TARGET"
	default:
		return "CHAIN_INTERMEDIATE"
	}
}

// TTTResult holds the output of a single technique's TTT computation (ALG-REQ-065).

// ======================================================================================================
// Path Inspector queries (ALG-REQ-001, ALG-REQ-002, ALG-REQ-003; migrated from REQ-029–031)
// ======================================================================================================

// QueryEntryPoints fetches all assets where is_entrance == true (ALG-REQ-002, migrated from REQ-030).
// Uses pure nGQL LOOKUP per REQ-243.
func QueryEntryPoints(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	query := `LOOKUP ON Asset WHERE Asset.is_entrance == true
YIELD id(vertex) AS vid, Asset.Asset_ID AS asset_id, Asset.Asset_Name AS asset_name;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryEntryPoints executing query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryEntryPoints completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	entries := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		entries = append(entries, map[string]interface{}{
			"asset_id":   safeString(record, 1),
			"asset_name": safeString(record, 2),
		})
	}

	log.Printf("nebula: QueryEntryPoints returned %d entry points", len(entries))
	return entries, nil
}

// QueryTargets fetches all assets where is_target == true (ALG-REQ-003, migrated from REQ-031).
// Uses pure nGQL LOOKUP per REQ-243.
func QueryTargets(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	query := `LOOKUP ON Asset WHERE Asset.is_target == true
YIELD id(vertex) AS vid, Asset.Asset_ID AS asset_id, Asset.Asset_Name AS asset_name;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryTargets executing query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryTargets completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	targets := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		targets = append(targets, map[string]interface{}{
			"asset_id":   safeString(record, 1),
			"asset_name": safeString(record, 2),
		})
	}

	log.Printf("nebula: QueryTargets returned %d targets", len(targets))
	return targets, nil
}

// QueryPaths executes the path discovery query (ALG-REQ-001 v1.3).
// Returns per-path ordered ID lists and stored TTB values.
// The APP layer builds host strings and computes position-aware TTA.
func QueryPaths(pool *nebula.ConnectionPool, cfg *config.Config, entryID, targetID string, maxHops int) ([]PathResult, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	query := fmt.Sprintf(`MATCH p = (a:Asset)-[e:connects_to*..%d]->(b:Asset)
WHERE a.Asset.Asset_ID == "%s" AND b.Asset.Asset_ID == "%s"
  AND ALL(n IN nodes(p) WHERE single(m IN nodes(p) WHERE m == n))
WITH nodes(p) AS pathNodes
WITH [n IN pathNodes | n.Asset.Asset_ID] AS ids,
     [n IN pathNodes | COALESCE(n.Asset.TTB, 10)] AS ttbs
RETURN ids, ttbs;`, maxHops, entryID, targetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryPaths executing MATCH query (%s -> %s, max %d hops)",
		queryStart.Format("15:04:05.000"), entryID, targetID, maxHops)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryPaths completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	paths := make([]PathResult, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		ids, err := extractStringList(record, 0)
		if err != nil {
			log.Printf("nebula: skipping row %d ids: %v", i, err)
			continue
		}

		ttbs, err := extractFloatList(record, 1)
		if err != nil {
			log.Printf("nebula: skipping row %d ttbs: %v", i, err)
			continue
		}

		paths = append(paths, PathResult{IDs: ids, TTBs: ttbs})
	}

	log.Printf("nebula: QueryPaths returned %d paths for %s -> %s", len(paths), entryID, targetID)
	return paths, nil
}

// QueryAssetTTB fetches the TTB value for a single asset by Asset_ID.
// Used by the path calculator to subtract the entry point's TTB (ALG-REQ-010, migrated from REQ-032).
// Uses pure nGQL LOOKUP per REQ-243.
func QueryAssetTTB(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) (int, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return 0, err
	}
	defer session.Release()

	query := fmt.Sprintf(`LOOKUP ON Asset WHERE Asset.Asset_ID == "%s"
YIELD Asset.TTB AS ttb;`, assetID)

	resultSet, err := session.Execute(query)
	if err != nil {
		return 0, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return 0, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	if resultSet.GetRowSize() == 0 {
		return 10, nil // default TTB per schema TA001
	}

	record, err := resultSet.GetRowValuesByIndex(0)
	if err != nil {
		return 10, nil
	}

	return safeInt(record, 0, 10), nil
}

