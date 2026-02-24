// ============================================
// INITIALIZATION
// ============================================
async function initialize() {
    try {
        console.log('ESP PoC: Initializing application...');

        // Load graph data and asset types in parallel
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

        console.log('ESP PoC: Initialization complete');
    } catch (error) {
        console.error('Initialization failed:', error);
        alert('Failed to load application data. Please check the console for details.');
    }
}

// ============================================
// EVENT LISTENERS
// ============================================
document.getElementById('btn-toggle-sidebar').addEventListener('click', toggleSidebar);
document.getElementById('btn-toggle-inspector').addEventListener('click', toggleInspector);
document.getElementById('btn-refresh').addEventListener('click', initialize);
document.getElementById('btn-path-inspector').addEventListener('click', openPathInspector);
document.getElementById('btn-path-close').addEventListener('click', closePathInspector);

document.getElementById('search-input').addEventListener('input', (e) => {
    AppState.searchTerm = e.target.value;
    applyFilters();
});

// Start application when page loads
window.addEventListener('DOMContentLoaded', initialize);
