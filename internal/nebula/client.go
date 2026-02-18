package nebula

import (
	"fmt"
	"log"
	"time"

	"ESP-data/config"

	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// AssetRow represents one row from the connectivity query
type AssetRow struct {
	SrcAssetID string
	DstAssetID string
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

// QueryAssets executes the connectivity query specified in REQ-020.
// Returns rows with source and destination Asset_IDs.
func QueryAssets(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetRow, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	useResult, err := session.Execute(useStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}
	if !useResult.IsSucceed() {
		return nil, fmt.Errorf("USE space failed: %s", useResult.GetErrorMsg())
	}

	// Native nGQL: LOOKUP all Assets and pipe to GO for connectivity (tested in console)
	query := `LOOKUP ON Asset YIELD id(vertex) AS vid | GO FROM $-.vid OVER connects_to YIELD $^.Asset.Asset_ID AS src_asset_id, $$.Asset.Asset_ID AS dst_asset_id;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssets executing query: %s", queryStart.Format("15:04:05.000"), query)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssets execution completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

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

		srcVal, err := record.GetValueByIndex(0)
		if err != nil {
			log.Printf("nebula: row %d src error: %v", i, err)
			continue
		}

		dstVal, err := record.GetValueByIndex(1)
		if err != nil {
			log.Printf("nebula: row %d dst error: %v", i, err)
			continue
		}

		srcStr, err := srcVal.AsString()
		if err != nil {
			log.Printf("nebula: row %d src not string: %v", i, err)
			continue
		}

		dstStr, err := dstVal.AsString()
		if err != nil {
			log.Printf("nebula: row %d dst not string: %v", i, err)
			continue
		}

		rows = append(rows, AssetRow{
			SrcAssetID: srcStr,
			DstAssetID: dstStr,
		})
	}

	log.Printf("nebula: QueryAssets returned %d asset connectivity rows", len(rows))
	return rows, nil
}

// QueryAssetsWithDetails fetches all assets with their type information for the asset list (REQ-021).
func QueryAssetsWithDetails(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}

	// Native nGQL: LOOKUP all Assets and pipe to GO for type details (tested in console)
	query := `LOOKUP ON Asset YIELD id(vertex) AS vid | GO FROM $-.vid OVER has_type YIELD $^.Asset.Asset_ID AS asset_id, $$.Asset_Type.Type_Name AS type_name;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetsWithDetails executing query: %s", queryStart.Format("15:04:05.000"), query)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetsWithDetails execution completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

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

		assetID, _ := record.GetValueByIndex(0)
		typeName, _ := record.GetValueByIndex(1)

		assetIDStr, _ := assetID.AsString()
		typeNameStr, _ := typeName.AsString()

		assets = append(assets, map[string]interface{}{
			"asset_id":  assetIDStr,
			"type_name": typeNameStr,
		})
	}

	log.Printf("nebula: QueryAssetsWithDetails returned %d assets", len(assets))
	return assets, nil
}

// QueryAssetDetail fetches detailed information for a single asset (REQ-022).
func QueryAssetDetail(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) (map[string]interface{}, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}

	// Native nGQL: GO from specific VID (equals Asset_ID) to type vertex
	query := fmt.Sprintf(`GO FROM "%s" OVER has_type YIELD $^.Asset.Asset_ID AS asset_id, $^.Asset.Asset_Name AS asset_name, $^.Asset.priority AS asset_priority, $^.Asset.is_entrance AS is_entrance, $^.Asset.is_target AS is_target, $^.Asset.has_vulnerability AS has_vulnerability, $$.Asset_Type.Type_Name AS type_name;`, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetDetail executing query for asset %s: %s", queryStart.Format("15:04:05.000"), assetID, query)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetDetail execution completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

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

	assetIDVal, _ := record.GetValueByIndex(0)
	assetNameVal, _ := record.GetValueByIndex(1)
	priorityVal, _ := record.GetValueByIndex(2)
	isEntranceVal, _ := record.GetValueByIndex(3)
	isTargetVal, _ := record.GetValueByIndex(4)
	hasVulnVal, _ := record.GetValueByIndex(5)
	typeNameVal, _ := record.GetValueByIndex(6)

	assetIDStr, _ := assetIDVal.AsString()
	assetNameStr, _ := assetNameVal.AsString()
	priorityInt64, _ := priorityVal.AsInt()
	isEntranceBool, _ := isEntranceVal.AsBool()
	isTargetBool, _ := isTargetVal.AsBool()
	hasVulnBool, _ := hasVulnVal.AsBool()
	typeNameStr, _ := typeNameVal.AsString()

	detail := map[string]interface{}{
		"asset_id":          assetIDStr,
		"asset_name":        assetNameStr,
		"asset_priority":    int(priorityInt64),
		"is_entrance":       isEntranceBool,
		"is_target":         isTargetBool,
		"has_vulnerability": hasVulnBool,
		"type_name":         typeNameStr,
	}

	log.Printf("nebula: QueryAssetDetail returned detail for %s", assetID)
	return detail, nil
}

// QueryNeighbors fetches neighbors for the inspector panel (REQ-023).
func QueryNeighbors(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) ([]map[string]interface{}, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}

	// Native nGQL: GO from specific VID (equals Asset_ID) over edge to neighbors
	query := fmt.Sprintf(`GO FROM "%s" OVER connects_to YIELD $$.Asset.Asset_ID AS neighbor_id;`, assetID)

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryNeighbors executing query for asset %s: %s", queryStart.Format("15:04:05.000"), assetID, query)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryNeighbors execution completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

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

		neighborID, _ := record.GetValueByIndex(0)
		neighborIDStr, _ := neighborID.AsString()

		neighbors = append(neighbors, map[string]interface{}{
			"neighbor_id": neighborIDStr,
		})
	}

	log.Printf("nebula: QueryNeighbors returned %d neighbors for %s", len(neighbors), assetID)
	return neighbors, nil
}

// QueryAssetTypes fetches asset types with counts for filter dropdown (REQ-024).
func QueryAssetTypes(pool *nebula.ConnectionPool, cfg *config.Config) ([]map[string]interface{}, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}

	// Native nGQL: LOOKUP all Assets, GO to types, GROUP BY for counts (tested in console)
	query := `LOOKUP ON Asset YIELD id(vertex) AS vid | GO FROM $-.vid OVER has_type YIELD $$.Asset_Type.Type_Name AS type_name | GROUP BY $-.type_name YIELD $-.type_name AS type_name, COUNT(*) AS count;`

	queryStart := time.Now()
	log.Printf("[%s] nebula: QueryAssetTypes executing query: %s", queryStart.Format("15:04:05.000"), query)

	resultSet, err := session.Execute(query)
	queryDuration := time.Since(queryStart)
	log.Printf("[%s] nebula: QueryAssetTypes execution completed in %.3f seconds", time.Now().Format("15:04:05.000"), queryDuration.Seconds())

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

		typeName, _ := record.GetValueByIndex(0)
		count, _ := record.GetValueByIndex(1)

		typeNameStr, _ := typeName.AsString()
		countInt64, _ := count.AsInt()

		// CRITICAL FIX: Convert int64 to int for JSON compatibility
		countInt := int(countInt64)

		types = append(types, map[string]interface{}{
			"type_name": typeNameStr,
			"count":     countInt,
		})
	}

	log.Printf("nebula: QueryAssetTypes returned %d asset types", len(types))
	return types, nil
}
