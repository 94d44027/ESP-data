package nebula

import (
	"fmt"
	"log"
	"math/rand"
	"strings"
	"time"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// AssetRow represents one row from the enriched connectivity query (REQ-020).
// Each row carries both endpoints of a connects_to edge together with
// the properties the front-end needs for colouring, labels, and badges.
type AssetRow struct {
	SrcAssetID          string
	SrcAssetName        string
	SrcIsEntrance       bool
	SrcIsTarget         bool
	SrcPriority         int
	SrcHasVulnerability bool
	SrcAssetType        string

	DstAssetID          string
	DstAssetName        string
	DstIsEntrance       bool
	DstIsTarget         bool
	DstPriority         int
	DstHasVulnerability bool
	DstAssetType        string
}

// NewPool creates and initializes a Nebula ConnectionPool.
// The caller is responsible for calling pool.Close() when done.
// This satisfies REQ-121: use Vesoft's Go client libraries.
func NewPool(cfg *config.Config) *nebula.ConnectionPool {
	hostAddress := nebula.HostAddress{
		Host: cfg.NebulaHost,
		Port: cfg.NebulaPort,
	}

	hostList := []nebula.HostAddress{hostAddress}
	poolConfig := nebula.GetDefaultConf()
	logger := nebula.DefaultLogger{}
	pool, err := nebula.NewConnectionPool(hostList, poolConfig, logger)
	if err != nil {
		log.Fatalf("nebula: failed to create pool: %v", err)
	}

	log.Printf("nebula: pool created for %s:%d", cfg.NebulaHost, cfg.NebulaPort)
	return pool
}

// openSession is a small helper that obtains a session and switches to
// the configured graph space.  Every exported Query* function uses it.
func openSession(pool *nebula.ConnectionPool, cfg *config.Config) (*nebula.Session, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	res, err := session.Execute(useStmt)
	if err != nil {
		session.Release()
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}
	if !res.IsSucceed() {
		session.Release()
		return nil, fmt.Errorf("USE space failed: %s", res.GetErrorMsg())
	}
	return session, nil
}

// safeString extracts a string from a ResultSet value, returning "" on error.
func safeString(record *nebula.Record, idx int) string {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return ""
	}
	s, err := val.AsString()
	if err != nil {
		return ""
	}
	return s
}

// safeBool extracts a bool from a ResultSet value, returning false on error.
func safeBool(record *nebula.Record, idx int) bool {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return false
	}
	b, err := val.AsBool()
	if err != nil {
		return false
	}
	return b
}

// safeInt extracts an int from a ResultSet value, returning the supplied
// default on error (e.g. 4 for priority).
func safeInt(record *nebula.Record, idx int, def int) int {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return def
	}
	n, err := val.AsInt()
	if err != nil {
		return def
	}
	return int(n)
}

// QueryAssets executes the enriched connectivity query specified in REQ-020.
// MATCH is used here because OPTIONAL MATCH with multi-hop property
// retrieval is significantly cleaner than chained GO statements (REQ-244
// justification recorded in Requirements.md REQ-020).
func QueryAssets(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetRow, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-020 query verbatim from requirements
	query := `MATCH (a:Asset)-[e:connects_to]->(b:Asset)
OPTIONAL MATCH (a)-[:has_type]->(at:Asset_Type)
OPTIONAL MATCH (b)-[:has_type]->(bt:Asset_Type)
RETURN
  a.Asset.Asset_ID          AS src_asset_id,
  a.Asset.Asset_Name        AS src_asset_name,
  a.Asset.is_entrance       AS src_is_entrance,
  a.Asset.is_target         AS src_is_target,
  a.Asset.priority          AS src_priority,
  a.Asset.has_vulnerability AS src_has_vulnerability,
  at.Asset_Type.Type_Name   AS src_asset_type,
  b.Asset.Asset_ID          AS dst_asset_id,
  b.Asset.Asset_Name        AS dst_asset_name,
  b.Asset.is_entrance       AS dst_is_entrance,
  b.Asset.is_target         AS dst_is_target,
  b.Asset.priority          AS dst_priority,
  b.Asset.has_vulnerability AS dst_has_vulnerability,
  bt.Asset_Type.Type_Name   AS dst_asset_type
LIMIT 300;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssets executing MATCH query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssets completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	rows := make([]AssetRow, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		rows = append(rows, AssetRow{
			SrcAssetID:          safeString(record, 0),
			SrcAssetName:        safeString(record, 1),
			SrcIsEntrance:       safeBool(record, 2),
			SrcIsTarget:         safeBool(record, 3),
			SrcPriority:         safeInt(record, 4, 4),
			SrcHasVulnerability: safeBool(record, 5),
			SrcAssetType:        safeString(record, 6),
			DstAssetID:          safeString(record, 7),
			DstAssetName:        safeString(record, 8),
			DstIsEntrance:       safeBool(record, 9),
			DstIsTarget:         safeBool(record, 10),
			DstPriority:         safeInt(record, 11, 4),
			DstHasVulnerability: safeBool(record, 12),
			DstAssetType:        safeString(record, 13),
		})
	}

	log.Printf("nebula: QueryAssets returned %d connectivity rows", len(rows))
	return rows, nil
}

// QueryAssetsWithDetails fetches all assets with their type information
// for the sidebar entity browser (REQ-021).
// MATCH is used because OPTIONAL MATCH ensures assets without a has_type
// edge are still returned (REQ-244 justification).
func QueryAssetsWithDetails(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-021 query verbatim from requirements
	query := `MATCH (a:Asset)
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID          AS asset_id,
  a.Asset.Asset_Name        AS asset_name,
  a.Asset.is_entrance       AS is_entrance,
  a.Asset.is_target         AS is_target,
  a.Asset.priority          AS priority,
  a.Asset.has_vulnerability AS has_vulnerability,
  t.Asset_Type.Type_Name    AS asset_type;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetsWithDetails executing MATCH query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetsWithDetails completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	assets := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		assets = append(assets, map[string]interface{}{
			"asset_id":          safeString(record, 0),
			"asset_name":        safeString(record, 1),
			"is_entrance":       safeBool(record, 2),
			"is_target":         safeBool(record, 3),
			"priority":          safeInt(record, 4, 4),
			"has_vulnerability": safeBool(record, 5),
			"asset_type":        safeString(record, 6),
		})
	}

	log.Printf("nebula: QueryAssetsWithDetails returned %d assets", len(assets))
	return assets, nil
}

// QueryAssetDetail fetches detailed information for a single asset (REQ-022).
// MATCH is used because OPTIONAL MATCH for type and segment is significantly
// cleaner than chained GO + FETCH statements (REQ-244 justification).
func QueryAssetDetail(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) (map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-022 query — uses parameterised WHERE on Asset_ID property
	query := fmt.Sprintf(`MATCH (a:Asset) WHERE a.Asset.Asset_ID == "%s"
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
OPTIONAL MATCH (a)-[:belongs_to]->(s:Network_Segment)
OPTIONAL MATCH (a)-[:runs_on]->(os:OS_Type)
RETURN
  a.Asset.Asset_ID            AS asset_id,
  a.Asset.Asset_Name          AS asset_name,
  a.Asset.Asset_Description   AS asset_description,
  a.Asset.Asset_Note          AS asset_note,
  a.Asset.is_entrance         AS is_entrance,
  a.Asset.is_target           AS is_target,
  a.Asset.priority            AS priority,
  a.Asset.has_vulnerability   AS has_vulnerability,
  a.Asset.TTB                 AS ttb,
  t.Asset_Type.Type_Name      AS asset_type,
  s.Network_Segment.Segment_Name AS segment_name,
  os.OS_Type.OS_Name             AS os_name;`, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetDetail executing query for asset %s", queryStart.Format("15:04:05.000"), assetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetDetail completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	if resultSet.GetRowSize() == 0 {
		return nil, fmt.Errorf("asset not found")
	}

	record, err := resultSet.GetRowValuesByIndex(0)
	if err != nil {
		return nil, fmt.Errorf("failed to read result: %w", err)
	}

	detail := map[string]interface{}{
		"asset_id":          safeString(record, 0),
		"asset_name":        safeString(record, 1),
		"asset_description": safeString(record, 2),
		"asset_note":        safeString(record, 3),
		"is_entrance":       safeBool(record, 4),
		"is_target":         safeBool(record, 5),
		"priority":          safeInt(record, 6, 4),
		"has_vulnerability": safeBool(record, 7),
		"ttb":               safeInt(record, 8, 10),
		"asset_type":        safeString(record, 9),
		"segment_name":      safeString(record, 10),
		"os_name":           safeString(record, 11),
	}

	log.Printf("nebula: QueryAssetDetail returned detail for %s", assetID)
	return detail, nil
}

// QueryNeighbors fetches immediate neighbors with direction for the
// inspector panel (REQ-023). Uses pure nGQL with UNION as required
// by REQ-243.
func QueryNeighbors(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-023 query — outbound UNION inbound
	query := fmt.Sprintf(`GO FROM "%s" OVER connects_to
YIELD dst(edge) AS neighbor_id, "outbound" AS direction
UNION
GO FROM "%s" OVER connects_to REVERSELY
YIELD src(edge) AS neighbor_id, "inbound" AS direction;`, assetID, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryNeighbors executing query for asset %s", queryStart.Format("15:04:05.000"), assetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryNeighbors completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	neighbors := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		neighbors = append(neighbors, map[string]interface{}{
			"neighbor_id": safeString(record, 0),
			"direction":   safeString(record, 1),
		})
	}

	log.Printf("nebula: QueryNeighbors returned %d neighbors for %s", len(neighbors), assetID)
	return neighbors, nil
}

// QueryAssetTypes fetches distinct asset types for filter checkboxes (REQ-024).
// Uses pure nGQL LOOKUP per REQ-243.
func QueryAssetTypes(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-024 query verbatim from requirements
	query := `LOOKUP ON Asset_Type
YIELD Asset_Type.Type_ID   AS type_id,
      Asset_Type.Type_Name AS type_name;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetTypes executing query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetTypes completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	types := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		types = append(types, map[string]interface{}{
			"type_id":   safeString(record, 0),
			"type_name": safeString(record, 1),
		})
	}

	log.Printf("nebula: QueryAssetTypes returned %d asset types", len(types))
	return types, nil
}

// QueryEdgeConnections fetches all connects_to edge properties between two
// specific assets, for the edge inspector panel (REQ-026).
// Uses pure nGQL GO statement per REQ-243.
func QueryEdgeConnections(pool *nebula.ConnectionPool, cfg *config.Config, sourceID, targetID string) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-026 query — pure nGQL per REQ-243
	query := fmt.Sprintf(`GO FROM "%s" OVER connects_to
WHERE dst(edge) == "%s"
YIELD
  connects_to.Connection_Protocol AS connection_protocol,
  connects_to.Connection_Port     AS connection_port;`, sourceID, targetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryEdgeConnections executing query for %s -> %s", queryStart.Format("15:04:05.000"), sourceID, targetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryEdgeConnections completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	connections := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		connections = append(connections, map[string]interface{}{
			"connection_protocol": safeString(record, 0),
			"connection_port":     safeString(record, 1),
		})
	}

	log.Printf("nebula: QueryEdgeConnections returned %d connections for %s -> %s", len(connections), sourceID, targetID)
	return connections, nil
}

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

		ttbs, err := extractIntList(record, 1)
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

// ============================================================
// Mitigations queries (REQ-033, REQ-034, REQ-035, REQ-036)
// ============================================================

// QueryMitigations fetches all MITRE mitigations for the editor dropdown (REQ-033).
// Uses pure nGQL LOOKUP per REQ-243.
func QueryMitigations(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-033 query — pure nGQL per REQ-243
	query := `LOOKUP ON tMitreMitigation
YIELD
  id(vertex) AS vid,
  tMitreMitigation.Mitigation_ID AS mitigation_id,
  tMitreMitigation.Mitigation_Name AS mitigation_name;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryMitigations executing LOOKUP query", queryStart.Format("15:04:05.000"))

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryMitigations completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	mitigations := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		mitigations = append(mitigations, map[string]interface{}{
			"mitigation_id":   safeString(record, 1),
			"mitigation_name": safeString(record, 2),
		})
	}

	log.Printf("nebula: QueryMitigations returned %d mitigations", len(mitigations))
	return mitigations, nil
}

// QueryAssetMitigations fetches all mitigations applied to a specific asset (REQ-034).
// MATCH is used because traversing applied_to edge from tMitreMitigation to Asset
// with property retrieval on both the edge and the source vertex is cleaner with
// MATCH than with chained GO/FETCH statements (REQ-244 justification).
func QueryAssetMitigations(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-034 query — MATCH per REQ-244 justification
	query := fmt.Sprintf(`MATCH (m:tMitreMitigation)-[e:applied_to]->(a:Asset)
WHERE a.Asset.Asset_ID == "%s"
RETURN m.tMitreMitigation.Mitigation_ID AS mitigation_id,
  m.tMitreMitigation.Mitigation_Name AS mitigation_name,
  e.Maturity AS maturity,
  e.Active AS active;`, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetMitigations executing MATCH query for asset %s",
		queryStart.Format("15:04:05.000"), assetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetMitigations completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	mitigations := make([]map[string]interface{}, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		mitigations = append(mitigations, map[string]interface{}{
			"mitigation_id":   safeString(record, 0),
			"mitigation_name": safeString(record, 1),
			"maturity":        safeInt(record, 2, 100),
			"active":          safeBool(record, 3),
		})
	}

	log.Printf("nebula: QueryAssetMitigations returned %d mitigations for asset %s", len(mitigations), assetID)
	return mitigations, nil
}

// UpsertMitigation adds or updates an applied_to edge between a mitigation and an asset (REQ-035).
// Uses pure nGQL UPSERT EDGE per REQ-243.
func UpsertMitigation(pool *nebula.ConnectionPool, cfg *config.Config, mitigationID, assetID string, maturity int, active bool) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	// REQ-035 query — pure nGQL per REQ-243
	// @0 rank is fixed per schema ED001
	// Version is hardcoded to "1.0" — version-aware modelling is deferred
	query := fmt.Sprintf(`UPSERT EDGE ON applied_to "%s" -> "%s" @0
SET Version = "1.0", Maturity = %d, Active = %v;`, mitigationID, assetID, maturity, active)

	queryStart := time.Now()
	log.Printf("[%s] nebula: UpsertMitigation executing for %s -> %s (maturity=%d, active=%v)",
		queryStart.Format("15:04:05.000"), mitigationID, assetID, maturity, active)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: UpsertMitigation completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return fmt.Errorf("upsert execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return fmt.Errorf("upsert failed: %s", resultSet.GetErrorMsg())
	}

	return nil
}

// DeleteMitigation removes an applied_to edge between a mitigation and an asset (REQ-036).
// Uses pure nGQL DELETE EDGE per REQ-243.
// Caution: only deletes rank 0 — correct per current design (REQ-035 note).
func DeleteMitigation(pool *nebula.ConnectionPool, cfg *config.Config, mitigationID, assetID string) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	// REQ-036 query — pure nGQL per REQ-243
	query := fmt.Sprintf(`DELETE EDGE applied_to "%s" -> "%s" @0;`, mitigationID, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: DeleteMitigation executing for %s -> %s",
		queryStart.Format("15:04:05.000"), mitigationID, assetID)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: DeleteMitigation completed in %.3f seconds",
		time.Now().Format("15:04:05.000"), queryDuration.Seconds())

	if err != nil {
		return fmt.Errorf("delete execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return fmt.Errorf("delete failed: %s", resultSet.GetErrorMsg())
	}

	return nil
}

// ============================================================
// Hash computation and TTB recalculation (ALG-REQ-042 through ALG-REQ-048)
// ============================================================

// StaleAssetHash holds the result of the hash computation query (ALG-REQ-042).
type StaleAssetHash struct {
	AssetID      string
	CurrentTTB   int
	StoredHash   string
	ComputedHash int64
}

// PathResult holds one discovered path's ordered node IDs and their stored TTBs.
// Returned by QueryPaths (ALG-REQ-001 v1.3).
type PathResult struct {
	IDs  []string // ordered Asset_IDs: [entry, ..intermediates.., target]
	TTBs []int    // stored TTB per node (Regular_chain values)
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

// chainTacticCount returns the number of tactics in a chain.
// Hardcoded for the stub (ALG-REQ-044 v1.3); will be replaced
// by a GrDB query when the real TTB formula is implemented.
var chainTacticCount = map[string]int{
	"CHAIN_ENTRANCE":     8,
	"CHAIN_INTERMEDIATE": 7,
	"CHAIN_TARGET":       10,
}

// ComputeTTBStub calculates a stub TTB value (ALG-REQ-044 v1.3).
// new_TTB = current_TTB + random(1,10) + chain_tactic_count(chainVID)
func ComputeTTBStub(currentTTB int, chainVID string) int {
	count, ok := chainTacticCount[chainVID]
	if !ok {
		count = 7 // default to intermediate
	}
	return currentTTB + rand.Intn(10) + 1 + count
}

// extractStringList extracts a list of strings from a ResultSet column.
func extractStringList(record *nebula.Record, idx int) ([]string, error) {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return nil, err
	}
	list, err := val.AsList()
	if err != nil {
		return nil, err
	}
	result := make([]string, 0, len(list))
	for _, v := range list {
		s, err := v.AsString()
		if err != nil {
			return nil, err
		}
		result = append(result, s)
	}
	return result, nil
}

// extractIntList extracts a list of ints from a ResultSet column.
func extractIntList(record *nebula.Record, idx int) ([]int, error) {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return nil, err
	}
	list, err := val.AsList()
	if err != nil {
		return nil, err
	}
	result := make([]int, 0, len(list))
	for _, v := range list {
		n, err := v.AsInt()
		if err != nil {
			return nil, err
		}
		result = append(result, int(n))
	}
	return result, nil
}

// safeInt64 extracts an int64 from a ResultSet value, returning 0 on error.
func safeInt64(record *nebula.Record, idx int) int64 {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return 0
	}
	n, err := val.AsInt()
	if err != nil {
		return 0
	}
	return n
}

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
OPTIONAL MATCH (a)-[:runs_on]->(os:OS_Type)
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.TTB AS current_ttb,
  a.Asset.hash AS stored_hash,
  hash(concat_ws("##",
    reduce(s = "", x IN conn_parts | s + x + ";"),
    reduce(s = "", x IN mit_parts | s + x + ";"),
    toString(a.Asset.has_vulnerability),
    COALESCE(os.OS_Type.OS_Name, "none"),
    COALESCE(t.Asset_Type.Type_Name, "none")
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
			CurrentTTB:   safeInt(record, 1, 10),
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
OPTIONAL MATCH (a)-[:runs_on]->(os:OS_Type)
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.TTB AS current_ttb,
  a.Asset.hash AS stored_hash,
  hash(concat_ws("##",
    reduce(s = "", x IN conn_parts | s + x + ";"),
    reduce(s = "", x IN mit_parts | s + x + ";"),
    toString(a.Asset.has_vulnerability),
    COALESCE(os.OS_Type.OS_Name, "none"),
    COALESCE(t.Asset_Type.Type_Name, "none")
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
			CurrentTTB:   safeInt(record, 1, 10),
			StoredHash:   safeString(record, 2),
			ComputedHash: safeInt64(record, 3),
		})
	}

	log.Printf("nebula: QueryScopedStaleHashes returned %d stale assets", len(results))
	return results, nil
}

// UpdateAssetTTBAndHash writes the new TTB, hash, and sets hash_valid = true
// for a single asset (ALG-REQ-045 step 2b).
func UpdateAssetTTBAndHash(pool *nebula.ConnectionPool, cfg *config.Config, assetID string, newTTB int, hashStr string) error {
	session, err := openSession(pool, cfg)
	if err != nil {
		return err
	}
	defer session.Release()

	query := fmt.Sprintf(`UPDATE VERTEX ON Asset "%s" SET TTB = %d, hash = "%s", hash_valid = true;`,
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
func QueryAssetHashValidity(pool *nebula.ConnectionPool, cfg *config.Config, assetIDs []string) (map[string]bool, map[string]int, error) {
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
	ttbs := make(map[string]int, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}
		aid := safeString(record, 0)
		validity[aid] = safeBool(record, 1)
		ttbs[aid] = safeInt(record, 2, 10)
	}

	return validity, ttbs, nil
}
