package nebula

import (
	"fmt"
	"log"
	"regexp"

	"ESP-data/config"
	nebula "github.com/vesoft-inc/nebula-go/v3"
)

// AssetRow represents one row from the enriched connectivity query (REQ-020).
// Now includes asset properties and type names for both source and destination.
type AssetRow struct {
	// Source asset fields
	SrcAssetID          string
	SrcAssetName        string
	SrcIsEntrance       bool
	SrcIsTarget         bool
	SrcPriority         int
	SrcHasVulnerability bool
	SrcAssetType        string // from has_type -> Asset_Type.Type_Name

	// Destination asset fields
	DstAssetID          string
	DstAssetName        string
	DstIsEntrance       bool
	DstIsTarget         bool
	DstPriority         int
	DstHasVulnerability bool
	DstAssetType        string // from has_type -> Asset_Type.Type_Name
}

// AssetListItem represents one asset in the sidebar list (REQ-021).
type AssetListItem struct {
	AssetID          string `json:"asset_id"`
	AssetName        string `json:"asset_name"`
	AssetType        string `json:"asset_type"`
	IsEntrance       bool   `json:"is_entrance"`
	IsTarget         bool   `json:"is_target"`
	Priority         int    `json:"priority"`
	HasVulnerability bool   `json:"has_vulnerability"`
}

// AssetDetail represents complete info for one asset (REQ-022).
type AssetDetail struct {
	AssetID          string  `json:"asset_id"`
	AssetName        string  `json:"asset_name"`
	AssetDescription *string `json:"asset_description,omitempty"` // nullable
	AssetNote        *string `json:"asset_note,omitempty"`        // nullable
	AssetType        *string `json:"asset_type,omitempty"`        // from has_type
	SegmentName      *string `json:"segment_name,omitempty"`      // from belongs_to
	IsEntrance       bool    `json:"is_entrance"`
	IsTarget         bool    `json:"is_target"`
	Priority         int     `json:"priority"`
	HasVulnerability bool    `json:"has_vulnerability"`
	TTB              *int    `json:"ttb,omitempty"` // Time To Bypass, nullable
}

// NeighborItem represents one neighbor in the connections list (REQ-023).
type NeighborItem struct {
	NeighborID string `json:"neighbor_id"`
	Direction  string `json:"direction"` // "inbound" or "outbound"
}

// AssetTypeItem represents one asset type (REQ-024).
type AssetTypeItem struct {
	TypeID   string `json:"type_id"`
	TypeName string `json:"type_name"`
}

// assetIDPattern validates Asset_ID format (REQ-025).
// Expected format: A followed by 4-5 digits (e.g., A00001, A12345).
var assetIDPattern = regexp.MustCompile(`^A\d{4,5}$`)

// ValidateAssetID checks if an asset ID matches the expected format (REQ-025).
func ValidateAssetID(id string) bool {
	return assetIDPattern.MatchString(id)
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

// QueryAssets executes the enriched connectivity query from REQ-020.
// Returns rows with asset properties and type names for both src and dst.
// Uses MATCH syntax per REQ-244 justification: OPTIONAL MATCH with multi-hop
// property retrieval is significantly cleaner than chained GO statements.
func QueryAssets(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetRow, error) {
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

	// Execute the enriched query from REQ-020
	// MATCH syntax justified: OPTIONAL MATCH retrieves properties from related tags cleanly
	query := `MATCH (a:Asset)-[e:connects_to]->(b:Asset)
OPTIONAL MATCH (a)-[:has_type]->(at:Asset_Type)
OPTIONAL MATCH (b)-[:has_type]->(bt:Asset_Type)
RETURN
  a.Asset.Asset_ID AS src_asset_id,
  a.Asset.Asset_Name AS src_asset_name,
  a.Asset.is_entrance AS src_is_entrance,
  a.Asset.is_target AS src_is_target,
  a.Asset.priority AS src_priority,
  a.Asset.has_vulnerability AS src_has_vulnerability,
  at.Asset_Type.Type_Name AS src_asset_type,
  b.Asset.Asset_ID AS dst_asset_id,
  b.Asset.Asset_Name AS dst_asset_name,
  b.Asset.is_entrance AS dst_is_entrance,
  b.Asset.is_target AS dst_is_target,
  b.Asset.priority AS dst_priority,
  b.Asset.has_vulnerability AS dst_has_vulnerability,
  bt.Asset_Type.Type_Name AS dst_asset_type
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

		// Helper to safely extract string (handles NULL)
		getString := func(index int) string {
			val, err := record.GetValueByIndex(index)
			if err != nil || val.IsNull() {
				return ""
			}
			str, _ := val.AsString()
			return str
		}

		// Helper to safely extract bool (handles NULL, defaults to false)
		getBool := func(index int) bool {
			val, err := record.GetValueByIndex(index)
			if err != nil || val.IsNull() {
				return false
			}
			b, _ := val.AsBool()
			return b
		}

		// Helper to safely extract int (handles NULL, defaults to 0)
		getInt := func(index int) int {
			val, err := record.GetValueByIndex(index)
			if err != nil || val.IsNull() {
				return 0
			}
			i64, _ := val.AsInt()
			return int(i64)
		}

		// Extract all 14 columns per REQ-020 query
		rows = append(rows, AssetRow{
			SrcAssetID:          getString(0),
			SrcAssetName:        getString(1),
			SrcIsEntrance:       getBool(2),
			SrcIsTarget:         getBool(3),
			SrcPriority:         getInt(4),
			SrcHasVulnerability: getBool(5),
			SrcAssetType:        getString(6),
			DstAssetID:          getString(7),
			DstAssetName:        getString(8),
			DstIsEntrance:       getBool(9),
			DstIsTarget:         getBool(10),
			DstPriority:         getInt(11),
			DstHasVulnerability: getBool(12),
			DstAssetType:        getString(13),
		})
	}

	log.Printf("nebula: QueryAssets returned %d rows", len(rows))
	return rows, nil
}

// QueryAssetsList executes the query from REQ-021 to populate the sidebar.
// Supports optional server-side filtering by asset type and search string.
func QueryAssetsList(pool *nebula.ConnectionPool, cfg *config.Config, assetType, search string) ([]AssetListItem, error) {
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

	// Base query from REQ-021 (MATCH syntax justified: same as REQ-020)
	query := `MATCH (a:Asset)
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.Asset_Name AS asset_name,
  a.Asset.is_entrance AS is_entrance,
  a.Asset.is_target AS is_target,
  a.Asset.priority AS priority,
  a.Asset.has_vulnerability AS has_vulnerability,
  t.Asset_Type.Type_Name AS asset_type;`

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	// Parse and filter results
	items := make([]AssetListItem, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}

		getString := func(index int) string {
			val, _ := record.GetValueByIndex(index)
			if val.IsNull() {
				return ""
			}
			str, _ := val.AsString()
			return str
		}
		getBool := func(index int) bool {
			val, _ := record.GetValueByIndex(index)
			if val.IsNull() {
				return false
			}
			b, _ := val.AsBool()
			return b
		}
		getInt := func(index int) int {
			val, _ := record.GetValueByIndex(index)
			if val.IsNull() {
				return 0
			}
			i64, _ := val.AsInt()
			return int(i64)
		}

		item := AssetListItem{
			AssetID:          getString(0),
			AssetName:        getString(1),
			IsEntrance:       getBool(2),
			IsTarget:         getBool(3),
			Priority:         getInt(4),
			HasVulnerability: getBool(5),
			AssetType:        getString(6),
		}

		// Server-side filtering (basic implementation)
		if assetType != "" && item.AssetType != assetType {
			continue
		}
		if search != "" && item.AssetID != search && item.AssetName != search {
			// Simple exact match; enhance with CONTAINS if needed
			continue
		}

		items = append(items, item)
	}

	log.Printf("nebula: QueryAssetsList returned %d items", len(items))
	return items, nil
}

// QueryAssetDetail executes the query from REQ-022 for the inspector panel.
// Returns detailed info for a single asset, including type and segment.
func QueryAssetDetail(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) (*AssetDetail, error) {
	// Validate input per REQ-025
	if !ValidateAssetID(assetID) {
		return nil, fmt.Errorf("invalid asset ID format: %s", assetID)
	}

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

	// Query from REQ-022 (MATCH syntax justified: multi-hop OPTIONAL MATCH)
	query := fmt.Sprintf(`MATCH (a:Asset) WHERE a.Asset.Asset_ID == "%s"
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
OPTIONAL MATCH (a)-[:belongs_to]->(s:Network_Segment)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.Asset_Name AS asset_name,
  a.Asset.Asset_Description AS asset_description,
  a.Asset.Asset_Note AS asset_note,
  a.Asset.is_entrance AS is_entrance,
  a.Asset.is_target AS is_target,
  a.Asset.priority AS priority,
  a.Asset.has_vulnerability AS has_vulnerability,
  a.Asset.TTB AS ttb,
  t.Asset_Type.Type_Name AS asset_type,
  s.Network_Segment.Segment_Name AS segment_name;`, assetID)

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
		return nil, fmt.Errorf("failed to read result: %w", err)
	}

	// Helper to get nullable string
	getNullableString := func(index int) *string {
		val, _ := record.GetValueByIndex(index)
		if val.IsNull() {
			return nil
		}
		str, _ := val.AsString()
		return &str
	}
	getString := func(index int) string {
		val, _ := record.GetValueByIndex(index)
		if val.IsNull() {
			return ""
		}
		str, _ := val.AsString()
		return str
	}
	getBool := func(index int) bool {
		val, _ := record.GetValueByIndex(index)
		if val.IsNull() {
			return false
		}
		b, _ := val.AsBool()
		return b
	}
	getInt := func(index int) int {
		val, _ := record.GetValueByIndex(index)
		if val.IsNull() {
			return 0
		}
		i64, _ := val.AsInt()
		return int(i64)
	}
	getNullableInt := func(index int) *int {
		val, _ := record.GetValueByIndex(index)
		if val.IsNull() {
			return nil
		}
		i64, _ := val.AsInt()
		i := int(i64)
		return &i
	}

	detail := &AssetDetail{
		AssetID:          getString(0),
		AssetName:        getString(1),
		AssetDescription: getNullableString(2),
		AssetNote:        getNullableString(3),
		IsEntrance:       getBool(4),
		IsTarget:         getBool(5),
		Priority:         getInt(6),
		HasVulnerability: getBool(7),
		TTB:              getNullableInt(8),
		AssetType:        getNullableString(9),
		SegmentName:      getNullableString(10),
	}

	log.Printf("nebula: QueryAssetDetail found asset %s", assetID)
	return detail, nil
}

// QueryNeighbors executes the query from REQ-023 for neighbor list.
// Returns immediate neighbors with direction (inbound/outbound).
// Uses pure nGQL per REQ-243.
func QueryNeighbors(pool *nebula.ConnectionPool, cfg *config.Config, assetID string) ([]NeighborItem, error) {
	// Validate input per REQ-025
	if !ValidateAssetID(assetID) {
		return nil, fmt.Errorf("invalid asset ID format: %s", assetID)
	}

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

	// Query from REQ-023 (pure nGQL with GO and UNION per REQ-243)
	query := fmt.Sprintf(`GO FROM "%s" OVER connects_to
YIELD connects_to._dst AS neighbor_id, "outbound" AS direction
UNION
GO FROM "%s" OVER connects_to REVERSELY
YIELD connects_to._dst AS neighbor_id, "inbound" AS direction;`, assetID, assetID)

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	neighbors := make([]NeighborItem, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}

		neighborVal, _ := record.GetValueByIndex(0)
		directionVal, _ := record.GetValueByIndex(1)

		neighborStr, _ := neighborVal.AsString()
		directionStr, _ := directionVal.AsString()

		neighbors = append(neighbors, NeighborItem{
			NeighborID: neighborStr,
			Direction:  directionStr,
		})
	}

	log.Printf("nebula: QueryNeighbors found %d neighbors for %s", len(neighbors), assetID)
	return neighbors, nil
}

// QueryAssetTypes executes the query from REQ-024 for filter checkboxes.
// Returns all distinct asset types.
// Uses pure nGQL per REQ-243.
func QueryAssetTypes(pool *nebula.ConnectionPool, cfg *config.Config) ([]AssetTypeItem, error) {
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

	// Query from REQ-024 (pure nGQL LOOKUP per REQ-243)
	query := `LOOKUP ON Asset_Type
YIELD Asset_Type.Type_ID AS type_id,
      Asset_Type.Type_Name AS type_name;`

	resultSet, err := session.Execute(query)
	if err != nil {
		return nil, fmt.Errorf("query execution failed: %w", err)
	}
	if !resultSet.IsSucceed() {
		return nil, fmt.Errorf("query failed: %s", resultSet.GetErrorMsg())
	}

	types := make([]AssetTypeItem, 0, resultSet.GetRowSize())
	for i := 0; i < resultSet.GetRowSize(); i++ {
		record, err := resultSet.GetRowValuesByIndex(i)
		if err != nil {
			continue
		}

		typeIDVal, _ := record.GetValueByIndex(0)
		typeNameVal, _ := record.GetValueByIndex(1)

		typeIDStr, _ := typeIDVal.AsString()
		typeNameStr, _ := typeNameVal.AsString()

		types = append(types, AssetTypeItem{
			TypeID:   typeIDStr,
			TypeName: typeNameStr,
		})
	}

	log.Printf("nebula: QueryAssetTypes found %d asset types", len(types))
	return types, nil
}
