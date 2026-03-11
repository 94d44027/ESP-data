// ============================================
// PATH INSPECTOR — Side Panel
// (UI-REQ-206, UI-REQ-207, UI-REQ-208, UI-REQ-209, UI-REQ-113, UI-REQ-2091)
// ============================================

// Format TTA float (hours) to hhh:mm:ss display (UI-REQ-207 §5)
function formatTTA(hours) {
    if (hours == null || isNaN(hours)) return '—';
    const totalSeconds = Math.round(hours * 3600);
    const h = Math.floor(totalSeconds / 3600);
    const m = Math.floor((totalSeconds % 3600) / 60);
    const s = totalSeconds % 60;
    return `${h}:${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`;
}

// Check if TTB params differ from defaults (UI-REQ-2091)
function ttbParamsAreDefault() {
    const p = AppState.ttbParams;
    return p.orientationTime === 15 && p.switchoverTime === 10 && p.priorityTolerance === 1;
}

// Update the collapsed header label to show "(defaults)" or "(custom)"
function updateCalcParamsLabel() {
    const label = document.getElementById('calc-params-label');
    if (label) {
        label.textContent = ttbParamsAreDefault() ? '(defaults)' : '(custom)';
    }
}

// Reset TTB params to defaults (UI-REQ-2091)
function resetCalcParamsToDefaults() {
    AppState.ttbParams = { orientationTime: 15, switchoverTime: 10, priorityTolerance: 1 };
    document.getElementById('param-orientation').value = '15';
    document.getElementById('param-switchover').value = '10';
    document.getElementById('param-priority').value = '1';
    updateCalcParamsLabel();
}

// Toggle the calc params collapsible section
function toggleCalcParams() {
    const section = document.getElementById('calc-params-content');
    const toggle = document.getElementById('calc-params-toggle');
    if (!section || !toggle) return;
    const isOpen = section.style.display !== 'none';
    section.style.display = isOpen ? 'none' : 'block';
    toggle.textContent = isOpen ? '▶' : '▼';
}

// Read current param values from inputs into AppState
function syncCalcParams() {
    const orient = parseFloat(document.getElementById('param-orientation').value);
    const switchT = parseFloat(document.getElementById('param-switchover').value);
    const priTol = parseInt(document.getElementById('param-priority').value, 10);

    if (!isNaN(orient) && orient >= 0 && orient <= 1440) AppState.ttbParams.orientationTime = orient;
    if (!isNaN(switchT) && switchT >= 0 && switchT <= 1440) AppState.ttbParams.switchoverTime = switchT;
    if (!isNaN(priTol) && priTol >= 0 && priTol <= 3) AppState.ttbParams.priorityTolerance = priTol;

    updateCalcParamsLabel();
}

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

    // Restore TTB params from AppState (UI-REQ-2091: persist within session)
    document.getElementById('param-orientation').value = AppState.ttbParams.orientationTime;
    document.getElementById('param-switchover').value = AppState.ttbParams.switchoverTime;
    document.getElementById('param-priority').value = AppState.ttbParams.priorityTolerance;
    updateCalcParamsLabel();

    // Collapse params section by default
    document.getElementById('calc-params-content').style.display = 'none';
    document.getElementById('calc-params-toggle').textContent = '▶';

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

// Run path calculation with TTB params and path-scoped recalculation
// (UI-REQ-207 §4, UI-REQ-2091, ALG-REQ-046)
async function runPathCalculation() {
    const fromId = document.getElementById('path-from').value;
    const toId = document.getElementById('path-to').value;
    const hops = document.getElementById('path-hops').value;

    if (!fromId || !toId) {
        document.getElementById('path-status').textContent = 'Please select both entry point and target';
        return;
    }

    // Sync params from UI into AppState before the call
    syncCalcParams();

    const statusEl = document.getElementById('path-status');
    const resultsSection = document.getElementById('path-results-section');
    const resultsBody = document.getElementById('path-results-body');

    statusEl.textContent = 'Calculating paths...';
    resultsBody.innerHTML = '';
    resultsSection.style.display = 'none';
    hideRecalcWarning();
    clearPathHighlights();

    try {
        const data = await API.fetchPaths(fromId, toId, hops, AppState.ttbParams);

        if (data.total === 0) {
            statusEl.textContent = 'No paths found between selected assets.';
            return;
        }

        statusEl.textContent = `Found ${data.total} path(s)`;
        resultsSection.style.display = 'block';

        resultsBody.innerHTML = data.paths.map((path, idx) => `
            <tr class="path-row" data-index="${idx}" data-hosts="${path.hosts}"
                onclick="selectPathRow(this, '${path.hosts}')">
                <td>${path.path_id}</td>
                <td class="path-hosts">${path.hosts}</td>
                <td class="path-tta">${formatTTA(path.tta)}</td>
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

// Select a path row and highlight on graph (UI-REQ-208)
function selectPathRow(rowEl, hostsStr) {
    document.querySelectorAll('.path-row.path-row-selected').forEach(r =>
        r.classList.remove('path-row-selected'));
    rowEl.classList.add('path-row-selected');
    highlightPath(hostsStr);
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

// Highlight a path on the Cytoscape graph with dimming (UI-REQ-208, UI-REQ-332)
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

    // Dim non-path elements (UI-REQ-332: opacity 0.3)
    AppState.cy.elements().not('.path-highlighted').addClass('path-dimmed');

    const highlighted = AppState.cy.elements('.path-highlighted');
    if (highlighted.length > 0) {
        AppState.cy.animate({ fit: { eles: highlighted, padding: 50 } }, { duration: 400 });
    }
}

// Clear path highlights and dimming from graph (UI-REQ-209)
function clearPathHighlights() {
    if (!AppState.cy) return;
    AppState.cy.elements().removeClass('path-highlighted path-dimmed');
}
