// ============================================
// PATH INSPECTOR — Side Panel
// (UI-REQ-206, UI-REQ-207, UI-REQ-208, UI-REQ-209)
// ============================================

// Open the Path Inspector side panel and populate dropdowns (UI-REQ-206)
async function openPathInspector() {
    const panel = document.getElementById('path-inspector-panel');
    const inspector = document.getElementById('inspector');
    const canvas = document.getElementById('main-canvas');

    // Show panel and adjust layout
    panel.classList.add('active');
    inspector.classList.add('path-inspector-open');
    canvas.classList.add('path-inspector-open');

    // Reset state
    document.getElementById('path-results-body').innerHTML = '';
    document.getElementById('path-results-section').style.display = 'none';
    document.getElementById('path-status').textContent = '';
    document.getElementById('path-hops').value = '6';

    // Populate dropdowns in parallel (UI-REQ-207 §1, §2)
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
}

// Run path calculation (UI-REQ-207 §4)
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

    try {
        const data = await API.fetchPaths(fromId, toId, hops);

        if (data.total === 0) {
            statusEl.textContent = 'No paths found between selected assets.';
            return;
        }

        statusEl.textContent = `Found ${data.total} path(s)`;
        resultsSection.style.display = 'block';

        // Render results table (UI-REQ-207 §5)
        resultsBody.innerHTML = data.paths.map(path => `
            <tr class="path-row" onclick="highlightPath('${path.hosts}')">
                <td>${path.path_id}</td>
                <td class="path-hosts">${path.hosts}</td>
                <td class="path-tta">${path.tta}</td>
            </tr>
        `).join('');

    } catch (error) {
        console.error('Path calculation failed:', error);
        statusEl.textContent = 'Path calculation failed. Check console for details.';
    }
}

// Highlight a path on the Cytoscape graph (UI-REQ-208)
function highlightPath(hostsStr) {
    if (!AppState.cy) return;

    // Clear previous highlights
    clearPathHighlights();

    // Parse "A00001 -> A00003 -> A00007" into array of asset IDs
    const assetIds = hostsStr.split(' -> ').map(s => s.trim()).filter(Boolean);

    // Highlight nodes
    assetIds.forEach(id => {
        const node = AppState.cy.nodes(`#${id}`);
        if (node.length > 0) {
            node.addClass('path-highlighted');
        }
    });

    // Highlight edges between consecutive nodes
    for (let i = 0; i < assetIds.length - 1; i++) {
        const edge = AppState.cy.edges(`[source="${assetIds[i]}"][target="${assetIds[i + 1]}"]`);
        if (edge.length > 0) {
            edge.addClass('path-highlighted');
        }
    }

    // Fit view to highlighted elements
    const highlighted = AppState.cy.elements('.path-highlighted');
    if (highlighted.length > 0) {
        AppState.cy.animate({
            fit: { eles: highlighted, padding: 50 }
        }, {
            duration: 400
        });
    }
}

// Clear path highlights from graph (UI-REQ-209)
function clearPathHighlights() {
    if (!AppState.cy) return;
    AppState.cy.elements().removeClass('path-highlighted');
}
