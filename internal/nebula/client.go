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

// AssetWithDetails represents an asset for the sidebar list (UI-REQ-120)
type AssetWithDetails struct {
	AssetID          string `json:"asset_id"`
	AssetName        string `json:"asset_name"`
	AssetType        string `json:"asset_type"`
	Priority         int    `json:"priority"`
	IsEntrance       bool   `json:"is_entrance"`
	IsTarget         bool   `json:"is_target"`
	HasVulnerability bool   `json:"has_vulnerability"`
}

// AssetDetail represents full asset info for the inspector (UI-REQ-211)
type AssetDetail struct {
	AssetID          string `json:"asset_id"`
	AssetName        string `json:"asset_name"`
	AssetType        string `json:"asset_type"`
	Priority         int    `json:"priority"`
	IsEntrance       bool   `json:"is_entrance"`
	IsTarget         bool   `json:"is_target"`
	HasVulnerability bool   `json:"has_vulnerability"`
}

// Neighbor represents a connected asset for the inspector connections list (UI-REQ-212)
type Neighbor struct {
	NeighborID   string `json:"neighbor_id"`
	NeighborName string `json:"neighbor_name"`
	Direction    string `json:"direction"` // "outgoing" or "incoming"
}

// AssetTypeCount represents asset type with count for filters (UI-REQ-122)
type AssetTypeCount struct {
	TypeName string `json:"type_name"`
	Count    int    `json:"count"`
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

// QueryAssetsWithDetails fetches all assets with properties for sidebar (REQ-021).
func QueryAssetsWithDetails(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetWithDetails, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		if err != nil {
			return nil, fmt.Errorf("failed to USE space: %w", err)
		}
		return nil, fmt.Errorf("USE space failed: %s", res.GetErrorMsg())
	}

	query := `
		MATCH (a:Asset)
		RETURN a.Asset.Asset_ID        AS asset_id,
		       a.Asset.Asset_Name      AS asset_name,
		       a.Asset.Asset_Type      AS asset_type,
		       a.Asset.Priority        AS priority,
		       a.Asset.Is_Entrance     AS is_entrance,
		       a.Asset.Is_Target       AS is_target,
		       a.Asset.Has_Vuln        AS has_vulnerability
		LIMIT 500;
	`
	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	assets := make([]AssetWithDetails, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			log.Printf("nebula: skipping asset row %d: %v", i, err)
			continue
		}

		var a AssetWithDetails

		if v, _ := record.GetValueByIndex(0); v != nil {
			a.AssetID, _ = v.AsString()
		}
		if v, _ := record.GetValueByIndex(1); v != nil {
			a.AssetName, _ = v.AsString()
		}
		if v, _ := record.GetValueByIndex(2); v != nil {
			a.AssetType, _ = v.AsString()
		}
		if v, _ := record.GetValueByIndex(3); v != nil {
			if i64, err := v.AsInt(); err == nil {
				a.Priority = int(i64)
			}
		}
		if v, _ := record.GetValueByIndex(4); v != nil {
			a.IsEntrance, _ = v.AsBool()
		}
		if v, _ := record.GetValueByIndex(5); v != nil {
			a.IsTarget, _ = v.AsBool()
		}
		if v, _ := record.GetValueByIndex(6); v != nil {
			a.HasVulnerability, _ = v.AsBool()
		}

		assets = append(assets, a)
	}

	log.Printf("nebula: QueryAssetsWithDetails returned %d assets", len(assets))
	return assets, nil
}

// QueryAssetDetail fetches details for a single asset (REQ-022).
func QueryAssetDetail(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) (*AssetDetail, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		if err != nil {
			return nil, fmt.Errorf("failed to USE space: %w", err)
		}
		return nil, fmt.Errorf("USE space failed: %s", res.GetErrorMsg())
	}

	query := fmt.Sprintf(`
		MATCH (a:Asset {Asset_ID: "%s"})
		RETURN a.Asset.Asset_ID        AS asset_id,
		       a.Asset.Asset_Name      AS asset_name,
		       a.Asset.Asset_Type      AS asset_type,
		       a.Asset.Priority        AS priority,
		       a.Asset.Is_Entrance     AS is_entrance,
		       a.Asset.Is_Target       AS is_target,
		       a.Asset.Has_Vuln        AS has_vulnerability;
	`, assetID)

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}
	if resultSet.GetRowSize() == 0 {
		return nil, fmt.Errorf("asset not found: %s", assetID)
	}

	record, err := resultSet.GetRowValuesByIndex(0)
	if err != nil {
		return nil, fmt.Errorf("failed to read row: %w", err)
	}

	d := &AssetDetail{}
	if v, _ := record.GetValueByIndex(0); v != nil {
		d.AssetID, _ = v.AsString()
	}
	if v, _ := record.GetValueByIndex(1); v != nil {
		d.AssetName, _ = v.AsString()
	}
	if v, _ := record.GetValueByIndex(2); v != nil {
		d.AssetType, _ = v.AsString()
	}
	if v, _ := record.GetValueByIndex(3); v != nil {
		if i64, err := v.AsInt(); err == nil {
			d.Priority = int(i64)
		}
	}
	if v, _ := record.GetValueByIndex(4); v != nil {
		d.IsEntrance, _ = v.AsBool()
	}
	if v, _ := record.GetValueByIndex(5); v != nil {
		d.IsTarget, _ = v.AsBool()
	}
	if v, _ := record.GetValueByIndex(6); v != nil {
		d.HasVulnerability, _ = v.AsBool()
	}

	log.Printf("nebula: QueryAssetDetail returned detail for %s", assetID)
	return d, nil
}

// QueryNeighbors fetches neighbors for a given asset (REQ-023).
func QueryNeighbors(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) ([]Neighbor, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		if err != nil {
			return nil, fmt.Errorf("failed to USE space: %w", err)
		}
		return nil, fmt.Errorf("USE space failed: %s", res.GetErrorMsg())
	}

	outgoing := fmt.Sprintf(`
		MATCH (a:Asset {Asset_ID: "%s"})-[:connects_to]->(b:Asset)
		RETURN b.Asset.Asset_ID   AS neighbor_id,
		       b.Asset.Asset_Name AS neighbor_name,
		       "outgoing"         AS direction;
	`, assetID)

	incoming := fmt.Sprintf(`
		MATCH (a:Asset)-[:connects_to]->(b:Asset {Asset_ID: "%s"})
		RETURN a.Asset.Asset_ID   AS neighbor_id,
		       a.Asset.Asset_Name AS neighbor_name,
		       "incoming"         AS direction;
	`, assetID)

	neighbors := make([]Neighbor, 0)

	// Outgoing
	if rs, err := session.Execute(outgoing); err == nil && rs.IsSucceed() {
		for i := 0; i < rs.GetRowSize(); i++ {
			record, err := rs.GetRowValuesByIndex(i)
			if err != nil {
				continue
			}
			var n Neighbor
			if v, _ := record.GetValueByIndex(0); v != nil {
				n.NeighborID, _ = v.AsString()
			}
			if v, _ := record.GetValueByIndex(1); v != nil {
				n.NeighborName, _ = v.AsString()
			}
			n.Direction = "outgoing"
			neighbors = append(neighbors, n)
		}
	}

	// Incoming
	if rs, err := session.Execute(incoming); err == nil && rs.IsSucceed() {
		for i := 0; i < rs.GetRowSize(); i++ {
			record, err := rs.GetRowValuesByIndex(i)
			if err != nil {
				continue
			}
			var n Neighbor
			if v, _ := record.GetValueByIndex(0); v != nil {
				n.NeighborID, _ = v.AsString()
			}
			if v, _ := record.GetValueByIndex(1); v != nil {
				n.NeighborName, _ = v.AsString()
			}
			n.Direction = "incoming"
			neighbors = append(neighbors, n)
		}
	}

	log.Printf("nebula: QueryNeighbors returned %d neighbors for %s", len(neighbors), assetID)
	return neighbors, nil
}

// QueryAssetTypes returns asset types with counts (REQ-024).
func QueryAssetTypes(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetTypeCount, error) {
	session, err := pool.GetSession(cfg.NebulaUser, cfg.NebulaPwd)
	if err != nil {
		return nil, fmt.Errorf("failed to get session: %w", err)
	}
	defer session.Release()

	useStmt := fmt.Sprintf("USE %s;", cfg.Space)
	if res, err := session.Execute(useStmt); err != nil || !res.IsSucceed() {
		if err != nil {
			return nil, fmt.Errorf("failed to USE space: %w", err)
		}
		return nil, fmt.Errorf("USE space failed: %s", res.GetErrorMsg())
	}

	query := `
		MATCH (a:Asset)
		RETURN a.Asset.Asset_Type AS type_name,
		       count(*)           AS count
		GROUP BY a.Asset.Asset_Type;
	`
	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	types := make([]AssetTypeCount, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}
		var t AssetTypeCount
		if v, _ := record.GetValueByIndex(0); v != nil {
			t.TypeName, _ = v.AsString()
		}
		if v, _ := record.GetValueByIndex(1); v != nil {
			if i64, err := v.AsInt(); err == nil {
				t.Count = int(i64)
			}
		}
		types = append(types, t)
	}

	log.Printf("nebula: QueryAssetTypes returned %d types", len(types))
	return types, nil
}
