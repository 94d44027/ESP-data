// ============================================
// GLOBAL STATE MANAGEMENT
// ============================================
const AppState = {
    cy: null,                    // Cytoscape instance
    selectedAssetId: null,       // Currently selected asset
    assetTypes: [],              // Available asset types
    activeFilters: new Set(),    // Active type filters
    searchTerm: '',              // Search filter
    allAssets: [],               // All assets from API
};
