// ============================================
// CYTOSCAPE GRAPH INITIALIZATION (UI-REQ-200)
// ============================================
function initCytoscape(graphData) {
    // Destroy existing instance if present
    if (AppState.cy) {
        AppState.cy.destroy();
    }

    // Initialize Cytoscape with graph data (REQ-300)
    AppState.cy = cytoscape({
        container: document.getElementById('cy'),
        elements: [...graphData.nodes, ...graphData.edges],

        // Graph Layout (UI-REQ-201)
        layout: {
            name: 'cose',
            animate: true,
            animationDuration: 500,
            nodeRepulsion: 8000,
            idealEdgeLength: 100,
            edgeElasticity: 100,
            nestingFactor: 1.2,
            gravity: 1,
            numIter: 1000,
            randomize: false
        },

        // Visual Styling (UI-REQ-203, UI-REQ-204)
        style: [
            // Node base styles
            {
                selector: 'node',
                style: {
                    'label': 'data(label)',
                    'text-valign': 'center',
                    'text-halign': 'center',
                    'font-size': '12px',
                    'color': '#e8eaed',
                    'text-outline-width': 2,
                    'text-outline-color': '#0a0e1a',
                    'width': 40,
                    'height': 40,
                    'border-width': 2,
                    'border-color': '#3d4556',
                    'shape': 'ellipse',
                }
            },

            // Node colors by type (UI-REQ-203)
            { selector: 'node[asset_type="Server"]', style: { 'background-color': '#5a9d5a' } },
            { selector: 'node[asset_type="Workstation"]', style: { 'background-color': '#4a9d9c' } },
            { selector: 'node[asset_type="Network Device"]', style: { 'background-color': '#5a7fbf' } },
            { selector: 'node[asset_type="Application"]', style: { 'background-color': '#10b981' } },
            { selector: 'node[asset_type="Database"]', style: { 'background-color': '#f59e0b' } },
            { selector: 'node[asset_type="Mobile Device"]', style: { 'background-color': '#9437FF' } },
            { selector: 'node[asset_type="IoT Device"]', style: { 'background-color': '#BE5014' } },

            // Priority border colors (UI-REQ-204)
            { selector: 'node[priority=1]', style: { 'border-color': '#ef4444', 'border-width': 3 } },
            { selector: 'node[priority=2]', style: { 'border-color': '#f59e0b', 'border-width': 3 } },
            { selector: 'node[priority=3]', style: { 'border-color': '#eab308', 'border-width': 2 } },
            { selector: 'node[priority=4]', style: { 'border-color': '#3b82f6', 'border-width': 2 } },

            // Special markers (UI-REQ-205)
            { selector: 'node[?is_entrance]', style: { 'shape': 'triangle' } },
            { selector: 'node[?is_target]', style: { 'shape': 'star' } },
            { selector: 'node[?has_vulnerability]', style: { 'border-style': 'dashed' } },

            // Selected node highlight
            {
                selector: 'node:selected',
                style: {
                    'border-color': '#4a9eff',
                    'border-width': 4,
                    'overlay-color': '#4a9eff',
                    'overlay-opacity': 0.2,
                    'overlay-padding': 8
                }
            },

            // Edge styles (REQ-012: directed with arrowheads)
            {
                selector: 'edge',
                style: {
                    'width': 2,
                    'line-color': '#3d4556',
                    'target-arrow-color': '#3d4556',
                    'target-arrow-shape': 'triangle',
                    'curve-style': 'bezier',
                    'arrow-scale': 1.2
                }
            },

            // Selected edge highlight
            {
                selector: 'edge:selected',
                style: {
                    'line-color': '#ff9933',
                    'target-arrow-color': '#ff9933',
                    'width': 3
                }
            }
        ]
    });

    // Node click handler — open asset inspector
    AppState.cy.on('tap', 'node', function(evt) {
        const nodeId = evt.target.id();
        selectAsset(nodeId);
    });

    // Edge click handler — open edge inspector (UI-REQ-330)
    AppState.cy.on('tap', 'edge', function(evt) {
        const sourceId = evt.target.data('source');
        const targetId = evt.target.data('target');
        selectEdge(sourceId, targetId);
    });

    // Click on background - deselect
    AppState.cy.on('tap', function(evt) {
        if (evt.target === AppState.cy) {
            deselectAsset();
        }
    });

    // Update stats
    updateStats();
}
