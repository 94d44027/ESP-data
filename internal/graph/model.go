package graph

import "ESP-data/internal/nebula"

// CyGraph represents Cytoscape.js compatible JSON format.
// This satisfies REQ-122 (Cytoscape.js JSON format).
type CyGraph struct {
	Nodes []CyNode `json:"nodes"`
	Edges []CyEdge `json:"edges"`
}

// CyNode is a single node in Cytoscape format.
type CyNode struct {
	Data CyNodeData `json:"data"`
}

// CyNodeData holds the node's attributes.
type CyNodeData struct {
	ID string `json:"id"`
}

// CyEdge is a single edge in Cytoscape format.
type CyEdge struct {
	Data CyEdgeData `json:"data"`
}

// CyEdgeData holds the edge's attributes.
type CyEdgeData struct {
	Source string `json:"source"`
	Target string `json:"target"`
}

// BuildGraph converts Nebula query results into Cytoscape.js graph format.
// This satisfies REQ-013 (Asset_ID in node), REQ-012 (directed edges for arrowheads).
func BuildGraph(rows []nebula.AssetRow) CyGraph {
	// Step 1: Collect unique node IDs
	nodeSet := make(map[string]bool)
	for _, row := range rows {
		nodeSet[row.SrcAssetID] = true
		nodeSet[row.DstAssetID] = true
	}

	// Step 2: Build node list
	nodes := make([]CyNode, 0, len(nodeSet))
	for id := range nodeSet {
		nodes = append(nodes, CyNode{
			Data: CyNodeData{ID: id},
		})
	}

	// Step 3: Build edge list
	edges := make([]CyEdge, 0, len(rows))
	for _, row := range rows {
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

// AssetsListResponse wraps asset list for JSON response (REQ-021).
type AssetsListResponse struct {
	Assets   []AssetWithDetails `json:"assets"`
	Total    int                `json:"total"`
	Filtered int                `json:"filtered"`
}

// AssetWithDetails represents an asset with its type for the asset list.
type AssetWithDetails struct {
	AssetID  string `json:"asset_id"`
	TypeName string `json:"type_name"`
}

// BuildAssetsList creates the asset list response with count.
func BuildAssetsList(items []map[string]interface{}, totalCount int) AssetsListResponse {
	assets := make([]AssetWithDetails, 0, len(items))
	for _, item := range items {
		assets = append(assets, AssetWithDetails{
			AssetID:  item["asset_id"].(string),
			TypeName: item["type_name"].(string),
		})
	}
	return AssetsListResponse{
		Assets:   assets,
		Total:    totalCount,
		Filtered: len(items),
	}
}

// AssetDetailResponse wraps AssetDetail for JSON response (REQ-022).
// Just returns the AssetDetail as-is, structure already matches UI spec.
type AssetDetailResponse struct {
	Detail AssetDetail `json:"detail"`
}

// AssetDetail represents detailed information for a single asset.
type AssetDetail struct {
	AssetID  string `json:"asset_id"`
	TypeName string `json:"type_name"`
}

// BuildAssetDetailResponse maps AssetDetail for JSON response (REQ-022).
func BuildAssetDetailResponse(detail map[string]interface{}) AssetDetailResponse {
	return AssetDetailResponse{
		Detail: AssetDetail{
			AssetID:  detail["asset_id"].(string),
			TypeName: detail["type_name"].(string),
		},
	}
}

// NeighborsResponse wraps neighbors list for JSON response (REQ-023).
type NeighborsResponse struct {
	Neighbors []Neighbor `json:"neighbors"`
	Total     int        `json:"total"`
}

// Neighbor represents a neighbor asset.
type Neighbor struct {
	NeighborID string `json:"neighbor_id"`
}

// BuildNeighborsList wraps neighbors list for JSON response (REQ-023).
func BuildNeighborsList(neighbors []map[string]interface{}) NeighborsResponse {
	neighborList := make([]Neighbor, 0, len(neighbors))
	for _, n := range neighbors {
		neighborList = append(neighborList, Neighbor{
			NeighborID: n["neighbor_id"].(string),
		})
	}
	return NeighborsResponse{
		Neighbors: neighborList,
		Total:     len(neighbors),
	}
}

// AssetTypesResponse wraps asset types list for JSON response (REQ-024).
type AssetTypesResponse struct {
	AssetTypes []AssetTypeCount `json:"asset_types"`
	Total      int              `json:"total"`
}

// AssetTypeCount represents an asset type with count.
type AssetTypeCount struct {
	TypeName string `json:"type_name"`
	Count    int    `json:"count"`
}

// BuildAssetTypesList creates the asset types response with count.
func BuildAssetTypesList(types []map[string]interface{}) AssetTypesResponse {
	assetTypes := make([]AssetTypeCount, 0, len(types))
	for _, t := range types {
		assetTypes = append(assetTypes, AssetTypeCount{
			TypeName: t["type_name"].(string),
			Count:    t["count"].(int),
		})
	}
	return AssetTypesResponse{
		AssetTypes: assetTypes,
		Total:      len(types),
	}
}
