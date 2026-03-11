# UI Requirements Specification (UIS)
## ESP PoC - Visual Layer

**Version:** 1.13  
**Date:** March 11, 2026  
**Prepared by:** Konstantin Smirnov with the kind assistance of Perplexity AI
**Project:** ESP PoC for Nebula Graph
**Reference:** Derived from demo UI screenshots and more
**Document code:** UIS

---

## 1. Overview

This document specifies the user interface requirements for the ESP PoC Visual Layer. The interface provides graph-based visualization of IT infrastructure assets and their relationships, enabling attack path analysis through interactive exploration. The interface is subject to ongoing changes which I hope will get in an organised way. 

### 1.1 Document Scope

This specification is referenced by **REQ-123** in the main Requirements.md (SRS v1.12) and provides detailed UI/UX requirements for the VIS layer. The implementation may span multiple HTML files or use a single-page application architecture as needed for functionality.

### 1.2 Design Philosophy

- **Dark theme** as primary colour scheme (matching reference UI)
- **High contrast** for accessibility (per REQ-011, REQ-101)
- **Graph-centric** layout with supporting panels
- **Minimal cognitive load** - focus on the visualization

### 1.3 Relationship to Other Documents

| Document                             | Version | Relationship                                                                                                  |
|--------------------------------------|---------|---------------------------------------------------------------------------------------------------------------|
| Requirements.md  (SRS)               | v1.13   | The document defining main functional requirements for PoC application                                        |
| ESP01_NebulaGraph_Schema.md (SCHEMA) | v1.9    | Defines database schema (ESP01)                                                                               |
| AlgoSpecs.md (ALGO)                  | v1.5    | Defines requirements to algorithms regarding attack path calculations (TTT/TTB computation added in v1.4–1.5) |


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
- Path Inspector button (chain-link or route icon) — opens the Path Inspector panel (UI-REQ-206)
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


### UI-REQ-112: Recalculate TTBs Button

**Location:** Top bar, right section — between the Path Inspector button (⛓) and the Refresh button.

**Appearance:**
- Icon: Calculator icon (🔄 with small "T" overlay, or `⟳` with subscript) — implementation may use an `refresh.svg` icon in `assets/icons/` sized properly
- Size: Consistent with other top bar icon buttons (20-24px icon, same padding)
- Tooltip: "Recalculate TTBs"

**Badge:**
- When `stale_count > 0` (fetched from `GET /api/system-state`, REQ-041), a small red/orange badge SHALL appear on the button showing the stale count number
- Badge style: circular, 14-16px diameter, positioned at top-right corner of the button, white text on red/orange background (`#e74c3c` or `#f39c12`)
- When `stale_count == 0`, the badge is hidden

**Behaviour:**
1. **On click:** Send `POST /api/recalculate-ttb` (REQ-040)
2. **During request:** Button shows a spinning indicator or becomes disabled to prevent double-clicks
3. **On success:** Display a brief toast notification (bottom-center, 3 seconds, auto-dismiss):
    - Text: "Recalculated TTB for {recalculated} asset(s). {unchanged} unchanged."
    - Style: Dark card with green left border (`#10b981`)
4. **On error:** Display a brief toast notification:
    - Text: "TTB recalculation failed"
    - Style: Dark card with red left border (`#e74c3c`)
    - Error details logged to `console.error`
5. **After response (success or error):** Re-fetch `GET /api/system-state` to update the badge count

**State refresh triggers:**
The VIS layer SHALL fetch `GET /api/system-state` and update the badge:
- On page load (after graph data is loaded)
- After each successful mitigation UPSERT (UI-REQ-256 step 4)
- After each successful mitigation DELETE (UI-REQ-257 step 3)
- After successful TTB recalculation (this button, step 5)

### UI-REQ-113: Stale Path Warning

**Location:** Path Inspector results table (UI-REQ-207 §5), TTA column.

**Trigger:** When the path calculation response includes a non-empty `recalculated_assets` array (per ALG-REQ-046).

**Appearance:**
- A small info icon (ℹ️ or ⚠️) appears next to the TTA value in the table header or as a note below the results table
- Tooltip on hover: "TTB was recalculated for {N} asset(s) during this path calculation: {asset_list}"

**Behaviour:**
- The warning is informational only — no user action is required
- The recalculated assets are listed in the tooltip so the user can identify which nodes were updated
- If `recalculated_assets` is empty or absent, no warning is shown



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
Eeach asset item shows `Entrance`, `Target`, `Vuln`, and `P{n}` priority badges

**Badge colours**:

| Asset Type       | Badge Colour        | Text colour |
|------------------|---------------------|-------------|
| `Workstation`    | Teal (`#4a9d9c`)    | `#ffffff`   |
| `Network Device` | Blue (`#5a7fbf`)    | `#ffffff`   |
| `Server`         | Green (`#5a9d5a`)   | `#ffffff`   |
| `Mobile Device`  | Grape (`#9437FF`)   | `#ffffff`   |
| `IoT Device`     | Orange (`#BE5014`)  | `#ffffff`   |
| `Application`    | Emerald (`#10b981`) | `#ffffff`   |
| `Database`       | Amber (`#f59e0b`)   | `#ffffff`   |



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

Note: marking "clear filter" button as deferred.

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

### UI-REQ-124: Sidebar Auto-Focus on Graph Selection:

| Aspect                       | Specification                                                                                                                                                                                                                |
|------------------------------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| Trigger                      | User clicks (selects) a node on the main Cytoscape graph canvas                                                                                                                                                              |
| Behavior — sidebar open      | The asset list SHALL smooth-scroll to the corresponding asset item and apply the "selected item" highlight style (background #2a4a5a or accent color with opacity, per UI-REQ-120)                                           |
| Behavior — sidebar collapsed | No action. The sidebar remains collapsed; no auto-expand occurs.                                                                                                                                                             |
| Highlight persistence        | The highlight remains active for as long as the node is selected on the canvas. Clicking empty canvas or pressing Escape clears both the graph selection and the sidebar highlight.                                          |
| Multi-select                 | When Cmd/Ctrl + Click adds nodes (per UI-REQ-330), the sidebar SHALL scroll to and highlight the most recently selected node. Previously highlighted items remain highlighted if their nodes are still in the selection set. |
| Scroll style                 | Smooth scroll with behavior: 'smooth' (or equivalent CSS/JS), block: 'nearest' to minimise unnecessary scrolling when the item is already visible.                                                                           |

---

## 5. Main Canvas (Graph Visualization)

### UI-REQ-200: Canvas Core Requirements

The main graph canvas SHALL:

- Occupy all available space between sidebars and top/bottom bars
- Use **Cytoscape.js** for rendering (per REQ-010)
- Have dark background: `#0a0a0a` or `#121212`
- Optional: Subtle dot grid pattern background (per reference UI "Show Background" setting)

**Graph Data Source:**
- Fetch from `/api/graph` endpoint on load (per REQ-020, which returns asset properties and types)
- Expected format: `{ nodes: [...], edges: [...] }` (Cytoscape elements format)
- Each node SHALL include: `id` (= Asset_ID), `label` (= Asset_Name or Asset_ID fallback), `asset_type`, `is_entrance`,`is_target`, `priority`, `has_vulnerability`
- Each edge SHALL include: `source`, `target` (Edge `id` is auto-generated by Cytoscape. Edge `label` is deferred to future release.)
- The `/api/graph` response contains **at most one edge per (source, target) pair** (per REQ-027). Protocol/port detail for individual connections is not included in the graph payload; it is fetched on demand via `/api/edges/{sourceId}/{targetId}` (REQ-026) when the user clicks an edge.

### UI-REQ-201: Node Visualization

**Node Appearance:**
- **Shape:** Shapes are designated like this:
  - Default (all types): Circle ( ellipse )
  - Entrance points ( is_entrance=true): Triangle 
  - Target assets ( is_target=true ): Star
  - Note: Shape selectors use Cytoscape’s truthy syntax ( ?is_entrance , ?is_target ) to correctly evaluate boolean data attributes.
- **Size:** 40-50px diameter (adjustable via settings)
  - **Color:** By Asset type:

      | Node Type        | Colour              | Label colour |
      |------------------|---------------------|--------------|
      | `Workstation`    | Teal (`#4a9d9c`)    | `#e0e0e0`    |
      | `Network Device` | Blue (`#5a7fbf`)    | `#e0e0e0`    |
      | `Server`         | Green (`#5a9d5a`)   | `#e0e0e0`    |
      | `Mobile Device`  | Grape (`#9437FF`)   | `#e0e0e0`    |
      | `IoT Device`     | Orange (`#BE5014`)  | `#e0e0e0`    |
      | `Application`    | Emerald (`#10b981`) | `#e0e0e0`    |
      | `Database`       | Amber (`#f59e0b`)   | `#e0e0e0`    |

- **Label:**
  - Display: Asset_Name (fallback to Asset_ID if name is empty)
  - Font: Sans-serif, 10-12px
  - Position: Below node (or inside if space permits)
  - **Important:** Asset ID must fit into or near node (per REQ-013)
  - Truncate with ellipsis if too long (show full in tooltip)

**Node States:**
- **Default:** As above
- **Hover:** Increase border width (2px → 4px), glow effect
- **Selected:** Bright border (`#ffffff` or accent color), drop shadow
- **Dimmed:** (when filtering) Opacity 0.3, desaturated
- **Highlighted:** (from search/path) Pulsing glow or distinct border color (orange `#ff9933`)

**Node Behavior:**
- Click: Select node, open detail panel, highlight in entity list (see UI-REQ-124)
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

**Edge Consolidation:**
- When multiple `connects_to` edges exist between the same source and target assets in the database, the graph SHALL render **a single visual edge** between those two nodes.
- The consolidated edge SHALL use the default edge appearance: color `#555555`, width 2px, with a single target arrowhead — regardless of how many underlying database edges it represents.
- De-duplication is performed server-side per REQ-027; the `/api/graph` response contains at most one edge per (source, target) pair.


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

## 5A. Path Inspector

### UI-REQ-206: Path Inspector Activation

A button SHALL be placed in the Top Bar (or Bottom Toolbar) to open the Path Inspector. Per the wireframe, this is a dedicated button (chain-link icon or similar) that opens the Path Inspector as a modal overlay or a slide-out panel on the right side of the canvas.
- Button icon: chain-link (🔗) or route icon
- Tooltip: "Path Inspector"
- The button is always visible (not context-dependent)

### UI-REQ-207: Path Inspector Panel Structure

The Path Inspector SHALL be a modal dialog or floating panel overlaying the main canvas (right-aligned per wireframe). It is stateless — closing it discards all calculated data; reopening requires a fresh calculation.

Panel title: "Path Inspector"

Layout (top to bottom):
1. Entry Point dropdown
   - Label: "Entry point:"
   - Populated from `GET /api/entry-points` (ALG-REQ-002 in AlgoSpec.md)
   - Each option displays: `Asset_Name (Asset_ID)` — e.g. "WiFi1 (A00002)"
   - Below the dropdown: description text showing the asset's full name/description
   
2. Target dropdown
   - Label: "Target:"
   - Populated from `GET /api/targets` (ALG-REQ-003 in AlgoSpec.md)
   - Same display format as entry point
   - Below the dropdown: description text

3. Hops (limit) selector
   - Label: "Hops (limit):"
   - Input type: number input or dropdown
   - Valid range: 2–9
   - Default: 6

4. Run button
   - Label: "Run"
   - Triggers the path calculation call to `GET /api/paths` (ALG-REQ-001 in AlgoSpec.md)
   - Disabled until both entry point and target are selected
   - Shows loading spinner while query executes

5. Results table
   - Columns: Path | Hosts | TTA 
   - Path column: displays the path ID (e.g. P00001)
   - Hosts column: displays the asset chain (e.g. `A00013 -> A00018 -> A00011`). Line wraps are allowed in this column. 
   - - TTA column: numeric value (hours), right-aligned, formatted as `hhh:mm:ss` (e.g. "12:27:00" for 12.45 hours). The VIS layer converts the float API response to sexagesimal display.
   - Table is scrollable vertically if many results 
   - Rows are clickable (see UI-REQ-208)
   - Results sorted by TTA ascending (matching API response order)

Visual style:
- Background: matches right panel / inspector panel (#202020 – #282828)
- Table header: bold, slightly darker background
- Table rows: alternating subtle striping for readability
- Selected row: highlighted with accent color background

### UI-REQ-208: Path Selection and Graph Highlighting

When a row in the Path Inspector results table is clicked:

1. The row becomes visually selected (highlighted background)
2. The corresponding path is highlighted on the Cytoscape graph canvas:
   - Path nodes: bold border, increased border width (e.g. 4px), accent colour (orange #ff9933 or similar per UI-REQ-202 "Path Member" state)
   - Path edges: colour changes to orange #ff9933, increased width (3–4px), optionally animated dashes
   - Non-path elements: dimmed (opacity 0.3) to make the path stand out
3. The graph viewport pans/zooms to fit the selected path
4. Clicking a different row switches the highlighting to the new path
5. Only one path may be highlighted at a time

> Note: This requirement promotes UI-REQ-332 from "Future Feature" to implemented for the path highlighting behaviour specifically when triggered from the Path Inspector.

### UI-REQ-209: Path Inspector Close Behaviour
- Close button (X) at top-right of the panel
- Closing the panel:
  - Clears all calculated data (stateless — per requirement 7 in the brief)
  - Removes any path highlighting from the graph (restores all elements to default state)
  - Does not persist entry point / target / hops selections
- Pressing Escape also closes the panel

>Note: A candidate for future change - teh path table persistence is a planned feature.

### UI-REQ-2091: TTB Calculation Parameters

**Location:** Path Inspector panel (UI-REQ-207), between the Hops selector (§3) and the Run button (§4).

The Path Inspector SHALL include an expandable **"Calculation Parameters"** section containing editable fields for the TTB calculation parameters defined in ALG-REQ-071, ALG-REQ-072, and ALG-REQ-075.

**Layout:**

The section is collapsed by default, showing only a header row:

```
▶ Calculation Parameters (defaults)
```

When expanded:

```
▼ Calculation Parameters
  Orientation Time:    [ 15    ] min
  Switchover Time:     [ 10    ] min
  Priority Tolerance:  [ 1     ] ▼
  [ Reset to Defaults ]
```

**Fields:**

| Field              | Type         | Default | Range      | Unit    | ALG-REQ     |
|--------------------|--------------|---------|------------|---------|-------------|
| Orientation Time   | Number input | 15      | 0 – 1440   | minutes | ALG-REQ-071 |
| Switchover Time    | Number input | 10      | 0 – 1440   | minutes | ALG-REQ-072 |
| Priority Tolerance | Dropdown     | 1       | 0, 1, 2, 3 | —       | ALG-REQ-075 |

**Input validation:**
- Orientation Time and Switchover Time: numeric, non-negative, max 1440 (24 hours). Invalid input is rejected with a red border and tooltip: "Value must be between 0 and 1440 minutes."
- Priority Tolerance: constrained by dropdown options (0 = "Highest only", 1 = "Top 2 levels", 2 = "Top 3 levels", 3 = "All priorities").

**Unit conversion:**
- The UI displays values in **minutes** (user-friendly) but passes them to the API in **hours** (ALG-REQ-071/072 define parameters in hours). The VIS layer performs the conversion: `hours = minutes / 60`.

**Behaviour:**
1. Parameter values persist across Path Inspector sessions within the same page load (stored in `state.js` or equivalent client-side state).
2. Changing a parameter does NOT automatically re-run the path calculation. The user must click "Run" to apply new parameters.
3. The "Reset to Defaults" button restores all three fields to their default values.
4. When the section is collapsed, the header shows "(defaults)" if all values match defaults, or "(custom)" if any value has been changed.
5. Parameter values are included in the path calculation API call.

>Design note 1: The collapsed-by-default design keeps the Path Inspector clean for users who don't need to adjust parameters. The "(defaults)" / "(custom)" indicator lets experienced users see at a glance whether non-standard parameters are in effect.

>Design note 2: The API transport mechanism for these parameters is not yet defined in SRS. For the PoC, the VIS layer MAY pass them as query parameters on the existing `GET /api/paths` endpoint (e.g., `?from=...&to=...&hops=6&orientation=0.25&switchover=0.1667&priority_tolerance=1`) or as a separate configuration endpoint. This is an implementation decision to be coordinated with SRS when the ALG-REQ-045/046 update happens.

>Design note 3: Displaying time in minutes rather than hours avoids confusing fractional values (0.25 hours vs. 15 minutes). The 0–1440 range covers 0–24 hours, matching ALG-REQ-071/072 valid ranges.


## 6. Right Panel (Detail Inspector)

### UI-REQ-210: Asset Inspector Panel Structure

**Visibility:**
- Hidden by default
- Appears when a node is selected on the canvas
- Slides in from right with animation (250ms)
- Close button (X) at top-right of the header bar

**Width:** 350-400px (fixed or resizable)

**Panel title:** "Asset Inspector"

**Overall layout principle:** The panel is divided into two vertical zones:
- **Fixed zone** (top): Header bar, Basic Information, and Security Flags — these sections do NOT scroll and remain pinned at the top of the panel at all times.
- **Scrollable zone** (bottom): Connections section — this section has its own vertical scroll (`overflow-y: auto`) and fills the remaining panel height. This ensures that an asset with many connections does not push Basic Information and Security Flags off-screen.

**Sections (Top to Bottom):**

1. **Header Bar**
    - Left: Panel title text "Asset Inspector" (bold, large)
    - Right: "Edit Mitigations" icon button — shield SVG icon (`assets/icons/shield.svg`), rendered as a 28×28px icon button. Hidden when no asset is selected. Opens the Mitigations Editor modal (UI-REQ-250). Passes Asset_ID, Asset_Name, and Asset_Description to the modal via `_inspectorDetail` cache in `inspector.js`.
    - Far right: Close button (X icon)
    - Future actions (deferred): "Set as Entry Point", "Set as Target", "Find Paths" buttons

2. **Basic Information** (compact two-column grid)
    - Data source: `GET /api/asset/{id}` endpoint (REQ-022)
    - Layout: CSS grid with two equal columns

   | Left column                  | Right column             |
   |------------------------------|--------------------------|
   | **ASSET ID** (label + value) | **NAME** (label + value) |

    - Below the first row, full-width:

   | Full width                                           |
   |------------------------------------------------------|
   | **DESCRIPTION** (label + value: `Asset_Description`) |

    - Below description, two-column row:

   | Left column       | Right column     |
   |--------------------|-----------------|
   | **TYPE** (label + value: `Asset_Type`) | **OS** (label + value: `os_name` from REQ-022) |

    - Field behaviour:
        - If `Asset_Description` is empty or null, the DESCRIPTION row SHALL still be rendered with an em-dash "—" or empty value (do not collapse the row)
        - If `os_name` is empty or null (asset has no `runs_on` relationship), the OS field SHALL display "—"
        - `Asset_Note`, `Segment_Name`, and `TTB` are available in the API response but are deferred for future rendering

3. **Security Flags** (compact 2×2 grid)
    - Data source: same `GET /api/asset/{id}` endpoint (REQ-022)
    - Section header: "SECURITY FLAGS" (bold)
    - Layout: CSS grid, two columns, two rows:

   | Left column                                                             | Right column                                                  |
   |-------------------------------------------------------------------------|---------------------------------------------------------------|
   | **PRIORITY** — badge `Priority N` (colored per priority level)          | **TARGET ASSET** — `Yes` badge (red) or `No` (muted text)     |
   | **HAS VULNERABILITY** — `Yes` badge (yellow/amber) or `No` (muted text) | **ENTRANCE POINT** — `Yes` badge (green) or `No` (muted text) |

    - Each cell: small uppercase label above the value/badge
    - Badge styles are consistent with existing sidebar badges (UI-REQ-121)

4. **Connections** (two-column, scrollable)
    - Data source: `GET /api/neighbors/{id}` endpoint (REQ-023)
    - Section header: "CONNECTIONS (N)" where N is the total neighbor count (outbound + inbound)
    - Below the header, two sub-column headers:
        - Left: "Outbound (K)" — where K is the count of outbound neighbors
        - Right: "Inbound (M)" — where M is the count of inbound neighbors
    - Each neighbor is rendered as a **compact clickable chip/button**:
        - Outbound chip: `-> {Asset_ID}` (arrow prefix indicates direction away from current asset)
        - Inbound chip: `{Asset_ID} ->` (arrow suffix indicates direction toward current asset)
    - Chip appearance:
        - Background: slightly lighter than panel background (`#2a2a2a` - `#333333`)
        - Border-radius: 4-6px
        - Padding: 6px 12px
        - Font: monospace or sans-serif, 12-13px
        - Hover: lighter background, subtle border highlight
    - Chip interaction: Click navigates to that asset (calls `selectAsset(neighborId)`)
    - The two columns are independent and do not need to align row-by-row (outbound and inbound lists may have different lengths)
    - **Scroll containment:** The connections area (sub-headers + chip lists) is wrapped in a scrollable container. Max-height is dynamically calculated as the remaining panel height after the fixed zone. Standard scrollbar on the right side.
    - Empty state: If no neighbors exist, display centered text: "No connections"

**Visual Style:**
- Background: Same as left sidebar (`#202020` - `#282828`)
- Text: Light gray (`#e0e0e0`)
- Section headers: Bold, uppercase, slightly larger font (12-13px)
- Field labels: Small uppercase text, muted color (`#808080`), 10-11px
- Field values: Regular weight, primary text color (`#e0e0e0`), 13-14px
- Dividers: Subtle horizontal lines (`#333333`) between sections (Basic Information, Security Flags, Connections)


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

### UI-REQ-212: Edge Inspector

**Trigger:** Click on an edge in the graph canvas (see UI-REQ-330, Edge Selection).

**Location:** Same right Inspector panel used for node inspection (UI-REQ-210), same position, same width (350–400px). When an edge is clicked, the Inspector panel content is replaced with the edge inspector view. When a node is subsequently clicked, it is replaced back with the node inspector view.

**Panel title:** "Edge Inspector". When a node is subsequently clicked, the panel title changes back to "Asset Inspector" (UI-REQ-210).

**Data source:** `GET /api/edges/{sourceId}/{targetId}` endpoint (REQ-026), where `sourceId` and `targetId` are taken from the clicked edge's `data.source` and `data.target` Cytoscape properties.

**Sections (top to bottom):**

1. **Source Asset Block**
    - Label "Source" (section sub-header, bold)
    - `Asset_Name` (large, bold) — e.g. "FW1"
    - `Asset_ID` in parentheses — e.g. "(A00002)"
    - `Asset_Description` — e.g. "Main DC Firewall"

2. **Target Asset Block**
    - Label "Target" (section sub-header, bold)
    - `Asset_Name` (large, bold) — e.g. "WS1"
    - `Asset_ID` in parentheses — e.g. "(A00003)"
    - `Asset_Description` — e.g. "Workstation"

3. **Connections Table**
    - Table header row: `Protocol` | `Port`
    - One data row per `connects_to` edge between the source and target
    - Values: `Connection_Protocol` and `Connection_Port` from the Nebula `connects_to` edge properties
    - Example rows: TCP | 389, TCP | 443, UDP | 1149, TCP/IP | 8000-8080
    - If no connections are returned (edge exists but has no properties): display single row with "—" placeholders

**Visual Style:**
- Consistent with node inspector (UI-REQ-210): same background (`#202020`–`#282828`), text colour (`#e0e0e0`), section dividers
- Source/Target blocks separated by a subtle horizontal divider (`#333333`)
- Connections table uses the same key-value table styling as the node inspector's Primary Attributes section
- Table header row: bold text, slightly darker background

**Interaction:**
- Clicking the `Asset_Name` in either the Source or Target block SHALL select that node in the graph and switch the Inspector to the node inspector view (as if the user clicked that node directly)
- Close button (X) at the top-right of the panel closes the Inspector (same as for node inspector)


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

## 8B. Mitigations Editor
### UI-REQ-250: Mitigations Editor Activation
Trigger: Shield icon button in the Inspector panel header (UI-REQ-210 §5), visible when any asset is selected.

Behaviour:

- Clicking the button opens the Mitigations Editor as a modal overlay (centered on canvas)
- The modal receives: asset_id, asset_name, asset_description from the current Inspector context
- The modal fetches applied mitigations from `GET /api/asset/{id}/mitigations` (REQ-034) on open
- If the Inspector is closed while the modal is open, the modal remains open (it is independent once launched)
- Only one Mitigations Editor modal can be open at a time

### UI-REQ-251: Mitigations Editor Modal Structure
Dimensions: ~850px width, auto height (min ~350px, max ~550px)

Layout (top to bottom):
1. Header bar
   - Title: `Mitigations for: {Asset_Name} {Asset_Description} Asset: {Asset_ID}`
   - Close button (X) at top-right corner
   - Closing discards any unsaved in-progress edits (no warning — PoC simplification)

2. Mitigations table (UI-REQ-252)
3. Add button row
   - Green plus (➕) icon button, right-aligned below the table
   - Adds a new editable row at the bottom of the table (UI-REQ-255)

Visual style:
- Background: Dark card (#252525) with rounded corners, consistent with Settings Modal (UI-REQ-240)
- Backdrop: Semi-transparent overlay (rgba(10, 10, 10, 0.85))
- Animation: Fade-in 200ms

Keyboard:   
- `Escape` closes the modal (when not in row edit mode)


### UI-REQ-252: Mitigations Table
Columns:

| Column          | Width            | Content                            | Editable                                  |
|-----------------|------------------|------------------------------------|-------------------------------------------|
| Mitigation      | ~120px           | Mitigation_ID (e.g. "M1020")       | Dropdown (UI-REQ-254)                     |
| Mitigation Name | flex (remaining) | Mitigation_Name                    | Read-only (auto-populated from selection) |
| Maturity        | ~100px           | Maturity level label (e.g. "High") | Dropdown (UI-REQ-254)                     |
| Active          | ~90px            | "Active" or "Disabled"             | Dropdown (UI-REQ-254)                     |

Data source: `GET /api/asset/{id}/mitigations` (REQ-034)

Scrolling:
- Table body is scrollable vertically
- Visible rows: 4–6 maximum before scrolling engages
- Scroll bar on the right side (standard convention)
- Table header remains fixed during scroll

>Empty state: If no mitigations are applied, show a single centered message row: "No mitigations applied. Click ➕ to add one."

### UI-REQ-253: Row Selection and Inline Actions
Selection:
Single-click on a table row highlights it (background `#2a4a5a` or accent colour with opacity)
- Only one row can be selected at a time
- Clicking a different row moves the selection
- Clicking outside the table (but inside the modal) deselects
Action icons (appear on selected row):
- Edit icon (pencil ✏️) — appears at the right edge of the selected row
- Delete icon (red ✗) — appears next to the edit icon
- Icons appear with a subtle fade-in (150ms)
Double-click:
- Double-clicking a row enters edit mode directly (equivalent to click + edit icon)

### UI-REQ-254: Edit Mode
Entering edit mode: Click the edit icon on a selected row, or double-click the row.

Editable fields in the selected row:
1. Mitigation (dropdown)
- Populated from `GET /api/mitigations` (REQ-033)
- Excludes mitigations already applied to this asset (i.e., already in the table), except the currently edited one
- First implementation: may show only Mitigation_ID in the dropdown; Mitigation_Name display is optional enhancement
- Selecting a mitigation auto-populates the Mitigation Name column (read-only)

2. Maturity (dropdown)
- Fixed value set:

| Value | Label  |
|-------|--------|
| 25    | Low    |
| 50    | Medium |
| 80    | High   |
| 100   | Best   |

3. Active (dropdown)
- Two options: "Active" (maps to `true`) / "Disabled" (maps to `false`)

Non-editable rows: All other rows in the table remain in read-only display while one row is being edited.

Keyboard behavior:
- `Enter` — confirms the edit (triggers save flow, UI-REQ-256)
- `Escape` — cancels the edit, reverts the row to its previous values, exits edit mode

### UI-REQ-255: Add New Mitigation
Trigger: Green plus (➕) icon button below the table.

Behaviour:
- If the table is empty, the plus button adds the first row in edit mode
- If the table has rows, the plus button appends a new empty row at the bottom and enters edit mode on it
- The new row has empty/default values: Mitigation dropdown unselected, Maturity = "Best" (100), Active = "Active" (true)
- The user MUST select a mitigation from the dropdown before saving
- If the user presses `Escape` on a new unsaved row, the row is removed entirely

### UI-REQ-256: Save Confirmation Flow
Trigger: User presses Enter in edit mode, or clicks a "Save" (✓) icon if one is displayed.

Flow:

1. Validation: Check that Mitigation is selected, Maturity is set, Active is set. If validation fails, highlight the missing field with a red border and do not proceed. 
2. Confirmation dialog: Modal dialog appears (centered, ~400px):
   - Title: "Confirm Mitigation Change"
   - Body: "Apply {MitigationName} (maturity: {Label}, {Active/Disabled}) to asset {Asset_ID}?"
   - Buttons: "Cancel" (secondary) | "Confirm" (primary)
3. On Confirm: Send `PUT /api/asset/{id}/mitigations` (REQ-035) with the mitigation data
4. Wait for server response:
   - Success (HTTP 200): Exit edit mode, update the row with confirmed values. Optional: brief green flash on the row.
   - Error (HTTP 4xx/5xx): Log the full error to the browser console (console.error). The row remains in edit mode. Optional: brief red flash on the row. (Detailed error messages deferred — PoC.)
5. On Cancel: Return to edit mode (the row remains editable, no data is lost)


### UI-REQ-257: Delete Mitigation Flow

**Trigger:** Click the red ✗ (delete) icon on a selected row.

**Flow:**
1. **Confirmation dialog:** Modal dialog appears:
    - Title: "Remove Mitigation"
    - Body: "Remove {MitigationName} from asset {Asset_ID}? This action cannot be undone."
    - Buttons: "Cancel" (secondary) | "Remove" (red/destructive primary)
2. **On Confirm:** Send `DELETE /api/asset/{id}/mitigations/{mitigationId}` (REQ-036)
3. **Wait for server response:**
    - **Success (HTTP 200):** Remove the row from the table with a fade-out animation (200ms). Update the row count.
    - **Error (HTTP 4xx/5xx):** Log to `console.error`. Row remains in the table unchanged.
4. **On Cancel:** Dialog closes, no action taken



### UI-REQ-258: Mitigations Editor Error Handling
- Loading failure (`GET /api/asset/{id}/mitigations` fails): Show message in the table area: "Failed to load mitigations. [Retry]"
- Mitigations list failure (`GET /api/mitigations` fails): Mitigation dropdown shows "Error loading mitigations" and is disabled
- Write failures (`PUT/DELETE`): Logged to `console.error`. No user-facing toast in this version (PoC simplification). Row remains in pre-action state.
- Network timeout: After 5 seconds (per CNST003), treat as error




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

### UI-REQ-330: Selection Behaviour

**Single Node Selection:**
1. Click node
2. Node highlighted (border change)
3. Right panel slides in with details
4. Entity list scrolls to and highlights corresponding item (per UI-REQ-124)

**Multi-Node Selection:**
1. `Cmd/Ctrl + Click` additional nodes
2. All selected nodes highlighted
3. Right panel shows count: "N nodes selected"
4. Can perform batch actions (future)

**Edge Selection:**
1. Click edge in the graph canvas
2. Edge highlighted (per UI-REQ-202 Selected state: colour `#ff9933`, width 3–4px)
3. VIS layer reads `source` and `target` from the clicked edge's Cytoscape data
4. VIS layer calls `GET /api/edges/{source}/{target}` (REQ-026)
5. Inspector panel slides in (or updates if already open) with the edge inspector view (UI-REQ-212)
6. If the Inspector was previously showing a node detail, it is replaced with the edge view
7. Clicking a node while the edge inspector is open replaces it with the node inspector view, and vice versa

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

### UI-REQ-332: Path Highlighting (via Path Inspector)

When an attack path is selected from the Path Inspector results table (UI-REQ-207), the graph canvas SHALL enter a path highlighting mode with the following behaviour:

1. Path visual state (implemented):
   - All edges that belong to the selected path are rendered with selected/path styling: color orange (#ff9933), width 3–4px, and (optionally) animated dashes or glow. 
   - All nodes that belong to the selected path are highlighted with a distinct border (e.g. white or orange), increased border width, and optional glow effect. 
   - All non-path nodes and edges are visually dimmed (e.g. opacity ≈0.3) but remain interactive.

2. Path steps presentation (implemented):
   - The Path Inspector table itself serves as the ordered list of path steps: each row represents a path, and within a row the Hosts column shows the ordered chain of Asset_IDs (e.g. `A00013 -> A00018 -> A00011`). 
   - Selecting a row in the table highlights the corresponding path on the graph per item 1 above.

3. Viewport behaviour (implemented):
   - When a path is selected, the graph viewport SHALL pan/zoom to fit all nodes of that path within the visible area. 
   - Selecting a different path updates the highlighted elements and refits the viewport accordingly.

4. Interaction details (partially deferred):
   - Clicking a row in the Path Inspector selects the entire path. 
   - Fine-grained “click a single hop in the host chain to zoom only to that hop” is deferred and not required for this version.

5. Clearing highlight state (implemented):
   - Closing the Path Inspector (UI-REQ-209) SHALL exit path highlighting mode, restoring all nodes and edges to their default visual state and clearing any selected path. 
   - An explicit “Clear Path” button inside the Path Inspector is optional; closing the panel is sufficient to satisfy this requirement.

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
- [x] Mitigation editing (basic CRUD via Mitigations Editor — UI-REQ-250–258)
- [ ] ~~Mitigation impact simulation~~ — partially addressed: TTT/TTB algorithms (ALG-REQ-060–080) compute TTA based on current mitigation state. Real-time "what-if" simulation (preview TTA changes before committing mitigations) remains future work.
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

### UI-REQ-401: File Structure

Given the complexity identified, the VIS layer is structured as:


```
static/
├── index.html              # HTML shell only (~80 lines)
├── css/
│   ├── main.css            # Layout, top bar, sidebar, panels
│   ├── graph.css           # Canvas background, node/edge overrides
│   ├── components.css      # Inspector, modals, path-inspector table
│   └── mitigation-editor.css  # Mitigations Editor modal styles
├── js/
│   ├── state.js            # AppState object
│   ├── api.js              # API client (fetch wrappers)
│   ├── graph.js            # initCytoscape, node/edge styling
│   ├── sidebar.js          # renderAssetList, renderAssetTypeFilters, applyFilters
│   ├── inspector.js        # selectAsset, deselectAsset, renderInspector
│   ├── edge-inspector.js   # selectEdge, renderEdgeInspector
│   ├── path-inspector.js   # entire Path Inspector feature
│   ├── ui-controls.js      # toggleSidebar, toggleInspector, updateStats
│   ├── mitigation-editor.js  # Mitigations Editor modal, table, CRUD
│   └── app.js              # initialize(), event listeners, DOMContentLoaded
├── assets/
│   └── icons/
│       └── shield.svg     # Edit Mitigations icon (UI-REQ-210 §5)
```

All files still served from `/opt/asset-viz/static/` as currently configured.

### UI-REQ-402: API Endpoints Required

The following API endpoints are defined in Requirements.md (SRS v1.12) and AlgoSpec.md (v1.1). Path-related endpoints (formerly REQ-029–031) are now specified in AlgoSpec.md. See also Appendix C of Requirements.md for the full endpoint summary.

| Endpoint                                                          | SRS Req     | Purpose                                               | Used by UI-REQ         |
|-------------------------------------------------------------------|-------------|-------------------------------------------------------|------------------------|
| `GET /api/graph`                                                  | REQ-020     | Graph nodes + edges with properties/types             | UI-REQ-200, 201, 202   |
| `GET /api/assets`                                                 | REQ-021     | Asset list for sidebar (with filtering)               | UI-REQ-120, 121, 122   |
| `GET /api/asset/{id}`                                             | REQ-022     | Single asset detail for inspector panel               | UI-REQ-210             |
| `GET /api/neighbors/{id}`                                         | REQ-023     | Immediate neighbors of an asset                       | UI-REQ-210 §3–4        |
| `GET /api/asset-types`                                            | REQ-024     | Distinct asset types for filter checkboxes            | UI-REQ-122             |
| `GET /api/edges/{sourceId}/{targetId}`                            | REQ-026     | All connections between two assets for edge inspector | UI-REQ-212             |
| `GET /api/paths?from=&to=&hops=`                                  | ALG-REQ-001 | Path calculation with TTA (AlgoSpec.md)               | UI-REQ-207             |
| `GET /api/entry-points`                                           | ALG-REQ-002 | Entry points for dropdown (AlgoSpec.md)               | UI-REQ-207 §1          |
| `GET /api/targets`                                                | ALG-REQ-003 | Targets for dropdown (AlgoSpec.md)                    | UI-REQ-207 §2          |
| `GET /api/mitigations`                                            | REQ-033     | All MITRE mitigations for editor dropdown             | UI-REQ-254             |
| `GET /api/asset/{id}/mitigations`                                 | REQ-034     | Applied mitigations for editor table                  | UI-REQ-252             |
| `PUT /api/asset/{id}/mitigations`                                 | REQ-035     | Add/update applied mitigation                         | UI-REQ-256             |
| `DELETE /api/asset/{id}/mitigations/{mid}`                        | REQ-036     | Remove applied mitigation                             | UI-REQ-257             |
| `GET /api/paths?...&orientation=&switchover=&priority_tolerance=` | ALG-REQ-001 | Path calculation with TTB params (AlgoSpec.md)        | UI-REQ-207, UI-REQ-210 |

**`/api/graph` node data format (per REQ-020):**
```json
{
  "nodes": [
    {
      "data": {
        "id": "A00001",
        "label": "CRM-SRV-01",
        "asset_type": "Server",
        "is_entrance": false,
        "is_target": true,
        "priority": 1,
        "has_vulnerability": true
      }
    }
  ],
  "edges": [
    {
      "data": {
        "source": "A00001",
        "target": "A00002"
      }
    }
  ]
}
```

**`/api/assets` response format (per REQ-021):**
```json
{
  "assets": [
    {
      "asset_id": "A00001",
      "asset_name": "CRM-SRV-01",
      "asset_type": "Server",
      "is_entrance": false,
      "is_target": true,
      "priority": 1,
      "has_vulnerability": true
    }
  ],
  "total": 63,
  "filtered": 63
}
```

**`/api/asset-types` response format (per REQ-024):

```json
{
  "asset_types": [
    { "type_id": "T001", "type_name": "Server" },
    { "type_id": "T002", "type_name": "Workstation" }
  ],
  "total": 5
}
```
Note: The endpoint does not return per-type asset counts. Filter checkbox counts are computed client-side from the `/api/assets` response data.

**`/api/edges/{sourceId}/{targetId}` response format (per REQ-026):**
```json
{
  "source": {
    "asset_id": "A00002",
    "asset_name": "FW1",
    "asset_description": "Main DC Firewall"
  },
  "target": {
    "asset_id": "A00003",
    "asset_name": "WS1",
    "asset_description": "Workstation"
  },
  "connections": [
    { "connection_protocol": "TCP", "connection_port": "389" },
    { "connection_protocol": "TCP", "connection_port": "443" }
  ],
  "total": 2
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

- **Main Requirements:** `Requirements/Requirements.md` (ESP PoC SRS v1.10)
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

## Appendix B: Mapping to Existing Requirements (cross-reference)

| New UI Req           | Original Req                        | Notes                                                        |
|----------------------|-------------------------------------|--------------------------------------------------------------|
| UI-REQ-200, 201, 202 | REQ-010                             | Cytoscape.js usage confirmed                                 |
| UI-REQ-300, 301      | REQ-011, REQ-101                    | Contrast colors specified                                    |
| UI-REQ-202           | REQ-012                             | Arrowhead on edges                                           |
| UI-REQ-201           | REQ-013                             | Asset ID fits in/near node                                   |
| UI-REQ-200           | REQ-020                             | Graph data with asset properties and types                   |
| UI-REQ-120, 121, 122 | REQ-021                             | Asset list endpoint for sidebar                              |
| UI-REQ-210           | REQ-022                             | Single asset detail for inspector panel                      |
| UI-REQ-210 §3–4      | REQ-023                             | Neighbor list and connection summary                         |
| UI-REQ-122           | REQ-024                             | Asset types list for filter checkboxes                       |
| UI-REQ-310           | REQ-100                             | 1920x1080 minimum resolution                                 |
| UI-REQ-400           | REQ-121, REQ-122                    | Go backend, JSON API                                         |
| UI-REQ-401           | REQ-123                             | Multi-file VIS layer (per SRS v1.10)                         |
| UI-REQ-212           | REQ-026                             | Edge inspector panel, edge detail endpoint                   |
| UI-REQ-202           | REQ-027                             | Edge consolidation (de-duplication)                          |
| UI-REQ-206           | ALG-REQ-001 (AlgoSpec.md)           | Inspector activation                                         |
| UI-REQ-207           | ALG-REQ-001, 002, 003 (AlgoSpec.md) | Path Inspector panel with dropdowns and results              |
| UI-REQ-208           | ALG-REQ-001 (AlgoSpec.md)           | Path selection and graph highlighting                        |
| UI-REQ-209           | ---                                 | Stateless close behaviour                                    |
| UI-REQ-250           | REQ-034                             | Editor activation, fetches applied mitigations               |
| UI-REQ-252           | REQ-034                             | Table data source                                            |
| UI-REQ-254           | REQ-033, REQ-039                    | Dropdown populated from mitigations list, maturity fixed set |
| UI-REQ-256           | REQ-035                             | Save triggers UPSERT EDGE                                    |
| UI-REQ-257           | REQ-036                             | Delete triggers DELETE EDGE                                  |
| UI-REQ-210           | ALG-REQ-071, 072, 075 (AlgoSpec.md) | TTB calculation parameter controls in Path Inspector         |

   
---

**End of UI Requirements Specification**
      
---

## Change Log

| Version | Date         | Changes                                                                                                                                                                                                                                                                               | Author          |
|---------|--------------|---------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|-----------------|
| 1.0     | feb 16, 2026 | Initial specification based on visual reference analysis                                                                                                                                                                                                                              | AI + K. Smirnov |
| 1.1     | Feb 17, 2026 | Aligned with SRS v1.3: updated API endpoint specs (UI-REQ-402) with concrete JSON formats per REQ-020–024; added data source references to UI-REQ-200, UI-REQ-210; added Segment_Name to inspector panel; updated Appendix B mapping                                                  | AI + K. Smirnov |
| 1.4     | Feb 20, 2026 | UI-REQ-202 amended (edge consolidation); UI-REQ-212 added (edge inspector); UI-REQ-200 amended (edge de-duplication note); UI-REQ-330 amended (edge selection flow); UI-REQ-402 updated (new endpoint + JSON format); Appendix B updated                                              | AI + K. Smirnov |
| 1.6     | Feb 23, 2026 | Added UI-REQ-206 - 209, Amended UI-REQ-110, Promoted from "Future" to "Partially implemented" UI-REQ-332, updated UI-REQ-402, Appendix B, UI-REQ-401 updated - new static UI files structure                                                                                          | AI + K.Smirnov  |
| 1.7     | Feb 24, 2026 | Added UI-REQ-124 (scroll to asset), enrich UI-REQ-201 and UI-REQ-330 with references to the new UI-REQ-124                                                                                                                                                                            | AI + K.Smirnov  |
| 1.8     | Feb 25, 2026 | UI-REQ-210 §5 updated (Edit Mitigations button); UI-REQ-250–258 added (Mitigations Editor modal); UI-REQ-401 updated (mitigation-editor.js); UI-REQ-402 updated (4 new endpoints); §16 updated; Appendix B updated; REQ-UI-241 deleted.                                               | AI + K.Smirnov  |
| 1.9     | Feb 26, 2026 | UI-REQ-210 §5 amended: button moved to Inspector header, emoji replaced with SVG icon (shield.svg); UI-REQ-250 trigger updated; UI-REQ-401 updated (mitigation-editor.css, shield.svg); implementation confirmed for UI-REQ-250–258. The version with the working mitigations editor. | AI + K.Smirnov  |
| 1.10    | Feb 28, 2026 | Changed UI-REQ-210 to reflect the new look of Asset Inspector, updated UI-REQ-212 (Edge inspector title - to be reviwed later).                                                                                                                                                       | AI + K. Smirnov |
| 1.11    | Mar 1, 2026  | Refactoring: REQ-029/030/031 references updated to ALG-REQ-001/002/003 (AlgoSpec.md). Updated §1.1 SRS version ref, UI-REQ-207 inline refs, UI-REQ-402 table, Appendix B mapping.                                                                                                     | AI + K. Smirnov |
| 1.12    | Mar 2, 2026  | UI-REQ-112 added (Recalculate TTBs button with stale-count badge). UI-REQ-113 added (stale path warning in Path Inspector). UI-REQ-110 amended (new button in right section). Appendix B updated.                                                                                     | AI + K. Smirnov |  
| 1.13    | Mar 11, 2026 | KSmirnov | §1.1 ALGO version updated (v1.5). UI-REQ-207 §5: TTA column format changed from integer to float (2 decimal places). UI-REQ-210 added (TTB Calculation Parameters — Orientation Time, Switchover Time, Priority Tolerance controls in Path Inspector). Future features checklist updated (mitigation impact partially addressed). Appendix B updated. |
