package graph

import "ESP-data/internal/nebula"

// ============================================================
// Cytoscape.js graph structures (REQ-020, REQ-122)
// ============================================================

// CyGraph represents Cytoscape.js compatible JSON format.
type CyGraph struct {
	Nodes []CyNode `json:"nodes"`
	Edges []CyEdge `json:"edges"`
}

// CyNode is a single node in Cytoscape format.
type CyNode struct {
	Data CyNodeData `json:"data"`
}

// CyNodeData holds every attribute the front-end needs for rendering:
// colours by type, labels, priority borders, entrance/target shapes,
// and vulnerability markers (UI-REQ-201 through UI-REQ-205).
type CyNodeData struct {
	ID               string `json:"id"`
	Label            string `json:"label"`
	AssetType        string `json:"asset_type"`
	IsEntrance       bool   `json:"is_entrance"`
	IsTarget         bool   `json:"is_target"`
	Priority         int    `json:"priority"`
	HasVulnerability bool   `json:"has_vulnerability"`
}

// CyEdge is a single directed edge in Cytoscape format (REQ-012).
type CyEdge struct {
	Data CyEdgeData `json:"data"`
}

// CyEdgeData holds the edge's source and target vertex IDs.
type CyEdgeData struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// BuildGraph converts the enriched Nebula query results (REQ-020) into
// Cytoscape.js elements format. Nodes are de-duplicated because the
// same asset can appear as both source and destination across rows.
func BuildGraph(rows []nebula.AssetRow) CyGraph {
	// De-duplicate nodes — keep the richest version of each
	type nodeInfo struct {
		Name             string
		AssetType        string
		IsEntrance       bool
		IsTarget         bool
		Priority         int
		HasVulnerability bool
	}
	nodeSet := make(map[string]nodeInfo, len(rows))

	addNode := func(id, name, assetType string, entrance, target bool, prio int, vuln bool) {
		if id == "" {
			return
		}
		if _, exists := nodeSet[id]; !exists {
			nodeSet[id] = nodeInfo{
				Name:             name,
				AssetType:        assetType,
				IsEntrance:       entrance,
				IsTarget:         target,
				Priority:         prio,
				HasVulnerability: vuln,
			}
		}
	}

	for _, row := range rows {
		addNode(row.SrcAssetID, row.SrcAssetName, row.SrcAssetType,
			row.SrcIsEntrance, row.SrcIsTarget, row.SrcPriority, row.SrcHasVulnerability)
		addNode(row.DstAssetID, row.DstAssetName, row.DstAssetType,
			row.DstIsEntrance, row.DstIsTarget, row.DstPriority, row.DstHasVulnerability)
	}

	// Build node list
	nodes := make([]CyNode, 0, len(nodeSet))
	for id, info := range nodeSet {
		label := id
		if info.Name != "" {
			label = info.Name
		}
		nodes = append(nodes, CyNode{
			Data: CyNodeData{
				ID:               id,
				Label:            label,
				AssetType:        info.AssetType,
				IsEntrance:       info.IsEntrance,
				IsTarget:         info.IsTarget,
				Priority:         info.Priority,
				HasVulnerability: info.HasVulnerability,
			},
		})
	}

	// Build edge list — de-duplicated per REQ-027.
	// At most one visual edge per (source, target) pair, regardless of
	// how many connects_to edges exist in the database.
	edgeSeen := make(map[string]bool, len(rows))
	edges := make([]CyEdge, 0, len(rows))
	for _, row := range rows {
		key := row.SrcAssetID + "|" + row.DstAssetID
		if edgeSeen[key] {
			continue
		}
		edgeSeen[key] = true
		edges = append(edges, CyEdge{
			Data: CyEdgeData{
				Source: row.SrcAssetID,
				Target: row.DstAssetID,
			},
		})
	}

	return CyGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

// ============================================================
// Asset list response (REQ-021 — sidebar entity browser)
// ============================================================

// AssetsListResponse wraps the asset list for JSON response.
type AssetsListResponse struct {
	Assets   []AssetWithDetails `json:"assets"`
	Total    int                `json:"total"`
	Filtered int                `json:"filtered"`
}

// AssetWithDetails carries every field the sidebar needs:
// ID, name, type badge, and boolean/priority badges.
type AssetWithDetails struct {
	AssetID          string `json:"asset_id"`
	AssetName        string `json:"asset_name"`
	AssetType        string `json:"asset_type"`
	IsEntrance       bool   `json:"is_entrance"`
	IsTarget         bool   `json:"is_target"`
	Priority         int    `json:"priority"`
	HasVulnerability bool   `json:"has_vulnerability"`
}

// BuildAssetsList converts the raw query maps into the typed response.
func BuildAssetsList(items []map[string]interface{}, totalCount int) AssetsListResponse {
	assets := make([]AssetWithDetails, 0, len(items))
	for _, item := range items {
		assets = append(assets, AssetWithDetails{
			AssetID:          mapStr(item, "asset_id"),
			AssetName:        mapStr(item, "asset_name"),
			AssetType:        mapStr(item, "asset_type"),
			IsEntrance:       mapBool(item, "is_entrance"),
			IsTarget:         mapBool(item, "is_target"),
			Priority:         mapInt(item, "priority"),
			HasVulnerability: mapBool(item, "has_vulnerability"),
		})
	}
	return AssetsListResponse{
		Assets:   assets,
		Total:    totalCount,
		Filtered: len(items),
	}
}

// ============================================================
// Single asset detail response (REQ-022 — inspector panel)
// ============================================================

// AssetDetail carries every field the inspector panel needs.
// Serialised flat (no wrapper) so the front-end can access
// detail.asset_id directly.
type AssetDetail struct {
	AssetID          string `json:"asset_id"`
	AssetName        string `json:"asset_name"`
	AssetDescription string `json:"asset_description"`
	AssetNote        string `json:"asset_note"`
	AssetType        string `json:"asset_type"`
	SegmentName      string `json:"segment_name"`
	IsEntrance       bool   `json:"is_entrance"`
	IsTarget         bool   `json:"is_target"`
	Priority         int    `json:"priority"`
	HasVulnerability bool   `json:"has_vulnerability"`
	TTB              int    `json:"ttb"`
}

// BuildAssetDetailResponse maps the raw query result into a typed struct.
// Returns the struct directly — NOT wrapped in { "detail": ... } —
// because the front-end reads detail.asset_id, detail.asset_name, etc.
func BuildAssetDetailResponse(detail map[string]interface{}) AssetDetail {
	return AssetDetail{
		AssetID:          mapStr(detail, "asset_id"),
		AssetName:        mapStr(detail, "asset_name"),
		AssetDescription: mapStr(detail, "asset_description"),
		AssetNote:        mapStr(detail, "asset_note"),
		AssetType:        mapStr(detail, "asset_type"),
		SegmentName:      mapStr(detail, "segment_name"),
		IsEntrance:       mapBool(detail, "is_entrance"),
		IsTarget:         mapBool(detail, "is_target"),
		Priority:         mapInt(detail, "priority"),
		HasVulnerability: mapBool(detail, "has_vulnerability"),
		TTB:              mapInt(detail, "ttb"),
	}
}

// ============================================================
// Neighbors response (REQ-023 — inspector connections list)
// ============================================================

// NeighborsResponse wraps the neighbors list for JSON response.
type NeighborsResponse struct {
	Neighbors []Neighbor `json:"neighbors"`
	Total     int        `json:"total"`
}

// Neighbor represents one connected asset with edge direction.
type Neighbor struct {
	NeighborID string `json:"neighbor_id"`
	Direction  string `json:"direction"`
}

// BuildNeighborsList converts the raw query maps into the typed response.
func BuildNeighborsList(neighbors []map[string]interface{}) NeighborsResponse {
	neighborList := make([]Neighbor, 0, len(neighbors))
	for _, n := range neighbors {
		neighborList = append(neighborList, Neighbor{
			NeighborID: mapStr(n, "neighbor_id"),
			Direction:  mapStr(n, "direction"),
		})
	}
	return NeighborsResponse{
		Neighbors: neighborList,
		Total:     len(neighborList),
	}
}

// ============================================================
// Asset types response (REQ-024 — filter checkboxes)
// ============================================================

// AssetTypesResponse wraps the asset types list for JSON response.
type AssetTypesResponse struct {
	AssetTypes []AssetTypeItem `json:"asset_types"`
	Total      int             `json:"total"`
}

// AssetTypeItem represents one asset type from the LOOKUP query.
type AssetTypeItem struct {
	TypeID   string `json:"type_id"`
	TypeName string `json:"type_name"`
}

// BuildAssetTypesList converts the raw query maps into the typed response.
func BuildAssetTypesList(types []map[string]interface{}) AssetTypesResponse {
	assetTypes := make([]AssetTypeItem, 0, len(types))
	for _, t := range types {
		assetTypes = append(assetTypes, AssetTypeItem{
			TypeID:   mapStr(t, "type_id"),
			TypeName: mapStr(t, "type_name"),
		})
	}
	return AssetTypesResponse{
		AssetTypes: assetTypes,
		Total:      len(assetTypes),
	}
}

// ============================================================
// ============================================================
// Edge detail response (REQ-026 — edge inspector panel)
// ============================================================

// EdgeAssetSummary carries the subset of asset fields shown in the
// edge inspector's Source/Target blocks (UI-REQ-212).
type EdgeAssetSummary struct {
	AssetID          string `json:"asset_id"`
	AssetName        string `json:"asset_name"`
	AssetDescription string `json:"asset_description"`
}

// EdgeConnection represents one connects_to edge between two assets.
type EdgeConnection struct {
	ConnectionProtocol string `json:"connection_protocol"`
	ConnectionPort     string `json:"connection_port"`
}

// EdgeDetailResponse is the combined response for GET /api/edges/{src}/{dst}.
type EdgeDetailResponse struct {
	Source      EdgeAssetSummary `json:"source"`
	Target      EdgeAssetSummary `json:"target"`
	Connections []EdgeConnection `json:"connections"`
	Total       int              `json:"total"`
}

// BuildEdgeDetailResponse assembles the edge inspector response from
// the source/target asset details (REQ-022 reuse) and the edge
// connection rows (REQ-026).
func BuildEdgeDetailResponse(srcDetail, dstDetail map[string]interface{}, connections []map[string]interface{}) EdgeDetailResponse {
	conns := make([]EdgeConnection, 0, len(connections))
	for _, c := range connections {
		conns = append(conns, EdgeConnection{
			ConnectionProtocol: mapStr(c, "connection_protocol"),
			ConnectionPort:     mapStr(c, "connection_port"),
		})
	}
	return EdgeDetailResponse{
		Source: EdgeAssetSummary{
			AssetID:          mapStr(srcDetail, "asset_id"),
			AssetName:        mapStr(srcDetail, "asset_name"),
			AssetDescription: mapStr(srcDetail, "asset_description"),
		},
		Target: EdgeAssetSummary{
			AssetID:          mapStr(dstDetail, "asset_id"),
			AssetName:        mapStr(dstDetail, "asset_name"),
			AssetDescription: mapStr(dstDetail, "asset_description"),
		},
		Connections: conns,
		Total:       len(conns),
	}
}

// Safe map accessors — prevent panics on missing/wrong-typed keys
// ============================================================

func mapStr(m map[string]interface{}, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

func mapBool(m map[string]interface{}, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

func mapInt(m map[string]interface{}, key string) int {
	if v, ok := m[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return 0
}
