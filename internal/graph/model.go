package graph

import (
	"fmt"

	"ESP-data/internal/nebula"
)

// CyNode represents a Cytoscape.js node with enriched data fields (REQ-020, UI-REQ-200).
type CyNode struct {
	Data map[string]interface{} `json:"data"`
}

// CyEdge represents a Cytoscape.js edge with direction (REQ-012).
type CyEdge struct {
	Data map[string]string `json:"data"`
}

// CyGraph represents the complete graph structure for Cytoscape.js (REQ-122).
type CyGraph struct {
	Nodes []CyNode `json:"nodes"`
	Edges []CyEdge `json:"edges"`
}

// BuildGraph converts enriched Nebula query results into Cytoscape.js graph format.
// This satisfies:
// - REQ-020: Graph data with asset properties and types
// - REQ-122: JSON output
// - REQ-013: Asset_ID in node
// - REQ-012: Directed edges for arrowheads
// - UI-REQ-200: Node data fields for graph visualization
func BuildGraph(rows []nebula.AssetRow) CyGraph {
	// Step 1: Collect unique nodes with their properties
	// Use map to deduplicate nodes (same Asset_ID may appear as src/dst multiple times)
	nodeMap := make(map[string]map[string]interface{})

	for _, row := range rows {
		// Add source node if not already present
		if _, exists := nodeMap[row.SrcAssetID]; !exists {
			nodeMap[row.SrcAssetID] = map[string]interface{}{
				"id":                row.SrcAssetID,          // Cytoscape element ID
				"label":             row.SrcAssetName,        // Display label (REQ-013)
				"asset_id":          row.SrcAssetID,          // For filtering/search
				"asset_name":        row.SrcAssetName,        // Full name
				"asset_type":        row.SrcAssetType,        // From has_type relationship
				"is_entrance":       row.SrcIsEntrance,       // Boolean flag
				"is_target":         row.SrcIsTarget,         // Boolean flag
				"priority":          row.SrcPriority,         // Numeric priority (1-4)
				"has_vulnerability": row.SrcHasVulnerability, // Boolean flag
			}
		}

		// Add destination node if not already present
		if _, exists := nodeMap[row.DstAssetID]; !exists {
			nodeMap[row.DstAssetID] = map[string]interface{}{
				"id":                row.DstAssetID,
				"label":             row.DstAssetName,
				"asset_id":          row.DstAssetID,
				"asset_name":        row.DstAssetName,
				"asset_type":        row.DstAssetType,
				"is_entrance":       row.DstIsEntrance,
				"is_target":         row.DstIsTarget,
				"priority":          row.DstPriority,
				"has_vulnerability": row.DstHasVulnerability,
			}
		}
	}

	// Step 2: Convert map to CyNode array
	nodes := make([]CyNode, 0, len(nodeMap))
	for _, nodeData := range nodeMap {
		nodes = append(nodes, CyNode{Data: nodeData})
	}

	// Step 3: Build CyEdge array from rows
	edges := make([]CyEdge, 0, len(rows))
	for i, row := range rows {
		edges = append(edges, CyEdge{
			Data: map[string]string{
				"id":     fmt.Sprintf("e%d", i), // Unique edge ID
				"source": row.SrcAssetID,        // Direction: src -> dst
				"target": row.DstAssetID,        // (REQ-012: arrowhead direction)
				"label":  "connects_to",         // Edge type label
			},
		})
	}

	// Step 4: Return complete graph ready for JSON marshaling
	return CyGraph{
		Nodes: nodes,
		Edges: edges,
	}
}

// BuildAssetListResponse constructs the JSON response for /api/assets (REQ-021).
type AssetListResponse struct {
	Assets   []nebula.AssetListItem `json:"assets"`
	Total    int                    `json:"total"`
	Filtered int                    `json:"filtered"`
}

// BuildAssetList creates the asset list response with counts.
func BuildAssetList(items []nebula.AssetListItem, totalCount int) AssetListResponse {
	return AssetListResponse{
		Assets:   items,
		Total:    totalCount,
		Filtered: len(items),
	}
}

// BuildAssetDetailResponse wraps AssetDetail for JSON response (REQ-022).
// Just returns the AssetDetail as-is; structure already matches UI spec.
func BuildAssetDetailResponse(detail *nebula.AssetDetail) interface{} {
	return detail
}

// BuildNeighborsResponse wraps neighbor list for JSON response (REQ-023).
type NeighborsResponse struct {
	Neighbors []nebula.NeighborItem `json:"neighbors"`
	Total     int                   `json:"total"`
}

// BuildNeighborsList creates the neighbors response with count.
func BuildNeighborsList(neighbors []nebula.NeighborItem) NeighborsResponse {
	return NeighborsResponse{
		Neighbors: neighbors,
		Total:     len(neighbors),
	}
}

// BuildAssetTypesResponse wraps asset types for JSON response (REQ-024).
type AssetTypesResponse struct {
	AssetTypes []nebula.AssetTypeItem `json:"asset_types"`
	Total      int                    `json:"total"`
}

// BuildAssetTypesList creates the asset types response with count.
func BuildAssetTypesList(types []nebula.AssetTypeItem) AssetTypesResponse {
	return AssetTypesResponse{
		AssetTypes: types,
		Total:      len(types),
	}
}
