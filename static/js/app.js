// ============================================
// INITIALIZATION
// ============================================
async function initialize() {
    try {
        console.log('ESP PoC: Initializing application...');

        // Load graph data, asset types, and assets in parallel
        const [graphData, assetTypesData, assetsData] = await Promise.all([
            API.fetchGraph(),
            API.fetchAssetTypes(),
            API.fetchAssets()
        ]);

        console.log('Loaded:', graphData.nodes.length, 'nodes,', graphData.edges.length, 'edges');

        // Initialize Cytoscape graph
        initCytoscape(graphData);

        // Store asset types and all assets
        AppState.assetTypes = assetTypesData.asset_types;
        AppState.allAssets = assetsData.assets;

        // Render UI components
        renderAssetTypeFilters(AppState.assetTypes);
        renderAssetList(AppState.allAssets);

        // Fetch system state for stale badge (UI-REQ-112)
        await refreshSystemState();

        console.log('ESP PoC: Initialization complete');
    } catch (error) {
        console.error('Initialization failed:', error);
        alert('Failed to load application data. Please check the console for details.');
    }
}

// ============================================
// SYSTEM STATE AND TTB RECALCULATION (UI-REQ-112)
// ============================================

// Refresh the stale-count badge from SystemState (ALG-REQ-048)
async function refreshSystemState() {
    try {
        const state = await API.fetchSystemState();
        AppState.staleCount = state.stale_count || 0;
        updateStaleBadge();
    } catch (err) {
        console.error('Failed to fetch system state:', err);
    }
}

// Update the stale-count badge on the Recalculate button
function updateStaleBadge() {
    const badge = document.getElementById('stale-badge');
    if (!badge) return;
    if (AppState.staleCount > 0) {
        badge.textContent = AppState.staleCount;
        badge.style.display = 'flex';
    } else {
        badge.style.display = 'none';
    }
}

// Handle Recalculate TTBs button click (UI-REQ-112)
async function handleRecalculateTTB() {
    const btn = document.getElementById('btn-recalculate-ttb');
    if (!btn) return;

    // Disable button during request
    btn.disabled = true;
    btn.classList.add('btn-loading');

    try {
        const result = await API.recalculateTTB();
        showToast(
            `Recalculated TTB for ${result.recalculated} asset(s). ${result.unchanged} unchanged.`,
            'success'
        );
    } catch (err) {
        console.error('TTB recalculation failed:', err);
        showToast('TTB recalculation failed', 'error');
    } finally {
        btn.disabled = false;
        btn.classList.remove('btn-loading');
        await refreshSystemState();
    }
}

// Toast notification (UI-REQ-112 sec 3, sec 4)
function showToast(message, type = 'success') {
    const existing = document.getElementById('esp-toast');
    if (existing) existing.remove();

    const toast = document.createElement('div');
    toast.id = 'esp-toast';
    toast.className = `esp-toast esp-toast-${type}`;
    toast.textContent = message;
    document.body.appendChild(toast);

    requestAnimationFrame(() => toast.classList.add('esp-toast-visible'));

    setTimeout(() => {
        toast.classList.remove('esp-toast-visible');
        setTimeout(() => toast.remove(), 300);
    }, 3000);
}

// ============================================
// EVENT LISTENERS
// ============================================
document.getElementById('btn-toggle-sidebar').addEventListener('click', toggleSidebar);
document.getElementById('btn-toggle-inspector').addEventListener('click', toggleInspector);
document.getElementById('btn-refresh').addEventListener('click', initialize);
document.getElementById('btn-path-inspector').addEventListener('click', openPathInspector);
document.getElementById('btn-path-close').addEventListener('click', closePathInspector);
document.getElementById('btn-recalculate-ttb').addEventListener('click', handleRecalculateTTB);

document.getElementById('search-input').addEventListener('input', (e) => {
    AppState.searchTerm = e.target.value;
    applyFilters();
});

// Start application when page loads
window.addEventListener('DOMContentLoaded', initialize);
