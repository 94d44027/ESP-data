// ============================================
// INSPECTOR PANEL (UI-REQ-210)
// ============================================

// Cached detail for the currently displayed asset (used by header button)
let _inspectorDetail = null;

async function selectAsset(assetId) {
    AppState.selectedAssetId = assetId;

    // Highlight node in graph
    if (AppState.cy) {
        AppState.cy.nodes().unselect();
        const node = AppState.cy.nodes(`#${assetId}`);
        if (node.length > 0) {
            node.select();
            AppState.cy.animate({
                center: { eles: node },
                zoom: 1.5
            }, {
                duration: 300
            });
        }
    }

    // Scroll to and highlight asset in sidebar list (UI-REQ-124)
    focusAssetInList(assetId);

    // Show inspector with loading state
    const inspectorContent = document.getElementById('inspector-content');
    inspectorContent.innerHTML = '<div class="loading"></div>';

    try {
        const [detail, neighborsData] = await Promise.all([
            API.fetchAssetDetail(assetId),
            API.fetchNeighbors(assetId)
        ]);

        _inspectorDetail = detail;
        renderInspector(detail, neighborsData.neighbors);

        // Show the mitigations icon button in the header (UI-REQ-210 §5)
        const mitBtn = document.getElementById('btn-edit-mitigations');
        if (mitBtn) mitBtn.style.display = '';
    } catch (error) {
        console.error('Failed to load asset details:', error);
        inspectorContent.innerHTML = `
            <div class="inspector-empty" style="color: var(--color-danger);">
                Failed to load asset details
            </div>
        `;
    }
}

function deselectAsset() {
    AppState.selectedAssetId = null;
    _inspectorDetail = null;

    if (AppState.cy) {
        AppState.cy.nodes().unselect();
    }

    document.querySelectorAll('.asset-item').forEach(item => {
        item.classList.remove('selected');
    });

    // Hide the mitigations icon button
    const mitBtn = document.getElementById('btn-edit-mitigations');
    if (mitBtn) mitBtn.style.display = 'none';

    document.getElementById('inspector-content').innerHTML = `
        <div class="inspector-empty">Select a node to view details</div>
    `;
}

function renderInspector(detail, neighbors) {
    const inspectorContent = document.getElementById('inspector-content');
    inspectorContent.innerHTML = `
        <!-- Basic Info (UI-REQ-211) -->
        <div class="property-section">
            <div class="property-section-title">Basic Information</div>
            <div class="property-list">
                <div class="property-item">
                    <div class="property-label">Asset ID</div>
                    <div class="property-value">${detail.asset_id}</div>
                </div>
                <div class="property-item">
                    <div class="property-label">Name</div>
                    <div class="property-value">${detail.asset_name}</div>
                </div>
                <div class="property-item">
                    <div class="property-label">Type</div>
                    <div class="property-value">${detail.asset_type || 'Unknown'}</div>
                </div>
                <div class="property-item">
                    <div class="property-label">Priority</div>
                    <div class="property-value">
                        <span class="badge badge-priority-${detail.priority}">
                            Priority ${detail.priority}
                        </span>
                    </div>
                </div>
            </div>
        </div>

        <!-- Flags Section -->
        <div class="property-section">
            <div class="property-section-title">Security Flags</div>
            <div class="property-list">
                <div class="property-item">
                    <div class="property-label">Entrance Point</div>
                    <div class="property-value">
                        ${detail.is_entrance ?
                            '<span class="badge badge-entrance">Yes</span>' :
                            '<span style="color: var(--color-text-muted);">No</span>'}
                    </div>
                </div>
                <div class="property-item">
                    <div class="property-label">Target Asset</div>
                    <div class="property-value">
                        ${detail.is_target ?
                            '<span class="badge badge-target">Yes</span>' :
                            '<span style="color: var(--color-text-muted);">No</span>'}
                    </div>
                </div>
                <div class="property-item">
                    <div class="property-label">Has Vulnerability</div>
                    <div class="property-value">
                        ${detail.has_vulnerability ?
                            '<span class="badge badge-vuln">Yes</span>' :
                            '<span style="color: var(--color-text-muted);">No</span>'}
                    </div>
                </div>
            </div>
        </div>

        <!-- Connections (UI-REQ-212) -->
        <div class="property-section">
            <div class="property-section-title">
                Connections (${neighbors.length})
            </div>
            <div class="connection-list">
                ${neighbors.length === 0 ?
                    '<div class="inspector-empty">No connections</div>' :
                    neighbors.map(neighbor => `
                        <div class="connection-item" onclick="selectAsset('${neighbor.neighbor_id}')">
                            <div class="connection-direction">
                                ${neighbor.direction === 'outbound' ? '\u2192 Outbound' : '\u2190 Inbound'}
                            </div>
                            <div class="connection-target">
                                ${neighbor.neighbor_id}
                            </div>
                        </div>
                    `).join('')
                }
            </div>
        </div>
    `;
}

// Wire the header icon button to MitEditor (UI-REQ-210 §5)
document.getElementById('btn-edit-mitigations').addEventListener('click', () => {
    if (_inspectorDetail) {
        MitEditor.open(
            _inspectorDetail.asset_id,
            _inspectorDetail.asset_name || '',
            _inspectorDetail.asset_description || ''
        );
    }
});
