package nebula

import (
	"fmt"
	"log"
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

// safeFloat64 extracts a float64 from a ResultSet value.
// NebulaGraph may return integer type for float columns when the value
// has no fractional part (e.g. 120 instead of 120.0), so we try AsFloat
// first, then fall back to AsInt conversion.
func safeFloat64(record *nebula.Record, idx int, def float64) float64 {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return def
	}
	f, err := val.AsFloat()
	if err == nil {
		return f
	}
	n, err := val.AsInt()
	if err == nil {
		return float64(n)
	}
	return def
}

// QueryAssets executes the enriched connectivity query specified in REQ-020.
// MATCH is used here because multi-hop property retrieval across asset
// pairs and their types is significantly cleaner than chained GO statements
// (REQ-244 justification). OPTIONAL MATCH removed per REQ-043 (DI-01).
func QueryAssets(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetRow, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-020 query verbatim from requirements (REQ-043: MATCH for has_type)
	query := `MATCH (a:Asset)-[e:connects_to]->(b:Asset)
MATCH (a)-[:has_type]->(at:Asset_Type)
MATCH (b)-[:has_type]->(bt:Asset_Type)
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
// MATCH is used because multi-hop property retrieval is cleaner than
// chained GO statements (REQ-244 justification). REQ-043: DI-01 guarantees
// every asset has a has_type edge.
func QueryAssetsWithDetails(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-021 query verbatim from requirements (REQ-043: MATCH for has_type)
	query := `MATCH (a:Asset)
MATCH (a)-[:has_type]->(t:Asset_Type)
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
// MATCH is used because type/segment/OS property retrieval is significantly
// cleaner than chained GO + FETCH statements (REQ-244 justification).
// REQ-043: DI-01/02/03 guarantee has_type, belongs_to, runs_on edges.
func QueryAssetDetail(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) (map[string]interface{}, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// REQ-022 query — uses parameterised WHERE on Asset_ID property (REQ-043: MATCH for type/segment/OS)
	query := fmt.Sprintf(`MATCH (a:Asset) WHERE a.Asset.Asset_ID == "%s"
MATCH (a)-[:has_type]->(t:Asset_Type)
MATCH (a)-[:belongs_to]->(s:Network_Segment)
MATCH (a)-[:runs_on]->(os:OS_Type)
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
	CurrentTTB   float64
	StoredHash   string
	ComputedHash int64
}

// PathResult holds one discovered path's ordered node IDs and their stored TTBs.
// Returned by QueryPaths (ALG-REQ-001 v1.3).
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
type TTTResult struct {
	TechniqueID   string  // Technique_ID
	TechniqueName string  // Technique_Name
	ExecMin       float64 // execution_min from TA008
	ExecMax       float64 // execution_max from TA008
	P             int     // count of possible mitigations (mitigates edges)
	A             int     // count of active-applied mitigations on this asset
	SumMaturity   float64 // sum of Maturity * 0.01 for active-applied mitigations
	TTT           float64 // computed Time To Execute Technique (hours)
}

// ComputeTTT computes the Time To Execute a Technique for a given (asset, technique) pair.
// Implements ALG-REQ-060 through ALG-REQ-066.
// Returns (nil, nil) when the technique is not executable on the asset's OS platform (ALG-REQ-062).
func ComputeTTT(pool *nebula.ConnectionPool, cfg *config.Config, assetVid, techniqueVid string) (*TTTResult, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	// ALG-REQ-062: OS platform pre-check
	osCheck := fmt.Sprintf(
		`MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)`+
			`<-[:can_be_executed_on]-(t:tMitreTechnique) `+
			`WHERE id(a) == "%s" AND id(t) == "%s" `+
			`RETURN count(*) AS cnt;`, assetVid, techniqueVid)

	osRS, err := session.Execute(osCheck)
	if err != nil {
		return nil, fmt.Errorf("ComputeTTT os check: %w", err)
	}
	if !osRS.IsSucceed() {
		return nil, fmt.Errorf("ComputeTTT os check: %s", osRS.GetErrorMsg())
	}
	if osRS.GetRowSize() > 0 {
		rec, _ := osRS.GetRowValuesByIndex(0)
		cnt := safeInt(rec, 0, 0)
		if cnt == 0 {
			return nil, nil
		}
	}

	// ALG-REQ-064: TTT query — split into two queries to avoid
	// OPTIONAL MATCH ... WHERE which is not supported in nGQL 3.x.

	// Query 1: Get technique properties and count of possible mitigations (P)
	q1 := fmt.Sprintf(
		`MATCH (t:tMitreTechnique) WHERE id(t) == "%s" `+
			`OPTIONAL MATCH (m:tMitreMitigation)-[:mitigates]->(t) `+
			`WITH t, count(m) AS P, collect(id(m)) AS mitigation_vids `+
			`RETURN t.tMitreTechnique.Technique_ID AS technique_id, `+
			`  t.tMitreTechnique.Technique_Name AS technique_name, `+
			`  t.tMitreTechnique.execution_min AS exec_min, `+
			`  t.tMitreTechnique.execution_max AS exec_max, `+
			`  P AS possible_mitigations, `+
			`  mitigation_vids AS mit_vids;`,
		techniqueVid)

	rs1, err := session.Execute(q1)
	if err != nil {
		return nil, fmt.Errorf("ComputeTTT query1: %w", err)
	}
	if !rs1.IsSucceed() {
		return nil, fmt.Errorf("ComputeTTT query1: %s", rs1.GetErrorMsg())
	}
	if rs1.GetRowSize() == 0 {
		return nil, fmt.Errorf("ComputeTTT: technique %s not found", techniqueVid)
	}

	rec1, _ := rs1.GetRowValuesByIndex(0)
	result := &TTTResult{
		TechniqueID:   safeString(rec1, 0),
		TechniqueName: safeString(rec1, 1),
		ExecMin:       safeFloat64(rec1, 2, 0.1667),
		ExecMax:       safeFloat64(rec1, 3, 120.0),
		P:             safeInt(rec1, 4, 0),
	}

	// Extract mitigation VIDs from the list column
	mitVids, _ := extractStringList(rec1, 5)

	// Query 2: Count active-applied mitigations on this asset from the P set.
	if len(mitVids) > 0 {
		quotedVids := make([]string, len(mitVids))
		for i, v := range mitVids {
			quotedVids[i] = fmt.Sprintf(`"%s"`, v)
		}
		vidListStr := strings.Join(quotedVids, ",")

		q2 := fmt.Sprintf(
			`MATCH (m2:tMitreMitigation)-[ap:applied_to]->(a:Asset) `+
				`WHERE id(a) == "%s" AND id(m2) IN [%s] AND ap.Active == true `+
				`RETURN count(m2) AS A, `+
				`  CASE WHEN count(m2) > 0 THEN sum(ap.Maturity) ELSE 0 END AS maturity_sum;`,
			assetVid, vidListStr)

		rs2, err := session.Execute(q2)
		if err != nil {
			log.Printf("nebula: ComputeTTT query2 failed: %v", err)
		} else if !rs2.IsSucceed() {
			log.Printf("nebula: ComputeTTT query2: %s", rs2.GetErrorMsg())
		} else if rs2.GetRowSize() > 0 {
			rec2, _ := rs2.GetRowValuesByIndex(0)
			result.A = safeInt(rec2, 0, 0)
			result.SumMaturity = safeFloat64(rec2, 1, 0.0)
		}
	}

	// ALG-REQ-060: TTT formula
	if result.P == 0 {
		result.TTT = result.ExecMin
	} else if result.A == result.P {
		result.TTT = result.ExecMax
	} else {
		maturityFactor := result.SumMaturity * 0.01
		pf := float64(result.P)
		result.TTT = result.ExecMin + (result.ExecMax-result.ExecMin)*(maturityFactor/pf)
	}

	return result, nil
}

// ======================================================================================================
// TTB Calculation — ALG-REQ-070 through ALG-REQ-080
// ======================================================================================================

// TTBParams holds configurable parameters for the TTB calculation (ALG-REQ-071, 072, 075).
type TTBParams struct {
	OrientationTime   float64
	SwitchoverTime    float64
	PriorityTolerance int
}

// TTBLogEntry records one step of the tactic chain traversal (ALG-REQ-079).
type TTBLogEntry struct {
	TacticID        string  `json:"tactic_id"`
	TacticName      string  `json:"tactic_name"`
	TechniqueID     *string `json:"technique_id"`
	TechniqueName   *string `json:"technique_name"`
	TTT             float64 `json:"ttt"`
	CandidatesCount int     `json:"candidates_count"`
}

// TTBResult is the output of ComputeTTB (ALG-REQ-070).
type TTBResult struct {
	TTB float64       `json:"ttb"`
	Log []TTBLogEntry `json:"log"`
}

// techniqueCandidate holds one technique row returned by the selection queries.
type techniqueCandidate struct {
	TechniqueID    string
	TechniqueName  string
	Priority       int
	VulnApplicable bool
	TTT            float64
}

// getOrderedTactics returns the tactic VIDs for a chain, ordered by chain_includes rank.
func getOrderedTactics(session *nebula.Session, chainVid string) ([]struct{ VID, TacticID, TacticName string }, error) {
	query := fmt.Sprintf(
		`GO FROM "%s" OVER chain_includes `+
			`YIELD chain_includes._rank AS rank, id($$) AS tactic_vid `+
			`| ORDER BY $-.rank ASC;`, chainVid)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("getOrderedTactics GO: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("getOrderedTactics GO: %s", rs.GetErrorMsg())
	}

	type tacticRef struct{ VID, TacticID, TacticName string }
	var tactics []tacticRef

	for i := 0; i < rs.GetRowSize(); i++ {
		record, _ := rs.GetRowValuesByIndex(i)
		vid := safeString(record, 1)
		if vid == "" {
			continue
		}
		tactics = append(tactics, tacticRef{VID: vid})
	}

	if len(tactics) == 0 {
		return nil, fmt.Errorf("getOrderedTactics: chain %s has no tactics", chainVid)
	}

	var vids []string
	for _, t := range tactics {
		vids = append(vids, fmt.Sprintf(`"%s"`, t.VID))
	}
	fetchQ := fmt.Sprintf(
		`FETCH PROP ON tMitreTactic %s `+
			`YIELD tMitreTactic.Tactic_ID AS tid, tMitreTactic.Tactic_Name AS tname;`,
		strings.Join(vids, ","))

	fs, err := session.Execute(fetchQ)
	if err != nil {
		return nil, fmt.Errorf("getOrderedTactics FETCH: %w", err)
	}
	if !fs.IsSucceed() {
		return nil, fmt.Errorf("getOrderedTactics FETCH: %s", fs.GetErrorMsg())
	}

	info := make(map[string]tacticRef)
	for i := 0; i < fs.GetRowSize(); i++ {
		record, _ := fs.GetRowValuesByIndex(i)
		vid := safeString(record, 0)
		info[vid] = tacticRef{
			VID:        vid,
			TacticID:   safeString(record, 1),
			TacticName: safeString(record, 2),
		}
	}

	result := make([]struct{ VID, TacticID, TacticName string }, 0, len(tactics))
	for _, t := range tactics {
		if inf, ok := info[t.VID]; ok {
			result = append(result, struct{ VID, TacticID, TacticName string }{
				VID: inf.VID, TacticID: inf.TacticID, TacticName: inf.TacticName,
			})
		} else {
			result = append(result, struct{ VID, TacticID, TacticName string }{
				VID: t.VID, TacticID: t.VID, TacticName: t.VID,
			})
		}
	}
	return result, nil
}

// selectFirstTacticTechniques implements ALG-REQ-073.
func selectFirstTacticTechniques(session *nebula.Session, assetVid, tacticVid string) ([]techniqueCandidate, error) {
	query := fmt.Sprintf(
		`MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)`+
			`<-[:can_be_executed_on]-(t:tMitreTechnique)-[:part_of]->(tac:tMitreTactic) `+
			`WHERE id(a) == "%s" AND id(tac) == "%s" `+
			`WITH collect({ `+
			`  tid: t.tMitreTechnique.Technique_ID, `+
			`  tname: t.tMitreTechnique.Technique_Name, `+
			`  pri: t.tMitreTechnique.priority, `+
			`  rcelpe: t.tMitreTechnique.rcelpe `+
			`}) AS rows `+
			`UNWIND rows AS r `+
			`RETURN DISTINCT r.tid AS technique_id, `+
			`       r.tname AS technique_name, `+
			`       r.pri AS technique_priority, `+
			`       r.rcelpe AS vuln_applicable `+
			`ORDER BY technique_priority DESC, technique_id;`,
		assetVid, tacticVid)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("selectFirstTacticTechniques: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("selectFirstTacticTechniques: %s", rs.GetErrorMsg())
	}

	return parseTechniqueCandidates(rs)
}

// selectPatternTechniques implements ALG-REQ-076.
func selectPatternTechniques(session *nebula.Session, previousTacticID, fastestTechniqueID, currentTacticID string) ([]techniqueCandidate, error) {
	stateID := previousTacticID + "|" + fastestTechniqueID
	query := fmt.Sprintf(
		`MATCH (src_state:tMitreState)-[:patterns_to]->(dst_state:tMitreState) `+
			`WHERE id(src_state) == "%s" `+
			`WITH dst_state.tMitreState.state_id AS dst_id `+
			`WITH dst_id, split(dst_id, "|") AS parts `+
			`WHERE size(parts) == 2 AND parts[0] == "%s" `+
			`WITH parts[1] AS technique_vid `+
			`MATCH (t:tMitreTechnique) `+
			`WHERE id(t) == technique_vid `+
			`RETURN t.tMitreTechnique.Technique_ID AS technique_id, `+
			`       t.tMitreTechnique.Technique_Name AS technique_name, `+
			`       t.tMitreTechnique.priority AS technique_priority, `+
			`       t.tMitreTechnique.rcelpe AS vuln_applicable `+
			`ORDER BY technique_priority DESC, technique_id;`,
		stateID, currentTacticID)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("selectPatternTechniques: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("selectPatternTechniques: %s", rs.GetErrorMsg())
	}

	return parseTechniqueCandidates(rs)
}

// filterByOS applies ALG-REQ-062 OS platform filter to pattern-derived candidates.
func filterByOS(session *nebula.Session, candidates []techniqueCandidate, assetVid string) ([]techniqueCandidate, error) {
	if len(candidates) == 0 {
		return candidates, nil
	}

	query := fmt.Sprintf(
		`MATCH (a:Asset)-[:runs_on]->(os:OS_Type)-[:represents]->(p:MitrePlatform)`+
			`<-[:can_be_executed_on]-(t:tMitreTechnique) `+
			`WHERE id(a) == "%s" `+
			`RETURN DISTINCT t.tMitreTechnique.Technique_ID AS technique_id;`, assetVid)

	rs, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("filterByOS: %w", err)
	}
	if !rs.IsSucceed() {
		return nil, fmt.Errorf("filterByOS: %s", rs.GetErrorMsg())
	}

	allowed := make(map[string]bool)
	for i := 0; i < rs.GetRowSize(); i++ {
		record, _ := rs.GetRowValuesByIndex(i)
		allowed[safeString(record, 0)] = true
	}

	var filtered []techniqueCandidate
	for _, c := range candidates {
		if allowed[c.TechniqueID] {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

// filterByVulnerability implements ALG-REQ-074.
func filterByVulnerability(candidates []techniqueCandidate, hasVulnerability bool) []techniqueCandidate {
	if !hasVulnerability {
		return candidates
	}
	var vulnCandidates []techniqueCandidate
	for _, c := range candidates {
		if c.VulnApplicable {
			vulnCandidates = append(vulnCandidates, c)
		}
	}
	if len(vulnCandidates) > 0 {
		return vulnCandidates
	}
	return candidates
}

// filterByPriority implements ALG-REQ-075.
func filterByPriority(candidates []techniqueCandidate, tolerance int) []techniqueCandidate {
	if len(candidates) == 0 {
		return candidates
	}
	maxPri := 0
	for _, c := range candidates {
		if c.Priority > maxPri {
			maxPri = c.Priority
		}
	}
	threshold := maxPri - tolerance
	if threshold < 1 {
		threshold = 1
	}
	var filtered []techniqueCandidate
	for _, c := range candidates {
		if c.Priority >= threshold {
			filtered = append(filtered, c)
		}
	}
	return filtered
}

// selectFastest implements ALG-REQ-077 (argmin TTT with deterministic tie-breaking).
func selectFastest(candidates []techniqueCandidate) *techniqueCandidate {
	if len(candidates) == 0 {
		return nil
	}
	best := &candidates[0]
	for i := 1; i < len(candidates); i++ {
		c := &candidates[i]
		if c.TTT < best.TTT {
			best = c
		} else if c.TTT == best.TTT {
			if c.Priority > best.Priority {
				best = c
			} else if c.Priority == best.Priority && c.TechniqueID < best.TechniqueID {
				best = c
			}
		}
	}
	return best
}

// parseTechniqueCandidates converts a ResultSet into techniqueCandidate slices.
func parseTechniqueCandidates(rs *nebula.ResultSet) ([]techniqueCandidate, error) {
	var candidates []techniqueCandidate
	for i := 0; i < rs.GetRowSize(); i++ {
		record, _ := rs.GetRowValuesByIndex(i)
		tid := safeString(record, 0)
		if tid == "" {
			continue
		}
		candidates = append(candidates, techniqueCandidate{
			TechniqueID:    tid,
			TechniqueName:  safeString(record, 1),
			Priority:       safeInt(record, 2, 4),
			VulnApplicable: safeBool(record, 3),
		})
	}
	return candidates, nil
}

// queryAssetHasVulnerability fetches the has_vulnerability flag for an asset.
func queryAssetHasVulnerability(session *nebula.Session, assetVid string) (bool, error) {
	query := fmt.Sprintf(`FETCH PROP ON Asset "%s" YIELD Asset.has_vulnerability AS hv;`, assetVid)
	rs, err := session.Execute(query)
	if err != nil {
		return false, fmt.Errorf("queryAssetHasVulnerability: %w", err)
	}
	if !rs.IsSucceed() {
		return false, fmt.Errorf("queryAssetHasVulnerability: %s", rs.GetErrorMsg())
	}
	if rs.GetRowSize() == 0 {
		return false, nil
	}
	record, _ := rs.GetRowValuesByIndex(0)
	return safeBool(record, 1), nil
}

// ComputeTTB implements the full TTB calculation algorithm (ALG-REQ-070).
func ComputeTTB(pool *nebula.ConnectionPool, cfg *config.Config, assetVid, chainVid string, params TTBParams) (*TTBResult, error) {
	session, err := openSession(pool, cfg)
	if err != nil {
		return nil, err
	}
	defer session.Release()

	tactics, err := getOrderedTactics(session, chainVid)
	if err != nil {
		return nil, fmt.Errorf("ComputeTTB: %w", err)
	}

	hasVuln, err := queryAssetHasVulnerability(session, assetVid)
	if err != nil {
		log.Printf("nebula: ComputeTTB warning — could not fetch has_vulnerability for %s: %v", assetVid, err)
	}

	ttb := params.OrientationTime
	var ttbLog []TTBLogEntry
	var previousTacticID string
	var fastestTechID *string
	techniqueCount := 0

	for i, tactic := range tactics {
		var candidates []techniqueCandidate
		usedFallback := false

		if i == 0 || fastestTechID == nil {
			candidates, err = selectFirstTacticTechniques(session, assetVid, tactic.VID)
			if err != nil {
				log.Printf("nebula: ComputeTTB selectFirstTacticTechniques failed for tactic %s: %v", tactic.TacticID, err)
				candidates = nil
			}
			if i > 0 {
				usedFallback = true
			}
		} else {
			candidates, err = selectPatternTechniques(session, previousTacticID, *fastestTechID, tactic.TacticID)
			if err != nil {
				log.Printf("nebula: ComputeTTB selectPatternTechniques failed for %s|%s -> %s: %v",
					previousTacticID, *fastestTechID, tactic.TacticID, err)
				candidates = nil
			}

			if len(candidates) > 0 {
				candidates, err = filterByOS(session, candidates, assetVid)
				if err != nil {
					log.Printf("nebula: ComputeTTB filterByOS failed: %v", err)
				}
			}

			if len(candidates) == 0 && !usedFallback {
				candidates, err = selectFirstTacticTechniques(session, assetVid, tactic.VID)
				if err != nil {
					log.Printf("nebula: ComputeTTB fallback selectFirstTacticTechniques failed for tactic %s: %v", tactic.TacticID, err)
					candidates = nil
				}
				usedFallback = true
			}
		}

		candidates = filterByVulnerability(candidates, hasVuln)
		candidates = filterByPriority(candidates, params.PriorityTolerance)
		candidatesCount := len(candidates)

		if candidatesCount == 0 {
			log.Printf("nebula: ComputeTTB — empty technique set for tactic %s (%s) on asset %s",
				tactic.TacticID, tactic.TacticName, assetVid)
			ttbLog = append(ttbLog, TTBLogEntry{
				TacticID:        tactic.TacticID,
				TacticName:      tactic.TacticName,
				TechniqueID:     nil,
				TechniqueName:   nil,
				TTT:             0.0,
				CandidatesCount: 0,
			})
			previousTacticID = tactic.TacticID
			fastestTechID = nil
			continue
		}

		for j := range candidates {
			tttResult, err := ComputeTTT(pool, cfg, assetVid, candidates[j].TechniqueID)
			if err != nil {
				log.Printf("nebula: ComputeTTB ComputeTTT failed for %s: %v", candidates[j].TechniqueID, err)
				continue
			}
			if tttResult == nil {
				candidates[j].TTT = 999999.0
			} else {
				candidates[j].TTT = tttResult.TTT
			}
		}

		fastest := selectFastest(candidates)
		if fastest == nil {
			ttbLog = append(ttbLog, TTBLogEntry{
				TacticID:        tactic.TacticID,
				TacticName:      tactic.TacticName,
				TechniqueID:     nil,
				TechniqueName:   nil,
				TTT:             0.0,
				CandidatesCount: candidatesCount,
			})
			previousTacticID = tactic.TacticID
			fastestTechID = nil
			continue
		}

		if techniqueCount > 0 {
			ttb += params.SwitchoverTime
		}
		ttb += fastest.TTT
		techniqueCount++

		tid := fastest.TechniqueID
		tname := fastest.TechniqueName
		ttbLog = append(ttbLog, TTBLogEntry{
			TacticID:        tactic.TacticID,
			TacticName:      tactic.TacticName,
			TechniqueID:     &tid,
			TechniqueName:   &tname,
			TTT:             fastest.TTT,
			CandidatesCount: candidatesCount,
		})

		previousTacticID = tactic.TacticID
		fastestTechID = &tid
	}

	return &TTBResult{TTB: ttb, Log: ttbLog}, nil
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

func extractFloatList(record *nebula.Record, idx int) ([]float64, error) {
	val, err := record.GetValueByIndex(idx)
	if err != nil {
		return nil, err
	}
	list, err := val.AsList()
	if err != nil {
		return nil, err
	}
	result := make([]float64, 0, len(list))
	for _, v := range list {
		// NebulaGraph may return int or float depending on the stored type
		if f, err := v.AsFloat(); err == nil {
			result = append(result, f)
		} else if n, err := v.AsInt(); err == nil {
			result = append(result, float64(n))
		} else {
			return nil, fmt.Errorf("cannot convert list element to float64")
		}
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
