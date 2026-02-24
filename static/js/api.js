// ============================================
// API CLIENT (REQ-020 through REQ-031)
// ============================================
const API = {
    // REQ-020: Fetch graph data for visualization
    async fetchGraph() {
        const response = await fetch('/api/graph');
        if (!response.ok) throw new Error('Failed to fetch graph data');
        return await response.json();
    },

    // REQ-021: Fetch asset list for sidebar
    async fetchAssets(type = '', search = '') {
        const params = new URLSearchParams();
        if (type) params.append('type', type);
        if (search) params.append('search', search);

        const url = `/api/assets${params.toString() ? '?' + params.toString() : ''}`;
        const response = await fetch(url);
        if (!response.ok) throw new Error('Failed to fetch assets');
        return await response.json();
    },

    // REQ-022: Fetch single asset detail
    async fetchAssetDetail(assetId) {
        const response = await fetch(`/api/asset/${assetId}`);
        if (!response.ok) throw new Error('Failed to fetch asset detail');
        return await response.json();
    },

    // REQ-023: Fetch neighbors for connections list
    async fetchNeighbors(assetId) {
        const response = await fetch(`/api/neighbors/${assetId}`);
        if (!response.ok) throw new Error('Failed to fetch neighbors');
        return await response.json();
    },

    // REQ-026: Fetch edge connections for edge inspector
    async fetchEdges(sourceId, targetId) {
        const response = await fetch(`/api/edges/${sourceId}/${targetId}`);
        if (!response.ok) throw new Error('Failed to fetch edge connections');
        return await response.json();
    },

    // REQ-024: Fetch asset types for filters
    async fetchAssetTypes() {
        const response = await fetch('/api/asset-types');
        if (!response.ok) throw new Error('Failed to fetch asset types');
        return await response.json();
    },

    // REQ-030: Fetch entry points for Path Inspector dropdown
    async fetchEntryPoints() {
        const response = await fetch('/api/entry-points');
        if (!response.ok) throw new Error('Failed to fetch entry points');
        return await response.json();
    },

    // REQ-031: Fetch targets for Path Inspector dropdown
    async fetchTargets() {
        const response = await fetch('/api/targets');
        if (!response.ok) throw new Error('Failed to fetch targets');
        return await response.json();
    },

    // REQ-029: Calculate paths between entry and target
    async fetchPaths(fromId, toId, hops = 6) {
        const response = await fetch(`/api/paths?from=${fromId}&to=${toId}&hops=${hops}`);
        if (!response.ok) throw new Error('Failed to calculate paths');
        return await response.json();
    }
};
