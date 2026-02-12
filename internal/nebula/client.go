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
