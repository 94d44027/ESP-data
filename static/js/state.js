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
    staleCount: 0,               // Assets with stale hashes (from SystemState)

    // TTB Calculation Parameters (UI-REQ-2091) — persist within page session
    ttbParams: {
        orientationTime: 15,     // minutes (default per ALG-REQ-071: 0.25h = 15min)
        switchoverTime: 10,      // minutes (default per ALG-REQ-072: ~0.1667h ≈ 10min)
        priorityTolerance: 1     // ALG-REQ-075 default
    }
};
