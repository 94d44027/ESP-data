package nebula

import (
	"fmt"
	"log"

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
	// Get a session from the pool
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}

	defer session.Release()

	// Switch to the target space
	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	useResult, err := session.Execute(useStmt)
	if err != nil {
		return nil, fmt.Errorf("failed to USE space: %w", err)
	}

	if !useResult.IsSucceed() {
		return nil, fmt.Errorf("USE space failed: %s", useResult.GetErrorMsg())
	}

	// Execute the query from REQ-020
	// Note: This uses Cypher (MATCH) as explicitly specified in requirements
	query := `MATCH (a:Asset)-[e:connects_to]->(b:Asset)
RETURN a.Asset.Asset_ID AS src_asset_id,
       b.Asset.Asset_ID AS dst_asset_id
LIMIT 300;`

	log.Printf("nebula: QueryAssets executing: %s", query)

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}

	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	// Parse the result rows
	rows := make([]AssetRow, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping row %d: %v", i, err)
			continue
		}

		// Extract src_asset_id and dst_asset_id
		// Columns are: [0]=src_asset_id, [1]=dst_asset_id
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

		// Convert ValueWrapper to string
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

	log.Printf("nebula: query returned %d asset connectivity rows", len(rows))
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

	query := `MATCH (a:Asset)-[:has_type]->(t:Asset_Type)
RETURN a.Asset.Asset_ID AS asset_id,
       t.Asset_Type.Type_Name AS type_name,
       count(*) AS count;`

	log.Printf("nebula: QueryAssetsWithDetails executing: %s", query)

	resultSet, err := session.Execute(query)
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

	query := fmt.Sprintf(`MATCH (a:Asset)-[:has_type]->(t:Asset_Type)
WHERE a.Asset.Asset_ID == "%s"
RETURN a.Asset.Asset_ID AS asset_id,
       t.Asset_Type.Type_Name AS type_name;`, assetID)

	log.Printf("nebula: QueryAssetDetail executing: %s", query)

	resultSet, err := session.Execute(query)
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
	typeNameVal, _ := record.GetValueByIndex(1)

	assetIDStr, _ := assetIDVal.AsString()
	typeNameStr, _ := typeNameVal.AsString()

	detail := map[string]interface{}{
		"asset_id":  assetIDStr,
		"type_name": typeNameStr,
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

	query := fmt.Sprintf(`MATCH (a:Asset)-[:connects_to]->(b:Asset)
WHERE a.Asset.Asset_ID == "%s"
RETURN b.Asset.Asset_ID AS neighbor_id;`, assetID)

	log.Printf("nebula: QueryNeighbors executing: %s", query)

	resultSet, err := session.Execute(query)
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

	query := `MATCH (a:Asset)-[:has_type]->(t:Asset_Type)
RETURN t.Asset_Type.Type_Name AS type_name,
       count(a) AS count;`

	log.Printf("nebula: QueryAssetTypes executing: %s", query)

	resultSet, err := session.Execute(query)
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
		countInt, _ := count.AsInt()

		types = append(types, map[string]interface{}{
			"type_name": typeNameStr,
			"count":     countInt,
		})
	}

	log.Printf("nebula: QueryAssetTypes returned %d asset types", len(types))
	return types, nil
}
