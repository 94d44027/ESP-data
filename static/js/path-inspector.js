// ============================================
// PATH INSPECTOR — Side Panel
// (UI-REQ-206, UI-REQ-207, UI-REQ-208, UI-REQ-209, UI-REQ-113)
// ============================================

// Open the Path Inspector side panel and populate dropdowns (UI-REQ-206)
async function openPathInspector() {
    const panel = document.getElementById('path-inspector-panel');
    const inspector = document.getElementById('inspector');
    const canvas = document.getElementById('main-canvas');

    panel.classList.add('active');
    inspector.classList.add('path-inspector-open');
    canvas.classList.add('path-inspector-open');

    document.getElementById('path-results-body').innerHTML = '';
    document.getElementById('path-results-section').style.display = 'none';
    document.getElementById('path-status').textContent = '';
    document.getElementById('path-hops').value = '6';
    hideRecalcWarning();

    try {
        const [entryData, targetData] = await Promise.all([
            API.fetchEntryPoints(),
            API.fetchTargets()
        ]);

        const fromSelect = document.getElementById('path-from');
        fromSelect.innerHTML = '<option value="">Select entry point...</option>' +
            entryData.entry_points.map(ep =>
                `<option value="${ep.asset_id}">${ep.asset_id} — ${ep.asset_name}</option>`
            ).join('');

        const toSelect = document.getElementById('path-to');
        toSelect.innerHTML = '<option value="">Select target...</option>' +
            targetData.targets.map(t =>
                `<option value="${t.asset_id}">${t.asset_id} — ${t.asset_name}</option>`
            ).join('');
    } catch (error) {
        console.error('Failed to populate Path Inspector dropdowns:', error);
        document.getElementById('path-status').textContent = 'Failed to load dropdowns';
    }
}

// Close the Path Inspector side panel and restore layout (UI-REQ-209)
function closePathInspector() {
    const panel = document.getElementById('path-inspector-panel');
    const inspector = document.getElementById('inspector');
    const canvas = document.getElementById('main-canvas');

    panel.classList.remove('active');
    inspector.classList.remove('path-inspector-open');
    canvas.classList.remove('path-inspector-open');

    clearPathHighlights();
    hideRecalcWarning();
}

// Run path calculation with path-scoped recalculation (UI-REQ-207 sec 4, ALG-REQ-046)
async function runPathCalculation() {
    const fromId = document.getElementById('path-from').value;
    const toId = document.getElementById('path-to').value;
    const hops = document.getElementById('path-hops').value;

    if (!fromId || !toId) {
        document.getElementById('path-status').textContent = 'Please select both entry point and target';
        return;
    }

    const statusEl = document.getElementById('path-status');
    const resultsSection = document.getElementById('path-results-section');
    const resultsBody = document.getElementById('path-results-body');

    statusEl.textContent = 'Calculating paths...';
    resultsBody.innerHTML = '';
    resultsSection.style.display = 'none';
    hideRecalcWarning();

    try {
        const data = await API.fetchPaths(fromId, toId, hops);

        if (data.total === 0) {
            statusEl.textContent = 'No paths found between selected assets.';
            return;
        }

        statusEl.textContent = `Found ${data.total} path(s)`;
        resultsSection.style.display = 'block';

        resultsBody.innerHTML = data.paths.map(path => `
            <tr class="path-row" onclick="highlightPath('${path.hosts}')">
                <td>${path.path_id}</td>
                <td class="path-hosts">${path.hosts}</td>
                <td class="path-tta">${path.tta}</td>
            </tr>
        `).join('');

        // Show stale path warning if recalculation occurred (UI-REQ-113)
        if (data.recalculated_assets && data.recalculated_assets.length > 0) {
            showRecalcWarning(data.recalculated_assets);
            await refreshSystemState();
        }

    } catch (error) {
        console.error('Path calculation failed:', error);
        statusEl.textContent = 'Path calculation failed. Check console for details.';
    }
}

// Show recalculation warning below results (UI-REQ-113)
function showRecalcWarning(assets) {
    const el = document.getElementById('path-recalc-warning');
    if (!el) return;
    el.style.display = 'flex';
    el.title = `TTB was recalculated for ${assets.length} asset(s): ${assets.join(', ')}`;
    el.querySelector('.recalc-warning-text').textContent =
        `\u26A0\uFE0F TTB recalculated for ${assets.length} asset(s) during path calculation`;
}

function hideRecalcWarning() {
    const el = document.getElementById('path-recalc-warning');
    if (el) el.style.display = 'none';
}

// Highlight a path on the Cytoscape graph (UI-REQ-208)
function highlightPath(hostsStr) {
    if (!AppState.cy) return;
    clearPathHighlights();

    const assetIds = hostsStr.split(' -> ').map(s => s.trim()).filter(Boolean);

    assetIds.forEach(id => {
        const node = AppState.cy.nodes(`#${id}`);
        if (node.length > 0) node.addClass('path-highlighted');
    });

    for (let i = 0; i < assetIds.length - 1; i++) {
        const edge = AppState.cy.edges(`[source="${assetIds[i]}"][target="${assetIds[i + 1]}"]`);
        if (edge.length > 0) edge.addClass('path-highlighted');
    }

    const highlighted = AppState.cy.elements('.path-highlighted');
    if (highlighted.length > 0) {
        AppState.cy.animate({ fit: { eles: highlighted, padding: 50 } }, { duration: 400 });
    }
}

// Clear path highlights from graph (UI-REQ-209)
function clearPathHighlights() {
    if (!AppState.cy) return;
    AppState.cy.elements().removeClass('path-highlighted');
}
