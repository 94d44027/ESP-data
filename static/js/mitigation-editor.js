// ============================================
// MITIGATIONS EDITOR (UI-REQ-250 through UI-REQ-258)
// ============================================

const MitEditor = {
    currentAssetId: null,
    currentAssetName: '',
    currentAssetDesc: '',
    allMitigations: [],     // from GET /api/mitigations (REQ-033)
    appliedMitigations: [], // from GET /api/asset/{id}/mitigations (REQ-034)
    selectedRowIdx: null,
    editingRowIdx: null,
    isNewRow: false,

    // ---- Public entry point ----
    open(assetId, assetName, assetDesc) {
        this.currentAssetId = assetId;
        this.currentAssetName = assetName || '';
        this.currentAssetDesc = assetDesc || '';
        this.selectedRowIdx = null;
        this.editingRowIdx = null;
        this.isNewRow = false;

        // Set title (UI-REQ-251 §1)
        document.getElementById('mit-modal-title').textContent =
            `Mitigations for: ${this.currentAssetName} ${this.currentAssetDesc} Asset: ${this.currentAssetId}`;

        // Show modal
        document.getElementById('mit-modal-backdrop').style.display = 'flex';

        // Load data
        this.loadData();
    },

    close() {
        this.editingRowIdx = null;
        this.isNewRow = false;
        document.getElementById('mit-modal-backdrop').style.display = 'none';
    },

    async loadData() {
        const tbody = document.getElementById('mit-table-body');
        tbody.innerHTML = '<tr><td colspan="4" style="text-align:center; color: var(--color-text-muted); padding: 24px;">Loading…</td></tr>';

        try {
            // Load all mitigations list once (REQ-033)
            if (this.allMitigations.length === 0) {
                const listData = await this.fetchJson('/api/mitigations');
                this.allMitigations = listData.mitigations || [];
            }

            // Load applied mitigations for this asset (REQ-034)
            const appliedData = await this.fetchJson(`/api/asset/${this.currentAssetId}/mitigations`);
            this.appliedMitigations = appliedData.mitigations || [];
            this.renderTable();
        } catch (err) {
            console.error('Failed to load mitigations:', err);
            tbody.innerHTML = `<tr><td colspan="4" style="text-align:center; color: var(--color-danger); padding: 24px;">
                Failed to load mitigations. <a href="#" onclick="MitEditor.loadData(); return false;" style="color: var(--color-accent-primary);">Retry</a>
            </td></tr>`;
        }
    },

    // ---- Rendering ----
    renderTable() {
        const tbody = document.getElementById('mit-table-body');
        tbody.innerHTML = '';

        if (this.appliedMitigations.length === 0 && !this.isNewRow) {
            tbody.innerHTML = `<tr><td colspan="4" style="text-align:center; color: var(--color-text-muted); padding: 24px;">
                No mitigations applied. Click ➕ to add one.
            </td></tr>`;
            return;
        }

        this.appliedMitigations.forEach((m, idx) => {
            const tr = this.createRow(m, idx);
            tbody.appendChild(tr);
        });
    },

    createRow(m, idx) {
        const tr = document.createElement('tr');
        tr.className = 'mit-row';
        if (idx === this.selectedRowIdx) tr.classList.add('mit-row-selected');

        if (idx === this.editingRowIdx) {
            return this.createEditRow(m, idx);
        }

        tr.innerHTML = `
            <td class="mit-cell-id">${this.escHtml(m.mitigation_id)}</td>
            <td class="mit-cell-name">${this.escHtml(m.mitigation_name)}</td>
            <td class="mit-cell-maturity">${this.maturityLabel(m.maturity)}</td>
            <td class="mit-cell-active">${m.active ? 'Active' : 'Disabled'}</td>
        `;

        // Selection (UI-REQ-253)
        tr.addEventListener('click', (e) => {
            e.stopPropagation();
            this.selectRow(idx);
        });

        // Double-click enters edit mode (UI-REQ-253)
        tr.addEventListener('dblclick', (e) => {
            e.stopPropagation();
            this.enterEditMode(idx);
        });

        // Action icons on selected row
        if (idx === this.selectedRowIdx) {
            const actionsCell = tr.lastElementChild || tr.children[3];
            const icons = document.createElement('span');
            icons.className = 'mit-row-actions';
            icons.innerHTML = `
                <span class="mit-icon mit-icon-edit" title="Edit">✏️</span>
                <span class="mit-icon mit-icon-delete" title="Delete">✗</span>
            `;
            icons.querySelector('.mit-icon-edit').addEventListener('click', (e) => {
                e.stopPropagation();
                this.enterEditMode(idx);
            });
            icons.querySelector('.mit-icon-delete').addEventListener('click', (e) => {
                e.stopPropagation();
                this.confirmDelete(idx);
            });
            actionsCell.appendChild(icons);
        }

        return tr;
    },

    createEditRow(m, idx) {
        const tr = document.createElement('tr');
        tr.className = 'mit-row mit-row-editing';

        // Build mitigation dropdown — exclude already-applied except current (UI-REQ-254)
        const appliedIds = new Set(this.appliedMitigations.map(a => a.mitigation_id));
        const mitigationOptions = this.allMitigations
            .filter(item => !appliedIds.has(item.mitigation_id) || item.mitigation_id === m.mitigation_id)
            .map(item => {
                const sel = item.mitigation_id === m.mitigation_id ? ' selected' : '';
                return `<option value="${this.escHtml(item.mitigation_id)}" data-name="${this.escHtml(item.mitigation_name)}"${sel}>${this.escHtml(item.mitigation_id)}</option>`;
            }).join('');

        tr.innerHTML = `
            <td>
                <select class="mit-edit-select" id="mit-edit-mid">
                    <option value="">Select…</option>
                    ${mitigationOptions}
                </select>
            </td>
            <td>
                <span id="mit-edit-name" class="mit-edit-name-display">${this.escHtml(m.mitigation_name || '')}</span>
            </td>
            <td>
                <select class="mit-edit-select" id="mit-edit-maturity">
                    <option value="25"${m.maturity === 25 ? ' selected' : ''}>Low</option>
                    <option value="50"${m.maturity === 50 ? ' selected' : ''}>Medium</option>
                    <option value="80"${m.maturity === 80 ? ' selected' : ''}>High</option>
                    <option value="100"${m.maturity === 100 ? ' selected' : ''}>Best</option>
                </select>
            </td>
            <td>
                <select class="mit-edit-select" id="mit-edit-active">
                    <option value="true"${m.active !== false ? ' selected' : ''}>Active</option>
                    <option value="false"${m.active === false ? ' selected' : ''}>Disabled</option>
                </select>
                <span class="mit-icon mit-icon-save" title="Save (Enter)">✓</span>
            </td>
        `;

        // Auto-populate name on mitigation change
        const midSelect = tr.querySelector('#mit-edit-mid');
        midSelect.addEventListener('change', () => {
            const opt = midSelect.selectedOptions[0];
            const nameSpan = tr.querySelector('#mit-edit-name');
            nameSpan.textContent = opt ? (opt.dataset.name || '') : '';
        });

        // Save icon
        tr.querySelector('.mit-icon-save').addEventListener('click', (e) => {
            e.stopPropagation();
            this.saveEdit(idx);
        });

        // Keyboard: Enter = save, Escape = cancel (UI-REQ-254)
        tr.addEventListener('keydown', (e) => {
            if (e.key === 'Enter') {
                e.preventDefault();
                this.saveEdit(idx);
            } else if (e.key === 'Escape') {
                e.preventDefault();
                this.cancelEdit(idx);
            }
        });

        // Focus the mitigation dropdown
        setTimeout(() => midSelect.focus(), 50);

        return tr;
    },

    // ---- Row interactions ----
    selectRow(idx) {
        if (this.editingRowIdx !== null) return; // don't change selection while editing
        this.selectedRowIdx = (this.selectedRowIdx === idx) ? null : idx;
        this.renderTable();
    },

    deselectAll() {
        if (this.editingRowIdx !== null) return;
        this.selectedRowIdx = null;
        this.renderTable();
    },

    enterEditMode(idx) {
        if (this.editingRowIdx !== null) return; // only one row at a time
        this.selectedRowIdx = idx;
        this.editingRowIdx = idx;
        this.renderTable();
    },

    cancelEdit(idx) {
        if (this.isNewRow && idx === this.appliedMitigations.length - 1) {
            // Remove the unsaved new row (UI-REQ-255)
            this.appliedMitigations.pop();
            this.isNewRow = false;
        }
        this.editingRowIdx = null;
        this.selectedRowIdx = null;
        this.renderTable();
    },

    // ---- Add new row (UI-REQ-255) ----
    addNewRow() {
        if (this.editingRowIdx !== null) return;
        const newRow = {
            mitigation_id: '',
            mitigation_name: '',
            maturity: 100,
            active: true
        };
        this.appliedMitigations.push(newRow);
        this.isNewRow = true;
        this.editingRowIdx = this.appliedMitigations.length - 1;
        this.selectedRowIdx = this.editingRowIdx;
        this.renderTable();

        // Scroll table to bottom
        const wrapper = document.getElementById('mit-table-wrapper');
        if (wrapper) wrapper.scrollTop = wrapper.scrollHeight;
    },

    // ---- Save (UI-REQ-256) ----
    async saveEdit(idx) {
        const midEl = document.getElementById('mit-edit-mid');
        const matEl = document.getElementById('mit-edit-maturity');
        const actEl = document.getElementById('mit-edit-active');

        const mitigationId = midEl.value;
        const maturity = parseInt(matEl.value, 10);
        const active = actEl.value === 'true';

        // Validation (UI-REQ-256 §1)
        if (!mitigationId) {
            midEl.style.borderColor = 'var(--color-danger)';
            midEl.focus();
            return;
        }
        midEl.style.borderColor = '';

        const mitigationName = midEl.selectedOptions[0]?.dataset.name || mitigationId;
        const matLabel = this.maturityLabel(maturity);
        const activeLabel = active ? 'Active' : 'Disabled';

        // Confirmation dialog (UI-REQ-256 §2)
        const confirmed = confirm(
            `Confirm Mitigation Change\n\nApply ${mitigationName} (maturity: ${matLabel}, ${activeLabel}) to asset ${this.currentAssetId}?`
        );
        if (!confirmed) return;

        // Send PUT (REQ-035)
        try {
            await this.sendJson(`/api/asset/${this.currentAssetId}/mitigations`, 'PUT', {
                mitigation_id: mitigationId,
                maturity: maturity,
                active: active
            });

            // Success: update local data and exit edit mode
            this.appliedMitigations[idx] = {
                mitigation_id: mitigationId,
                mitigation_name: mitigationName,
                maturity: maturity,
                active: active
            };
            this.editingRowIdx = null;
            this.isNewRow = false;
            this.selectedRowIdx = null;

            // Brief green flash
            this.renderTable();
            const rows = document.querySelectorAll('#mit-table-body .mit-row');
            if (rows[idx]) {
                rows[idx].classList.add('mit-flash-success');
                setTimeout(() => rows[idx].classList.remove('mit-flash-success'), 600);
            }
        } catch (err) {
            console.error('Upsert mitigation failed:', err);
            // Brief red flash, stay in edit mode
            const editRow = document.querySelector('.mit-row-editing');
            if (editRow) {
                editRow.classList.add('mit-flash-error');
                setTimeout(() => editRow.classList.remove('mit-flash-error'), 600);
            }
        }
    },

    // ---- Delete (UI-REQ-257) ----
    async confirmDelete(idx) {
        const m = this.appliedMitigations[idx];
        if (!m) return;

        const confirmed = confirm(
            `Remove Mitigation\n\nRemove ${m.mitigation_name || m.mitigation_id} from asset ${this.currentAssetId}? This action cannot be undone.`
        );
        if (!confirmed) return;

        try {
            const resp = await fetch(
                `/api/asset/${this.currentAssetId}/mitigations/${encodeURIComponent(m.mitigation_id)}`,
                { method: 'DELETE' }
            );
            if (!resp.ok) throw new Error(`DELETE failed: ${resp.status}`);

            // Fade-out and remove (UI-REQ-257 §3)
            const rows = document.querySelectorAll('#mit-table-body .mit-row');
            if (rows[idx]) {
                rows[idx].style.transition = 'opacity 200ms';
                rows[idx].style.opacity = '0';
                await new Promise(r => setTimeout(r, 200));
            }

            this.appliedMitigations.splice(idx, 1);
            this.selectedRowIdx = null;
            this.renderTable();
        } catch (err) {
            console.error('Delete mitigation failed:', err);
        }
    },

    // ---- Helpers ----
    maturityLabel(val) {
        switch (val) {
            case 25: return 'Low';
            case 50: return 'Medium';
            case 80: return 'High';
            case 100: return 'Best';
            default: return String(val);
        }
    },

    escHtml(str) {
        const div = document.createElement('div');
        div.textContent = str || '';
        return div.innerHTML;
    },

    async fetchJson(url) {
        const resp = await fetch(url);
        if (!resp.ok) throw new Error(`GET ${url}: ${resp.status}`);
        return resp.json();
    },

    async sendJson(url, method, body) {
        const resp = await fetch(url, {
            method: method,
            headers: { 'Content-Type': 'application/json' },
            body: JSON.stringify(body)
        });
        if (!resp.ok) {
            let msg = `${method} ${url}: ${resp.status}`;
            try { const d = await resp.json(); if (d.error) msg = d.error; } catch(_) {}
            throw new Error(msg);
        }
        return resp.json();
    }
};

// ---- Keyboard: Escape closes modal (UI-REQ-251) ----
document.addEventListener('keydown', (e) => {
    if (e.key === 'Escape' && document.getElementById('mit-modal-backdrop').style.display === 'flex') {
        if (MitEditor.editingRowIdx !== null) return; // handled by edit row
        MitEditor.close();
    }
});
