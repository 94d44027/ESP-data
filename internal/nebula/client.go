package nebula

import (
	"fmt"
	"log"
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
  s.Network_Segment.Segment_Name AS segment_name;`, assetID)

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
