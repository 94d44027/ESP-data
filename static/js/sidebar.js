// ============================================
// ASSET LIST RENDERING (UI-REQ-120)
// ============================================
function renderAssetList(assets) {
    const listContainer = document.getElementById('asset-list');

    if (assets.length === 0) {
        listContainer.innerHTML = '<div class="inspector-empty">No assets found</div>';
        return;
    }

    listContainer.innerHTML = assets.map(asset => `
        <div class="asset-item ${asset.asset_id === AppState.selectedAssetId ? 'selected' : ''}"
             data-asset-id="${asset.asset_id}"
             onclick="selectAsset('${asset.asset_id}')">
            <div class="asset-item-header">
                <span class="asset-item-id">${asset.asset_id}</span>
            </div>
            <div class="asset-item-name">${asset.asset_name || ''}</div>
            <div class="asset-item-badges">
                ${asset.is_entrance ? '<span class="badge badge-entrance">Entrance</span>' : ''}
                ${asset.is_target ? '<span class="badge badge-target">Target</span>' : ''}
                ${asset.has_vulnerability ? '<span class="badge badge-vuln">Vuln</span>' : ''}
                ${asset.priority ? `<span class="badge badge-priority-${asset.priority}">P${asset.priority}</span>` : ''}
            </div>
        </div>
    `).join('');
}

// ============================================
// ASSET TYPE FILTERS (UI-REQ-122)
// ============================================
function renderAssetTypeFilters(assetTypes) {
    const filterContainer = document.getElementById('filter-types');

    // Compute per-type counts from the already-loaded asset list
    const counts = {};
    (AppState.allAssets || []).forEach(a => {
        const t = a.asset_type || 'Unknown';
        counts[t] = (counts[t] || 0) + 1;
    });

    filterContainer.innerHTML = assetTypes.map(type => `
        <div class="filter-checkbox">
            <input type="checkbox"
                   id="filter-${type.type_name.replace(/\s+/g, '-')}"
                   value="${type.type_name}"
                   onchange="toggleTypeFilter('${type.type_name}')">
            <label for="filter-${type.type_name.replace(/\s+/g, '-')}">${type.type_name}</label>
            <span class="filter-count">${counts[type.type_name] || 0}</span>
        </div>
    `).join('');
}

function toggleTypeFilter(typeName) {
    if (AppState.activeFilters.has(typeName)) {
        AppState.activeFilters.delete(typeName);
    } else {
        AppState.activeFilters.add(typeName);
    }

    applyFilters();
}

// ============================================
// SEARCH FUNCTIONALITY (UI-REQ-111)
// ============================================
function applyFilters() {
    let filtered = AppState.allAssets;

    // Apply type filters
    if (AppState.activeFilters.size > 0) {
        filtered = filtered.filter(asset =>
            AppState.activeFilters.has(asset.asset_type)
        );
    }

    // Apply search filter
    if (AppState.searchTerm) {
        const term = AppState.searchTerm.toLowerCase();
        filtered = filtered.filter(asset =>
            asset.asset_id.toLowerCase().includes(term) ||
            (asset.asset_name || '').toLowerCase().includes(term)
        );
    }

    renderAssetList(filtered);
}

// ============================================
// SIDEBAR AUTO-FOCUS ON GRAPH SELECTION (UI-REQ-124)
// ============================================
function focusAssetInList(assetId) {
    // If sidebar is collapsed, do nothing (UI-REQ-124: closed sidebar)
    const sidebar = document.getElementById('sidebar');
    if (sidebar.classList.contains('collapsed')) {
        return;
    }

    // Remove previous selection highlight from all items
    document.querySelectorAll('.asset-item').forEach(item => {
        item.classList.remove('selected');
    });

    // Find the matching asset item in the DOM
    const targetItem = document.querySelector(`.asset-item[data-asset-id="${assetId}"]`);
    if (!targetItem) {
        return; // Asset not in current filtered list
    }

    // Apply selected highlight
    targetItem.classList.add('selected');

    // Smooth-scroll into view (block: 'nearest' avoids unnecessary scroll
    // when item is already visible)
    targetItem.scrollIntoView({ behavior: 'smooth', block: 'nearest' });
}