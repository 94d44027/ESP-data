package graph

import (
	"fmt"

	"ESP-data/internal/nebula"
)

type CyNode struct {
	Data map[string]string `json:"data"`
}

type CyEdge struct {
	Data map[string]string `json:"data"`
}

type CyGraph struct {
	Nodes []CyNode `json:"nodes"`
	Edges []CyEdge `json:"edges"`
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
