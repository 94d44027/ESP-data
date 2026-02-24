// ============================================
// UI CONTROLS
// ============================================
function toggleSidebar() {
    const sidebar = document.getElementById('sidebar');
    const canvas = document.getElementById('main-canvas');

    sidebar.classList.toggle('collapsed');
    canvas.classList.toggle('sidebar-collapsed');
}

function toggleInspector() {
    const inspector = document.getElementById('inspector');
    const canvas = document.getElementById('main-canvas');

    inspector.classList.toggle('collapsed');
    canvas.classList.toggle('inspector-collapsed');
}

function updateStats() {
    if (AppState.cy) {
        document.getElementById('stat-nodes').textContent = AppState.cy.nodes().length;
        document.getElementById('stat-edges').textContent = AppState.cy.edges().length;
    }
}
