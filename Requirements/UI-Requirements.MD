# UI Requirements Specification
## ESP PoC - Visual Layer

**Version:** 1.0  
**Date:** February 17, 2026  
**Prepared by:** Based on visual reference analysis  
**Project:** ESP PoC for Nebula Graph  
**Reference:** Derived from demo UI screenshots

---

## 1. Overview

This document specifies the user interface requirements for the ESP PoC Visual Layer. The interface provides graph-based visualization of IT infrastructure assets and their relationships, enabling attack path analysis through interactive exploration.

### 1.1 Document Scope

This specification supersedes **REQ-123** from the main Requirements.md and provides detailed UI/UX requirements that may span multiple HTML files or a more complex single-page application architecture as needed for functionality.

### 1.2 Design Philosophy

- **Dark theme** as primary color scheme (matching reference UI)
- **High contrast** for accessibility (per REQ-011, REQ-101)
- **Graph-centric** layout with supporting panels
- **Minimal cognitive load** - focus on the visualization

---

## 2. Overall Layout Architecture

### UI-REQ-100: Application Shell Structure

The application SHALL use a fixed-position layout with the following zones:

| Zone               | Location                               | Purpose                                          | Collapsible |
|--------------------|----------------------------------------|--------------------------------------------------|-------------|
| **Top Bar**        | Top, full width, ~50-60px height       | Application title, global actions, view controls | No          |
| **Left Sidebar**   | Left, ~280-320px width, below top bar  | Entity browser, search, filtering                | Yes         |
| **Main Canvas**    | Center, fills remaining space          | Graph visualization area                         | No          |
| **Right Panel**    | Right, ~350-400px width, below top bar | Node/edge detail inspector                       | Yes         |
| **Bottom Toolbar** | Bottom of canvas, ~40-50px height      | Graph manipulation controls, zoom, layout        | No          |

**Behavior:**
- Sidebar collapse states should be remembered in browser `sessionStorage`
- When sidebar collapses, show icon-only button to re-expand
- Right panel appears only when node/edge selected (can be manually closed)
- Canvas should resize fluidly when sidebars toggle

---

## 3. Top Bar (Global Navigation)

### UI-REQ-110: Top Bar Structure

The top bar SHALL contain:

**Left Section:**
- Application logo/icon (home link)
- Application title: "ESP PoC" or configurable name


**Center Section:**
- Currently selected view indicator (e.g., "Graph View", "Path Analysis")

**Right Section:**
- Global search input field (magnifying glass icon)
- View mode toggles (graph layout selector)
- Settings/preferences button (gear icon)
- Help/documentation button (question mark icon)
- Refresh data button (per REQ-013 concept - reload graph)

**Visual Style:**
- Background: Dark gray/charcoal (`#1a1a1a` - `#252525` range)
- Text: Light gray or white (`#e0e0e0` - `#ffffff`)
- Height: 56px
- Icons: 20-24px, with hover states

### UI-REQ-111: Global Search

**Location:** Top bar, right section  
**Width:** 200-250px, expands to 350px on focus

**Functionality:**
- Placeholder text: "Search assets..."
- Search types: Asset ID, IP address, asset name
- Real-time filter as user types (debounced 300ms)
- Results highlight matching nodes in graph
- Dropdown suggestion list (max 10 results) with node type badges

**Keyboard shortcuts:**
- `Cmd/Ctrl + K` to focus search
- `Escape` to clear and unfocus
- Arrow keys to navigate suggestions
- `Enter` to select/zoom to highlighted node

---

## 4. Left Sidebar (Entity Browser)

### UI-REQ-120: Sidebar Structure

**Layout (Top to Bottom):**

1. **Tab Bar** (if multiple views needed)
   - "Assets" tab (primary - list of all nodes)
   - "Add" tab (for future: manual node addition - out of scope for v1.0)

2. **Counter and Search Bar**
   - Node count display: "N / Total nodes"
   - Search input: Icon on left, clear button on right
   - Filter toggle button (funnel icon)

3. **Entity List**
   - Scrollable list of all assets from database
   - Each item shows:
     - Asset ID (primary text)
     - Asset type badge (e.g., "Server", "Network device", "Workstation"). Asset type is defined by "has_type" relationship from the Asset.
     - Optionally: small icon representing type

**Visual Style:**
- Background: Slightly lighter than canvas (`#202020` - `#282828`)
- Borders: Subtle dividers between items (`#333333`)
- Selected item: Highlight background (`#2a4a5a` or accent color with opacity)
- Hover state: Lighter background (`#303030`)

### UI-REQ-121: Entity List Item

Each entity in the list SHALL display:

**Structure:**
```
[Icon] Asset_ID_text               [Badge]
       optional_secondary_text
```

**Badge colors** (following reference UI teal/green badges):
- `Workstation` → Teal badge (`#4a9d9c`, text: `#ffffff`)
- `Network Device` → Blue badge (`#5a7fbf`, text: `#ffffff`)  
- `Server` → Green badge (`#5a9d5a`, text: `#ffffff`)
- `Mobile Device` → Grape badge (`#9437FF`, text: `#ffffff`)
- `IoT Device` → Orange badge (`#BE5014`, text: `#ffffff`)
- (Extend as needed for Asset types in your schema)

**Interaction:**
- Click: Select node in graph and zoom/center on it
- Hover: Show tooltip with extended info (if available)
- Right-click: Context menu (future: "Focus on this node", "Hide neighbors", etc.)

### UI-REQ-122: Sidebar Search and Filter

**Search Field:**
- Placeholder: "Search..." or "Filter entities..."
- Searches against Asset_Name or Asset_ID field
- Case-insensitive
- Updates list in real-time
- Show match count: "N / Total nodes" updates dynamically

**Filter Button (funnel icon):**
- Click opens filter panel overlay or dropdown
- **Filter Options:**
  - **By Asset Type:** Checkboxes for Workstation, Network Device, Server, etc.
  - **By Connection Count:** Slider or range input (future)
  - **By Attribute:** (future extensibility)
- "Clear Filters" button
- Filter state indicated by highlight on funnel icon when active

**Behavior:**
- Filters combine with search (AND logic)
- Graph canvas updates to dim filtered out nodes (option: hide filtered out nodes completely)
- Filter state saved in `sessionStorage`

### UI-REQ-123: Sidebar Collapse

**Collapse Button:**
- Icon button at top-right of sidebar header
- Icon: Left-pointing chevron or double-chevron
- Collapses sidebar to ~40px icon bar
- Shows only asset type icon buckets when collapsed (optional enhancement)

**Collapsed State:**
- Sidebar width: 0px or 40px (icon strip)
- Button to expand: Right-pointing chevron
- Animation: Smooth 250ms transition

---

## 5. Main Canvas (Graph Visualization)

### UI-REQ-200: Canvas Core Requirements

The main graph canvas SHALL:

- Occupy all available space between sidebars and top/bottom bars
- Use **Cytoscape.js** for rendering (per REQ-010)
- Have dark background: `#0a0a0a` or `#121212`
- Optional: Subtle dot grid pattern background (per reference UI "Show Background" setting)

**Graph Data Source:**
- Fetch from `/api/graph` endpoint on load (per existing `index.html`)
- Expected format: `{ nodes: [...], edges: [...] }` (Cytoscape elements format)

### UI-REQ-201: Node Visualization

**Node Appearance:**
- **Shape:** Circle (default for all Asset types, extensible)
- **Size:** 40-50px diameter (adjustable via settings)
- **Color:** By Asset type:
  - `Workstation` → Teal (`#4a9d9c`)
  - `Network Device` → Blue (`#5a7fbf`)
  - `Server` → Green (`#5a9d5a`)
  - `Mobile Device` → Grape (`#9437FF`)
  - `IoT Device` → Orange (`#BE5014`)
- **Label:**
  - Display: Asset_ID
  - Font: Sans-serif, 10-12px
  - Position: Below node (or inside if space permits)
  - Color: `#e0e0e0`
  - **Important:** Asset ID must fit into or near node (per REQ-013)
  - Truncate with ellipsis if too long (show full in tooltip)

**Node States:**
- **Default:** As above
- **Hover:** Increase border width (2px → 4px), glow effect
- **Selected:** Bright border (`#ffffff` or accent color), drop shadow
- **Dimmed:** (when filtering) Opacity 0.3, desaturated
- **Highlighted:** (from search/path) Pulsing glow or distinct border color (orange `#ff9933`)

**Node Behavior:**
- Click: Select node, open detail panel, highlight in entity list
- Double-click: Expand/collapse neighbors (if too dense)
- Drag: Move node (layout should allow manual repositioning)
- Right-click: Context menu (future)

### UI-REQ-202: Edge Visualization

**Edge Appearance:**
- **Style:** Directed arrows (per REQ-012)
- **Width:** 1.5-2px
- **Color:** Gray (`#555555`) for default
- **Arrow:**
  - Target arrowhead: Triangle shape
  - Size: Proportional to edge width
  - Color: Matches line color

**Edge Labels:**
- Display edge type/relationship label if available
- Position: Midpoint of edge
- Font: 9-10px, color `#999999`
- Only show on hover or when zoomed in (avoid clutter)

**Edge States:**
- **Default:** As above
- **Hover:** Highlight with brighter color (`#999999`), increase width slightly
- **Selected:** (when clicking edge or part of path) Color: Orange or accent (`#ff9933`), width 3-4px
- **Path Member:** (when showing attack path) Color: Orange (`#ff9933`), animated dashes or glow
- **Dimmed:** (when filtering) Opacity 0.2

**Edge Labels (future):**
- Relationship type: `connects_to`, `has_type` etc.
- Display on hover or via toggle

### UI-REQ-203: Graph Layouts

The application SHALL support multiple layout algorithms:

**Default Layout:**
- **Force-directed (Cose):** Per existing `index.html` - `{ name: 'cose', animate: true }`
- Good for organic exploration
- Animates on load

**Additional Layouts (via settings or dropdown):**
- **Circle:** Nodes in circle
- **Grid:** Regular grid pattern
- **Concentric:** Nodes in concentric rings (by degree/importance)
- **Breadth-First (BFS):** Tree layout starting from selected node (for path viz)
- **Dagre (Hierarchical):** Directed acyclic graph layout (future)

**Layout Controls:**
- Dropdown in top bar or bottom toolbar: "Layout: [Cose ▾]"
- Clicking triggers re-layout with animation
- Save preference in `sessionStorage`

### UI-REQ-204: Graph Interaction and Navigation

**Pan:**
- Click-and-drag on empty canvas space
- Cursor: Grab hand icon

**Zoom:**
- Mouse wheel: Scroll up/down to zoom in/out
- Pinch gesture on trackpad
- Zoom range: 10% - 500%
- Zoom controls in bottom toolbar (see UI-REQ-230)

**Fit to View:**
- Button in bottom toolbar: "Fit" or home icon
- Centers and scales graph to show all nodes

**Selection:**
- Click node: Single selection
- `Cmd/Ctrl + Click`: Multi-select (additive)
- Box select: Click-and-drag with `Shift` held (marquee selection)
- Click empty space: Deselect all

**Keyboard Shortcuts:**
- `F`: Fit graph to view
- `+` / `-`: Zoom in/out
- `Arrow keys`: Pan canvas
- `Delete` / `Backspace`: Remove selected nodes from view (hide, not delete from DB)
- `Esc`: Clear selection

### UI-REQ-205: Minimap (Optional Enhancement)

**Location:** Bottom-right corner of canvas (or top-right), ~150x100px

**Purpose:**
- Shows zoomed-out overview of entire graph
- Current viewport indicated by rectangle overlay
- Click or drag in minimap to navigate

**Visual:**
- Semi-transparent overlay
- Nodes simplified (small dots)
- Can be toggled on/off via settings

---

## 6. Right Panel (Detail Inspector)

### UI-REQ-210: Inspector Panel Structure

**Visibility:**
- Hidden by default
- Appears when node or edge is selected
- Slides in from right with animation (250ms)
- Close button (X) at top-right

**Width:** 350-400px (fixed or resizable)

**Sections (Top to Bottom):**

1. **Header**
   - Node icon (large, colored by type)
   - Primary label: Asset_Name (large, bold font)
   - Type badge (e.g., "Server")
   - Close button (X icon)

2. **Primary Attributes** (Key-value pairs)
   - Properties displayed:
     - `Asset_ID`: (value)
     - `Asset_Name`: (value)
     - `Asset_Description`: (value)
     - `Asset_Type`: derived from "has_type" relationship to Asset_Type, "Type_Name" field of Asset_Type
     - `is_entrance`: (value)
     - `is_target`: (value)
     - `priority`: (value)
     - `has_vulnerability`: (value)
     - `TTB`: (value)
   - Display as table
   - Truncate long values with expand button

3. **Connections Summary**
   - "N neighbor(s)" (clickable to expand list)
   - "Inbound: M" / "Outbound: K"

4. **Neighbor List** (Collapsible)
   - Shows immediate neighbors
   - Each item: Icon + Asset_ID + edge label
   - Click neighbor to navigate to that node

5.**Actions** (future)
   - "Set as Entry Point" button
   - "Set as Target" button
   - "Find Paths" button

**Visual Style:**
- Background: Same as left sidebar (`#202020` - `#282828`)
- Text: Light gray (`#e0e0e0`)
- Section headers: Bold, slightly larger font
- Dividers: Subtle lines between sections

### UI-REQ-211: Neighbor Visualization (Radial View)

When a node is selected, the detail panel MAY show a **radial mini-graph** (per screenshot 5):

- Central node (selected asset)
- Surrounding circle of immediate neighbors
- Hover over neighbor highlights it in main canvas
- Click neighbor to navigate

**Style:**
- Small circular layout (~150x150px)
- Node colors consistent with main graph
- Simplified labels

---

## 7. Bottom Toolbar (Graph Controls)

### UI-REQ-230: Bottom Toolbar Structure

**Location:** Fixed at bottom of canvas area, full width of canvas (not extending under sidebars)

**Height:** 44-50px

**Background:** Semi-transparent dark (`rgba(26, 26, 26, 0.95)`) with slight blur (backdrop-filter)

**Left Section:**
- **Zoom controls:**
  - Zoom out button (`-` or magnifying glass minus)
  - Zoom level indicator (e.g., "100%")
  - Zoom in button (`+` or magnifying glass plus)
  - Fit to view button (home icon or "Fit")
- **Reset view button** (circular arrow - reset pan/zoom)

**Center Section:**
- **Layout selector:** Dropdown "Layout: Cose ▾"
- **Node visibility info:** "Showing 63 / 300 nodes" (if filtered)

**Right Section:**
- **Filters button** (funnel icon with badge if active)
- **Graph settings button** (sliders icon - opens settings modal per screenshot 2)
- **Minimap toggle** (optional)
- **Export button** (download icon - future: export PNG/SVG)

**Visual Style:**
- Buttons: Icon-only with tooltips on hover
- Spacing: 8-12px between button groups, 4-6px between buttons
- Icons: 18-20px, light gray (`#b0b0b0`), white on hover
- Dividers: Vertical lines between sections

---

## 8. Modals and Overlays

### UI-REQ-240: Settings Modal

**Trigger:** Gear icon in top bar OR sliders icon in bottom toolbar

**Appearance:**
- Centered modal overlay, ~600px width, auto height
- Dark background with slight transparency (`rgba(10, 10, 10, 0.85)`)
- Modal content: Dark card (`#252525`) with rounded corners

**Tabs (horizontal):**
- Information (about/version)
- General (UI preferences)
- Graph (graph-specific settings)
- Node colors (color customization)

**Settings from Screenshot 2:**

**General Tab:**
- [ ] Show Flow Assistant (toggle)
- [ ] Auto-zoom on Node Selection (toggle)
- [x] Show Minimap (toggle - enabled in reference)
- [x] Show Background (toggle - dotted grid pattern)
- [ ] Auto Color Links by Node Type (toggle)

**Graph Tab:**
- Layout algorithm selector (dropdown)
- Node size slider (30px - 60px)
- Edge width slider (1px - 4px)
- Label font size slider (8px - 14px)
- Animation speed (fast/medium/slow)

**Node Colors Tab:**
- List of asset types with color pickers
- Reset to defaults button

**Buttons:**
- "Cancel" (bottom-left)
- "Apply" or "Save" (bottom-right, primary style)

### UI-REQ-241: Path Analysis Dialog (Future - Out of Scope for v1.0)

When user selects "Calculate Path" (future feature per REQ-020 scope):

**Structure:**
- Modal dialog, ~500px width
- Title: "Calculate Attack Path"
- **Entry Point Selector:** Dropdown populated from predefined list
- **Target Selector:** Dropdown populated from predefined list
- **Options:**
  - Max path length (number input, default 5)
  - Path algorithm (shortest/all paths/k-shortest)
- **Buttons:**
  - "Cancel"
  - "Calculate" (primary)

**Result Display:**
- Closes modal
- Highlights path(s) on graph with animated orange edges
- Shows path list in right panel with hop-by-hop breakdown

---

## 9. Color Palette and Theme

### UI-REQ-300: Dark Theme Color Specification

Based on reference UI, the application SHALL use the following color palette:

**Backgrounds:**
- App background (darkest): `#0a0a0a`
- Canvas background: `#121212`
- Sidebar/panel background: `#1e1e1e` - `#252525`
- Toolbar background: `rgba(26, 26, 26, 0.95)`
- Card/modal background: `#2a2a2a`

**Text:**
- Primary text: `#e0e0e0` - `#ffffff`
- Secondary text: `#b0b0b0`
- Muted text: `#808080`
- Disabled text: `#505050`

**Interactive Elements:**
- Primary accent (buttons, selection): Teal `#40b5b4` or `#4ecdc4`
- Hover state: Lighter teal `#5fd9d8`
- Active/pressed state: Darker teal `#3a9c9b`
- Link color: Light blue `#6db8ff`

**Status Colors:**
- Success/positive: Green `#5ac05a`
- Warning: Orange `#ff9933`
- Error/critical: Red `#e74c3c`
- Info: Blue `#5a9bd5`

**Graph Nodes (by Asset Type):**
- `Workstation` → Teal (`#4a9d9c`)
- `Network Device` → Blue (`#5a7fbf`)
- `Server` → Green (`#5a9d5a`)
- `Mobile Device` → Grape (`#9437FF`)
- `IoT Device` → Orange (`#BE5014`)

**Graph Edges:**
- Default: Gray `#555555`
- Selected/path: Orange `#ff9933`
- Hover: Light gray `#999999`

**Borders:**
- Subtle dividers: `#333333`
- Input borders: `#444444`
- Input borders (focus): Accent teal `#40b5b4`

### UI-REQ-301: Contrast Requirements

All text and interactive elements MUST meet WCAG AA contrast standards:
- Large text (18pt+): Minimum 3:1 contrast ratio
- Normal text: Minimum 4.5:1 contrast ratio
- Interactive elements: Minimum 3:1 contrast against background

(Complies with REQ-011 and REQ-101)

---

## 10. Responsive Behavior

### UI-REQ-310: Minimum Resolution

Per REQ-100, the interface SHALL be functional on:
- **Minimum resolution:** 1920x1080 pixels (desktop)
- **Optimal resolution:** 2560x1440 or higher

**Breakpoints (future mobile/tablet - out of scope for v1.0):**
- Desktop: ≥1920px
- Laptop: 1366px - 1919px
- (Tablet/mobile not required for PoC)

### UI-REQ-311: Sidebar Responsive Behavior

- Below 1920px width: Sidebar can collapse to maximize canvas space
- Below 1600px width: Right panel overlays canvas instead of pushing content
- At minimum resolution (1920x1080): All controls remain accessible

---

## 11. Performance and Animation

### UI-REQ-320: Rendering Performance

- Initial graph load: Render within 2 seconds for ≤300 nodes (per REQ-202)
- Layout calculation: Complete within 3 seconds
- Smooth animation at 60fps for zoom/pan operations
- Node selection response: <100ms

### UI-REQ-321: Animation Standards

- Layout transitions: 800ms easing
- Sidebar collapse/expand: 250ms cubic-bezier
- Panel slide-in/out: 250ms ease-out
- Hover effects: 150ms ease
- Zoom: Smooth continuous animation

**Accessibility:**
- Respect `prefers-reduced-motion` media query
- Disable/reduce animations if user preference set

---

## 12. Interaction Patterns

### UI-REQ-330: Selection Behavior

**Single Node Selection:**
1. Click node
2. Node highlighted (border change)
3. Right panel slides in with details
4. Entity list scrolls to and highlights corresponding item

**Multi-Node Selection:**
1. `Cmd/Ctrl + Click` additional nodes
2. All selected nodes highlighted
3. Right panel shows count: "N nodes selected"
4. Can perform batch actions (future)

**Edge Selection:**
1. Click edge
2. Edge highlighted
3. Right panel shows edge details (source, target, relationship type)

**Clear Selection:**
- Click empty canvas space
- Press `Escape` key
- Click X button in detail panel

### UI-REQ-331: Search and Filter Interaction

**Search Flow:**
1. User types in search field (sidebar or global)
2. After 300ms debounce, filter entity list
3. Matching nodes in graph pulsate or highlight with border
4. Non-matching nodes dim (opacity 0.3) OR graph shows only matches
5. Selection behavior:
   - Click search result → zoom and center on that node
   - Enter key → select first result

**Filter Flow:**
1. User clicks filter button, opens filter panel
2. User checks/unchecks asset type checkboxes
3. Graph and entity list update in real-time
4. Filter state persists during session
5. "Clear Filters" resets to show all

### UI-REQ-332: Path Highlighting (Future Feature)

When attack path is calculated:
1. All path edges change to orange color
2. Path edges animate (dashed line moving or glow pulse)
3. Path nodes highlighted with orange border
4. Non-path elements dimmed
5. Right panel lists path steps in order
6. Click path step → zoom to that hop
7. "Clear Path" button returns to normal view

---

## 13. Keyboard Shortcuts Summary

### UI-REQ-340: Global Shortcuts

| Shortcut               | Action                        |
|------------------------|-------------------------------|
| `Cmd/Ctrl + K`         | Focus global search           |
| `Cmd/Ctrl + F`         | Focus entity list search      |
| `F`                    | Fit graph to view             |
| `+` or `=`             | Zoom in                       |
| `-` or `_`             | Zoom out                      |
| `0` (zero)             | Reset zoom to 100%            |
| `Arrow keys`           | Pan canvas                    |
| `Escape`               | Clear selection / close panel |
| `Cmd/Ctrl + A`         | Select all nodes (visible)    |
| `Delete` / `Backspace` | Hide selected nodes           |
| `Cmd/Ctrl + Z`         | Undo last action (future)     |

---

## 14. Accessibility

### UI-REQ-350: ARIA and Semantic HTML

- All interactive elements must have proper ARIA labels
- Keyboard navigation: All controls reachable via Tab
- Focus indicators: Visible outline on all focusable elements
- Screen reader support: Descriptive labels for graph elements

### UI-REQ-351: Keyboard Navigation

- Tab order: Top bar → Left sidebar → Canvas → Right panel → Bottom toolbar
- Within entity list: Arrow up/down to navigate items, Enter to select
- Within graph: Tab focuses on selected node, arrow keys to move focus to neighbors
- Modal dialogs: Focus trap within modal, Escape to close

---

## 15. Error States and Empty States

### UI-REQ-360: Loading States

**Graph Loading:**
- Show spinner in center of canvas
- Text: "Loading graph data..."
- Progress indicator if possible

**Sidebar Loading:**
- Show skeleton placeholders for entity list items
- Or spinner with "Loading entities..."

### UI-REQ-361: Error States

**API Error:**
- Show error message in place of graph
- Icon: Warning triangle
- Message: "Failed to load graph data. [Retry]"
- Retry button re-fetches from `/api/graph`

**Empty State:**
- No nodes in database:
  - Message: "No assets found. Check database connection."
  - Icon: Empty graph illustration

**No Search Results:**
- Message in entity list: "No assets match 'query'"
- Button: "Clear search" to reset

### UI-REQ-362: User Feedback

**Actions with feedback:**
- Selection: Immediate visual change + haptic feedback (if supported)
- Path calculation: Progress indicator + completion toast
- Settings save: Toast notification: "Settings saved"
- Copy node ID: Toast: "Copied to clipboard"

---

## 16. Future Enhancements (Out of Scope for v1.0)

The following features are illustrated in reference UI but marked for future releases:

- [ ] Timeline/playback controls (for temporal analysis)
- [ ] Console/terminal panel (for query execution)
- [ ] Vault/data source management
- [ ] Multiple investigation workspaces
- [ ] Collaboration features (sharing, comments)
- [ ] Export to PDF/PNG with annotations
- [ ] Advanced filtering (date ranges, custom properties)
- [ ] Path probability scoring
- [ ] Mitigation simulation

---

## 17. Technical Implementation Notes

### UI-REQ-400: Technology Stack

Per existing project structure:

- **Frontend:** HTML5 + CSS3 + JavaScript (ES6+)
- **Graph Library:** Cytoscape.js 3.x (per REQ-010)
- **HTTP Client:** Fetch API
- **State Management:** Browser sessionStorage (no external library needed for PoC)
- **Icons:** Lucide Icons, Heroicons, or similar lightweight icon set
- **Fonts:** System fonts (`-apple-system, BlinkMacSystemFont, 'Segoe UI', sans-serif`)

### UI-REQ-401: File Structure (Revised from REQ-123)

Given the complexity identified, the VIS layer MAY be structured as:


```
static/
├── index.html          # Main HTML shell
├── css/
│   ├── main.css        # Layout and global styles
│   ├── graph.css       # Graph-specific styles
│   └── components.css  # Sidebar, panels, modals
├── js/
│   ├── app.js          # Main application initialization
│   ├── graph.js        # Cytoscape instance and graph logic
│   ├── sidebar.js      # Entity list and search
│   ├── detail-panel.js # Right panel logic
│   └── utils.js        # Helper functions
└── assets/
    └── icons/          # SVG icons if needed
```

All files still served from `/opt/asset-viz/static/` as currently configured.

### UI-REQ-402: API Endpoints Required

Based on UI requirements, the following endpoints are needed:

**Existing:**
- `GET /api/graph` - Returns all nodes and edges (current implementation)

**New endpoints needed for full functionality:**
- `GET /api/assets` - Returns list of assets for sidebar (with filtering)
  - Query params: `?type=domain&search=dev`
- `GET /api/asset/{id}` - Returns detailed info for single asset
- `GET /api/paths?from={id}&to={id}` - Calculate attack paths (future)
  - Returns: `{ paths: [[node1, node2, ...]], path_data: {...} }`
- `GET /api/neighbors/{id}` - Returns immediate neighbors of a node

**Data Format Example (for sidebar):**
```json
{
  "assets": [
    {
      "id": "db-dev-very-vulnerable.mistery-domain.org",
      "type": "domain",
      "display_name": "db-dev-very-vulnerable...",
      "properties": {
        "ip": "192.168.1.50",
        "created_at": "2025-12-10T10:26:49Z"
      }
    }
  ],
  "total": 63,
  "filtered": 3
}
```

### UI-REQ-403: Browser Compatibility

- Chrome 100+
- Firefox 98+
- Safari 15+
- Edge 100+

(Per REQ-100)

**No support required for:** IE11, older mobile browsers

---

## 18. Testing Requirements

### UI-REQ-410: Visual Testing

- [ ] Dark theme renders correctly in all sections
- [ ] All contrast ratios meet WCAG AA standards
- [ ] Hover states visible on all interactive elements
- [ ] Focus indicators visible for keyboard navigation

### UI-REQ-411: Functional Testing

- [ ] Graph loads and displays all 300 nodes from demo dataset
- [ ] Sidebar search filters nodes correctly
- [ ] Node selection opens detail panel with correct data
- [ ] Zoom and pan work smoothly
- [ ] Layout algorithms all produce valid layouts
- [ ] Keyboard shortcuts work as specified
- [ ] Responsive behavior correct at 1920x1080 resolution

### UI-REQ-412: Performance Testing

- [ ] Graph renders <2s for 300 nodes
- [ ] Search debouncing works (300ms)
- [ ] No memory leaks during extended session
- [ ] 60fps maintained during pan/zoom animations

---

## 19. Acceptance Criteria

The UI implementation SHALL be considered complete when:

✅ All UI-REQ sections implemented  
✅ Visual appearance matches reference UI dark theme aesthetic  
✅ All interactive elements respond correctly to user input  
✅ Graph visualization displays data from `/api/graph` endpoint  
✅ Sidebar entity list populated and searchable  
✅ Detail panel shows node information on selection  
✅ Keyboard shortcuts functional  
✅ Contrast requirements met (REQ-011, REQ-101)  
✅ Performance targets met (REQ-201, REQ-202)  
✅ No console errors during normal operation  
✅ Compatible with required browsers (REQ-100)  

---

## 20. References

- **Main Requirements:** `Requirements/Requirements.md` (ESP PoC SRS v1.1)
- **Reference UI:** Screenshots provided (demo3.mp4 frames)
- **Existing Implementation:** `static/index.html` (ESP-data repo)
- **Cytoscape.js Documentation:** https://js.cytoscape.org/
- **Nebula Graph Schema:** `Data/ESP01_NebulaGraph_Schema.md`

---

## Appendix A: UI Component Hierarchy

```
┌─ TopBar ───────────────────────────────────────────────────────────┐
│  Logo | Title | [Search] | View Controls | Settings | Help | ⟳     │
└────────────────────────────────────────────────────────────────────┘
┌──────────┬───────────────────────────────────────────┬─────────────┐
│          │                                           │             │
│ Left     │                                           │    Right    │
│ Sidebar  │           Graph Canvas                    │    Panel    │
│          │         (Cytoscape.js)                    │  (Inspector)│
│ ┌──────┐ │                                           │             │
│ │Search│ │                                           │  [Details]  │
│ └──────┘ │                                           │             │
│          │                                           │             │
│ Entity   │                                           │ Attributes  │
│ List     │                                           │ Neighbors   │
│  • Item1 │                                           │ Paths       │
│  • Item2 │                                           │             │
│  • ...   │        ┌────────────────────────┐         │             │
│          │        │  Bottom Toolbar        │         │             │
│          │        │  Zoom | Layout | ⚙️    │         │             │
│          │        └────────────────────────┘         │             │
└──────────┴───────────────────────────────────────────┴─────────────┘
```

---

## Appendix B: Mapping to Existing Requirements

| New UI Req           | Original Req     | Notes                               |
|----------------------|------------------|-------------------------------------|
| UI-REQ-200, 201, 202 | REQ-010          | Cytoscape.js usage confirmed        |
| UI-REQ-300, 301      | REQ-011, REQ-101 | Contrast colors specified           |
| UI-REQ-202           | REQ-012          | Arrowhead on edges                  |
| UI-REQ-201           | REQ-013          | Asset ID fits in/near node          |
| UI-REQ-200           | REQ-020          | Data from nGQL query result         |
| UI-REQ-310           | REQ-100          | 1920x1080 minimum resolution        |
| UI-REQ-400           | REQ-121, REQ-122 | Go backend, JSON API                |
| UI-REQ-401           | REQ-123          | **Revised** - multi-file acceptable |

---

**End of UI Requirements Specification**

---

## Change Log

| Version | Date       | Changes                                                  | Author          |
|---------|------------|----------------------------------------------------------|-----------------|
| 1.0     | 2026-02-16 | Initial specification based on visual reference analysis | AI + K. Smirnov |

