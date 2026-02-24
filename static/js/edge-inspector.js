// ============================================
// EDGE INSPECTOR (UI-REQ-212, UI-REQ-330)
// ============================================
async function selectEdge(sourceId, targetId) {
    AppState.selectedAssetId = null;

    // Deselect nodes, keep edge selected
    if (AppState.cy) {
        AppState.cy.nodes().unselect();
    }
    document.querySelectorAll('.asset-item').forEach(item => {
        item.classList.remove('selected');
    });

    // Show inspector with loading state
    const inspectorContent = document.getElementById('inspector-content');
    inspectorContent.innerHTML = '<div class="loading"></div>';

    try {
        const edgeData = await API.fetchEdges(sourceId, targetId);
        renderEdgeInspector(edgeData);
    } catch (error) {
        console.error('Failed to load edge details:', error);
        inspectorContent.innerHTML = `
            <div class="inspector-empty" style="color: var(--color-danger);">
                Failed to load edge details
            </div>
        `;
    }
}

function renderEdgeInspector(edgeData) {
    const inspectorContent = document.getElementById('inspector-content');
    inspectorContent.innerHTML = `
        <!-- Source Asset Block (UI-REQ-212 §1) -->
        <div class="property-section">
            <div class="edge-asset-block">
                <div class="edge-asset-label">Source</div>
                <div class="edge-asset-name" onclick="selectAsset('${edgeData.source.asset_id}')">
                    ${edgeData.source.asset_name || edgeData.source.asset_id}
                </div>
                <div class="edge-asset-id">(${edgeData.source.asset_id})</div>
                ${edgeData.source.asset_description ?
                    `<div class="edge-asset-desc">${edgeData.source.asset_description}</div>` : ''}
            </div>
        </div>

        <!-- Target Asset Block (UI-REQ-212 §2) -->
        <div class="property-section">
            <div class="edge-asset-block">
                <div class="edge-asset-label">Target</div>
                <div class="edge-asset-name" onclick="selectAsset('${edgeData.target.asset_id}')">
                    ${edgeData.target.asset_name || edgeData.target.asset_id}
                </div>
                <div class="edge-asset-id">(${edgeData.target.asset_id})</div>
                ${edgeData.target.asset_description ?
                    `<div class="edge-asset-desc">${edgeData.target.asset_description}</div>` : ''}
            </div>
        </div>

        <!-- Connections Table (UI-REQ-212 §3) -->
        <div class="property-section">
            <div class="property-section-title">
                Connections (${edgeData.total})
            </div>
            <table class="edge-connections-table">
                <thead>
                    <tr><th>Protocol</th><th>Port</th></tr>
                </thead>
                <tbody>
                    ${edgeData.connections.map(conn => `
                        <tr>
                            <td>${conn.connection_protocol}</td>
                            <td>${conn.connection_port}</td>
                        </tr>
                    `).join('')}
                </tbody>
            </table>
        </div>
    `;
}
