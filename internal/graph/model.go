package graph

import (
	"fmt"

	"ESP-data/internal/nebula"
)

// CyNode represents a node in Cytoscape.js format
type CyNode struct {
	Data map[string]string `json:"data"`
}

// CyEdge represents an edge in Cytoscape.js format
type CyEdge struct {
	Data map[string]string `json:"data"`
}

// CyGraph represents the complete graph for Cytoscape.js
type CyGraph struct {
	Nodes []CyNode `json:"nodes"`
	Edges []CyEdge `json:"edges"`
}

// AssetsListResponse wraps asset list for JSON response (REQ-021)
type AssetsListResponse struct {
	Assets   []nebula.AssetWithDetails `json:"assets"`
	Total    int                       `json:"total"`
	Filtered int                       `json:"filtered"`
}

// AssetDetailResponse wraps AssetDetail for JSON response (REQ-022)
type AssetDetailResponse struct {
	Detail nebula.AssetDetail `json:"detail"`
}

// NeighborsResponse wraps neighbors list for JSON response (REQ-023)
type NeighborsResponse struct {
	Neighbors []nebula.Neighbor `json:"neighbors"`
	Total     int               `json:"total"`
}

// AssetTypesResponse wraps asset types for JSON response (REQ-024)
type AssetTypesResponse struct {
	AssetTypes []nebula.AssetTypeCount `json:"asset_types"`
	Total      int                     `json:"total"`
}

// BuildGraph converts Nebula query results into Cytoscape.js graph format.
// This satisfies REQ-122 (JSON output), REQ-013 (Asset_ID in node),
// and REQ-012 (directed edges for arrowheads).
func BuildGraph(rows []nebula.AssetRow) CyGraph {
	// Step 1: Collect unique node IDs
	nodeSet := make(map[string]bool)
	for _, row := range rows {
		nodeSet[row.SrcAssetID] = true
		nodeSet[row.DstAssetID] = true
	}

	// Step 2: Build CyNode array
	nodes := make([]CyNode, 0, len(nodeSet))
	for assetID := range nodeSet {
		nodes = append(nodes, CyNode{
			Data: map[string]string{
				"id":    assetID, // Cytoscape element ID
				"label": assetID, // Display label (REQ-013: Asset ID fits in node)
			},
		})
	}

	// Step 3: Build CyEdge array
	edges := make([]CyEdge, 0, len(rows))
	for i, row := range rows {
		edges = append(edges, CyEdge{
			Data: map[string]string{
				"id":     fmt.Sprintf("e%d", i), // Unique edge ID
				"source": row.SrcAssetID,        // Direction: src -> dst
				"target": row.DstAssetID,        // (REQ-012: arrowhead direction)
			},
		})
	}

	// Step 4: Return complete graph ready for JSON marshaling
	return CyGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

// BuildAssetsList creates the asset list response with count.
func BuildAssetsList(items []nebula.AssetWithDetails, totalCount int) AssetsListResponse {
	return AssetsListResponse{
		Assets:   items,
		Total:    totalCount,
		Filtered: len(items),
	}
}

// BuildAssetDetailResponse wraps AssetDetail for JSON response (REQ-022).
// Just returns the AssetDetail as-is, structure already matches UI spec.
func BuildAssetDetailResponse(detail *nebula.AssetDetail) AssetDetailResponse {
	return AssetDetailResponse{
		Detail: *detail,
	}
}

// BuildNeighborsList wraps neighbors list for JSON response (REQ-023).
func BuildNeighborsList(neighbors []nebula.Neighbor) NeighborsResponse {
	return NeighborsResponse{
		Neighbors: neighbors,
		Total:     len(neighbors),
	}
}

// BuildAssetTypesList creates the asset types response with count.
func BuildAssetTypesList(types []nebula.AssetTypeCount) AssetTypesResponse {
	return AssetTypesResponse{
		AssetTypes: types,
		Total:      len(types),
	}
}
