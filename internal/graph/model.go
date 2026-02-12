package graph

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

// BuildGraph takes raw rows and returns a CyGraph
