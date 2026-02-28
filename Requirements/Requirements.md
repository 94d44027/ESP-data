# Software Requirements Specification (SRS)
## ESP Proof Of Concept system

**Version:** 1.11  
**Date:** March 1, 2026  
**Prepared by:** Konstantin Smirnov with the kind assistance of Perplexity AI
**Project:** ESP PoC for Nebula Graph
**Document code:** SRS

---

## 1. Introduction

### 1.1 Purpose
This Software Requirements Specification (SRS) defines the functional and non-functional requirements for the ESP Platform Proof of Concept (ESP PoC), an online tool intended to calculate TTA (Time To Attack) for IT Infrastructure models.

### 1.2 Document Scope
This document describes the complete set of requirements for Version 1.0 of ESP PoC. It serves as the foundation for system design, development, testing, and acceptance criteria.

### 1.3 Relationship to Other Documents

| Document                             | Version | Relationship                                                                                                    |
|--------------------------------------|---------|-----------------------------------------------------------------------------------------------------------------|
| UI-Requirements.md  (UIR)            | v1.11   | UI-REQ-207 consumes path calculation results; UI-REQ-208/332 visualise them on the graph canvas.                |
| ESP01_NebulaGraph_Schema.md (SCHEMA) | v1.6    | Defines database schema (ESP01)                                                                                 |
| AlgoSpecs.md (ALGO)                  | v1.0    | Defines requirements to algorithms regarding attack path calculations ((REQ-029 through REQ-032 migrated there) |

### 1.4 Intended Audience
- Software developers and architects
- Other staff working with PoC

### 1.5 Product Scope
ESP PoC will provide a web-based platform enabling a user to analyse the fixed (pre-existing) set of data representing a simple IT infrastructure of a small company. The intended purpose of building the PoC is to verify that nGQL queries ran against Nebula Graph database will provide adequate performance to enable future scalability. 

**In Scope:**
- Displaying the IT Infrastructure as a graph (assets and their connections with each other)
- Selecting entry points (from the predefined list)
- Selecting targets (from the predefined list)
- Calculating the paths from entry point to a target (by sum of TTB = TTA) within IT Infrastructure
- Displaying these paths to the user over the IT infrastructure graph
- Viewing and editing mitigations applied to assets (add, modify, remove applied_to relationships)

**Out of Scope (Future Releases):**
- Displaying the network segments
- Performing calculations of TTA/TTB against the existing set of mitigations
- Loading new data in the database
- Administrative functions
- Secure access
- User account management

### 1.6 Business Objectives
- Prove that graph database (Nebula Graph) is better for attack path analysis than relational database
- Improve the existing data schema


---

## 2. Overall Description

### 2.1 Product Perspective
ESP PoC (also referred as "the system") is a standalone web application that consists of the following components:
- **CMP001:** Nebula Graph 3.8.0 Graph database (further down referred as GrDB)
- **CMP002:** Application layer, implementing business logic (further down referred as APP layer)
- **CMP003:** Presentation layer, implementing visualisation (further down referred as VIS layer)
- **CMPO04:** Optional relational database like mySQL (further down referred as RDBMS)

The system is a prototype intended for future updates.

### 2.2 User Classes and Characteristics

**End user**
- Frequency: Daily to weekly usage
- Technical expertise: architects and developers
- Primary needs: check nGQL and overall PoC performance, see how it works
- Access level: all components


### 2.3 Operating Environment
- **Client side:** Modern web browsers (Chrome 100+, Firefox 98+, Safari 15+, Edge 100+)
- **Server side:** Linux (Ubuntu Server 24 LTS)
- **Database:** Nebula Graph 3.8.0, newest versions of mySQL (if needed to store table data)
- **Network:** Minimum 1 Mbps internet connection

### 2.4 Design and Implementation Constraints
- **CNST001:** No data protection regulations, data is not confidential
- **CNST002:** 90% uptime, no built-in resilience or data protection 
- **CNST003:** Response time for queries must not exceed 5 seconds
- **CNST004:** No user concurrency is required
- **CNST005:** APP layer shall be built in Go programming language for better performance
- **VNST006:** VIS layer can be built using any framework optimised for graph visualisation

### 2.5 Proposed functional architecture

The proposed architecture is given at the picture below

![architecture](<ESP-data_architecture_v1.png> "proposed architecture")

#### 2.5.1 GrDB
- host name: nebbie.m82
- OS: Ubuntu server 24 LTS
- port: 9669
- user: root
- password: nebula
- space: ESP01

#### 2.5.3 APP Layer
- same host
- same OS
- Written in Go
- located at /opt/asset-viz/
- Compiled at developer workstation (MacOS Tahoe 26.2 on Apple M4) for Ubuntu Server 24 on Intel (VMWare machine)
- Deployed manually or by means of GoLand

#### 2.5.3 VIS Layer
- same host
- same OS
- static HTML page, or multiple pages
- uses Cytoscape
- located at /opt/asset-viz/static/

### 2.6 Proposed software module architecture

All paths are for developers workstation and have a base of `~/projects/ESP-data/`

| Path             | Module     | Module purpose                                                                  |
|------------------|------------|---------------------------------------------------------------------------------|
| cmd/asset-viz/   | main.go    | entry point, where HTTP server starts                                           |
| internal/nebula/ | client.go  | SessionPool setup, query execution                                              |
| internal/graph/  | model.go   | CyNode, CyEdge, CyGraph structs + builder                                       |
| api/             | handler.go | HTTP handlers for /api/graph and other API endpoints                            |
| static/          | index.html | Cytoscape.js front-end (entry point; see UI-Requirements.MD for full structure) |
| config/          | config.go  | Nebula host, port, user, password, space                                        |

---

## 3. Specific Requirements

### 3.1 Functional Requirements

#### 3.1.1 User Authentication and Authorization

**REQ-001:** No user authentication is required to access VIS layer

**REQ-002:** Host, port, username, password, space for connection to Nebula Graph Database must be read from OS environment variables of NEBULA_HOST, NEBULA_PORT, NEBULA_USER, NEBULA_PASS, NEBULA_SPACE respectfully

#### 3.1.2 Visualisation

**REQ-010:** Cytoscape should be used for visualisation

**REQ-011:** Contrast colours must be used for visualisation

**REQ-012:** Connection directions should be indicated by an arrowhead

**REQ-013:** Asset identification must be visible on or near the node. The node label SHALL display Asset_Name when available, with Asset_ID as fallback. Asset_ID is always accessible via the inspector panel.

#### 3.1.3 Data Retrieval and API

**REQ-020:** The graph connectivity data, including asset properties and asset types, SHALL be retrieved from the database using the following query. This query serves the `/api/graph` endpoint and provides the data needed for graph visualisation (node colouring, labels) and the sidebar entity list (type badges, filtering). Justification for using MATCH syntax: OPTIONAL MATCH with multi-hop property retrieval is significantly cleaner than chained GO statements; this is an acceptable use per REQ-244.

```
MATCH (a:Asset)-[e:connects_to]->(b:Asset)
OPTIONAL MATCH (a)-[:has_type]->(at:Asset_Type)
OPTIONAL MATCH (b)-[:has_type]->(bt:Asset_Type)
RETURN
  a.Asset.Asset_ID AS src_asset_id,
  a.Asset.Asset_Name AS src_asset_name,
  a.Asset.is_entrance AS src_is_entrance,
  a.Asset.is_target AS src_is_target,
  a.Asset.priority AS src_priority,
  a.Asset.has_vulnerability AS src_has_vulnerability,
  at.Asset_Type.Type_Name AS src_asset_type,
  b.Asset.Asset_ID AS dst_asset_id,
  b.Asset.Asset_Name AS dst_asset_name,
  b.Asset.is_entrance AS dst_is_entrance,
  b.Asset.is_target AS dst_is_target,
  b.Asset.priority AS dst_priority,
  b.Asset.has_vulnerability AS dst_has_vulnerability,
  bt.Asset_Type.Type_Name AS dst_asset_type
LIMIT 300;
```

> **Note:** The MATCH query returns one row per `connects_to` edge, including edges with different rank values. For a pair of assets with N connections (N ranked edges), the query produces N rows. The APP layer de-duplicates these into a single visual edge per REQ-027. The `LIMIT 300` will be uplifted at teh later versions, candidate for future change.

**REQ-021:** The APP layer SHALL provide an API endpoint (`GET /api/assets`) that returns a list of all assets with their associated type names, to populate the sidebar entity browser (UI-REQ-120) and support filtering by asset type (UI-REQ-122). The underlying query:

```
MATCH (a:Asset)
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.Asset_Name AS asset_name,
  a.Asset.is_entrance AS is_entrance,
  a.Asset.is_target AS is_target,
  a.Asset.priority AS priority,
  a.Asset.has_vulnerability AS has_vulnerability,
  t.Asset_Type.Type_Name AS asset_type;
```

Optional query parameters: `?type=Server&search=CRM` for server-side filtering.

_Note “In the current version, filtering is performed client-side from the full asset list. Server-side filtering via query parameters is deferred (till we hit the larder models). 

**REQ-022:** The APP layer SHALL provide an API endpoint (`GET /api/asset/{id}`) that returns all properties of a single asset together with its related type, network segment, and operating system, for the detail inspector panel (UI-REQ-210). The underlying query:

```nGQL
MATCH (a:Asset) WHERE a.Asset.Asset_ID == $assetId
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
OPTIONAL MATCH (a)-[:belongs_to]->(s:Network_Segment)
OPTIONAL MATCH (a)-[:runs_on]->(os:OS_Type)
RETURN
  a.Asset.Asset_ID AS asset_id,
  a.Asset.Asset_Name AS asset_name,
  a.Asset.Asset_Description AS asset_description,
  a.Asset.Asset_Note AS asset_note,
  a.Asset.is_entrance AS is_entrance,
  a.Asset.is_target AS is_target,
  a.Asset.priority AS priority,
  a.Asset.has_vulnerability AS has_vulnerability,
  a.Asset.TTB AS ttb,
  t.Asset_Type.Type_Name AS asset_type,
  s.Network_Segment.Segment_Name AS segment_name,
  os.OS_Type.OS_Name AS os_name;
```

>Note: The `$assetId` parameter SHALL be validated against the expected format (e.g. `^A\d{4,5}$`) before query execution to prevent injection.

**REQ-023:** The APP layer SHALL provide an API endpoint (`GET /api/neighbors/{id}`) that returns the immediate neighbors of a given asset with edge direction, for the inspector connections summary and neighbor list (UI-REQ-210 §3–4). The underlying nGQL query (pure nGQL per REQ-243):

``` nGQL
GO FROM $assetId OVER connects_to
YIELD dst(edge) AS neighbor_id, "outbound" AS direction
UNION
GO FROM $assetId OVER connects_to REVERSELY
YIELD src(edge) AS neighbor_id, "inbound" AS direction;
```
>Design note: The UNION query may return the same `neighbor_id` twice — once as "outbound" and once as "inbound" — when bidirectional `connects_to` edges exist between two assets. The APP layer SHALL NOT de-duplicate these entries; both rows are passed to the VIS layer, which separates them into Outbound and Inbound columns in the inspector panel (UI-REQ-210 §3).

**REQ-024:** The APP layer SHALL provide an API endpoint (`GET /api/asset-types`) that returns all distinct asset types, for populating the filter checkboxes in the sidebar (UI-REQ-122). The underlying nGQL query (pure nGQL per REQ-243):

```
LOOKUP ON Asset_Type
YIELD Asset_Type.Type_ID AS type_id,
      Asset_Type.Type_Name AS type_name;
```

**REQ-026:** The APP layer SHALL provide an API endpoint (`GET /api/edges/{sourceId}/{targetId}`) that returns all `connects_to` edge properties between two given assets, for the edge inspector panel (UI-REQ-212). Both `sourceId` and `targetId` SHALL be validated per REQ-025 before query execution. The underlying nGQL query (pure nGQL per REQ-243):

```
GO FROM "{sourceId}" OVER connects_to
WHERE dst(edge) == "{targetId}"
YIELD
  connects_to.Connection_Protocol AS connection_protocol,
  connects_to.Connection_Port     AS connection_port;
```

The response SHALL also include the source and target asset summary (Asset_Name, Asset_ID, Asset_Description) so the edge inspector can render both endpoints without a separate API call. To obtain asset properties, the APP layer SHALL issue two additional lookups using the existing `QueryAssetDetail` function (REQ-022) for both the source and the target asset. The combined response format:

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
    { "connection_protocol": "TCP", "connection_port": "443" },
    { "connection_protocol": "UDP", "connection_port": "1149" },
    { "connection_protocol": "TCP/IP", "connection_port": "8000-8080" }
  ],
  "total": 4
}
```

**REQ-027:** When building the graph response for the `/api/graph` endpoint (REQ-020), the APP layer SHALL de-duplicate edges so that at most **one edge per unique (source, target) pair** is included in the response. This ensures the VIS layer renders a single visual connection between any two assets, regardless of how many `connects_to` edges exist in the database. De-duplication SHALL be performed in the APP layer (Go code) rather than by modifying the nGQL query.

Note: a candidate for future change - a separate query that will deduplicate endpoints on teh database level. I.e. that it is not the APP layer that makes the deduplication, but the database itself, in a new API point.

**REQ-028:** When inserting `connects_to` edges into NebulaGraph, each edge between the same `(source_vid, destination_vid)` pair MUST use a unique `@rank` value (starting from 0, incrementing by 1). This ensures multiple connections between the same asset pair are stored as separate edges rather than overwriting each other. See ESP01_NebulaGraph_Schema.md, `connects_to` section, "Edge Uniqueness and Rank" for the rank assignment convention.

**REQ-029:** Path Calculation Endpoint — migrated to AlgoSpec.md, ALG-REQ-001. API contract summary retained in Appendix C below.

**REQ-030:** Entry Points List Endpoint — migrated to AlgoSpec.md, ALG-REQ-002. API contract summary retained in Appendix C below.

**REQ-031:** Targets List Endpoint — migrated to AlgoSpec.md, ALG-REQ-003. API contract summary retained in Appendix C below.

**REQ-032:** TTA Calculation Rule — migrated to AlgoSpec.md, ALG-REQ-010.

>Design note: REQ-029 through REQ-032 have been migrated to AlgoSpec.md (v1.0) as part of the requirement document refactoring. The full algorithmic specification, nGQL queries, response formats, and computation rules now live in that document. The API endpoint summary in Appendix C below is retained for quick reference.

#### 3.1.3A Mitigations API

**REQ-033:** Mitigations List Endpoint. The APP layer SHALL provide an API endpoint (`GET /api/mitigations`) that returns all MITRE mitigations stored in the database. This populates the mitigation dropdown in the Mitigations Editor (UI-REQ-254). The underlying nGQL query (pure nGQL per REQ-243):

``` nGQL
LOOKUP ON tMitreMitigation
YIELD
id(vertex) AS vid,
tMitreMitigation.Mitigation_ID AS mitigation_id,
tMitreMitigation.Mitigation_Name AS mitigation_name;
```
Response format:

```json
{
"mitigations": [
{ "mitigation_id": "M1020", "mitigation_name": "SSL/TLS Inspection" },
{ "mitigation_id": "M1027", "mitigation_name": "Password Policies" }
],
"total": 43
}
```
>Note: The number of Mitre Mitigations may change in time

**REQ-034:** Asset Mitigations Endpoint. The APP layer SHALL provide an API endpoint (`GET /api/asset/{id}/mitigations`) that returns all mitigations applied to a specific asset, including edge properties. The {`id`} parameter SHALL be validated per REQ-025. Justification for using MATCH syntax (per REQ-244): traversing `applied_to` edge from `tMitreMitigation` to `Asset` with property retrieval on both the edge and the source vertex is cleaner with MATCH than with chained GO/FETCH statements. The underlying query:

``` nGQL
MATCH (m:tMitreMitigation)-[e:applied_to]->(a:Asset)
WHERE a.Asset.Asset_ID == "{assetId}"
RETURN m.tMitreMitigation.Mitigation_ID AS mitigation_id,
m.tMitreMitigation.Mitigation_Name AS mitigation_name,
e.Maturity AS maturity,
e.Active AS active;
```

Response format:

```json
{
"asset_id": "A00014",
"mitigations": [
{
"mitigation_id": "M1020",
"mitigation_name": "SSL/TLS Inspection",
"maturity": 100,
"active": true
},
{
"mitigation_id": "M1037",
"mitigation_name": "Limit Access to Resource Over Network",
"maturity": 50,
"active": false
}
],
"total": 4
}
```

**REQ-035:** Upsert Mitigation Endpoint. The APP layer SHALL provide an API endpoint (`PUT /api/asset/{id}/mitigations`) that adds or updates a mitigation applied to a given asset by performing an `UPSERT EDGE` on the `applied_to` relationship. Both `{id}` (asset) and the `mitigation_id` in the request body SHALL be validated (REQ-025, REQ-038). The underlying nGQL statement:

```nGQL
UPSERT EDGE ON applied_to
"{mitigationId}" -> "{assetId}" @0
SET Version = "1.0", Maturity = {maturity}, Active = {active};
```
Request body:

```json
{
"mitigation_id": "M1032",
"maturity": 100,
"active": true
}
```
Response: HTTP 200 on success with `{ "status": "ok" }`. HTTP 500 on database error with `{ "error": "..." }`.

>Note: The `@0` rank is fixed. Per schema ED001, there cannot be multiple `applied_to` edges between the same pair of Mitigation and Asset. Version is hardcoded to `"1.0"` in this release — version-aware infrastructure modelling is deferred.

>Note: The `UPSERT EDGE` performance is lower than INSERT due to read-modify-write at partition level (per NebulaGraph documentation). This is acceptable for PoC given `CNST004` (no concurrency requirement). Anyhow, no huge volume of such `UPSERT` operations is expected.

**REQ-036:** Delete Mitigation Endpoint. The APP layer SHALL provide an API endpoint (`DELETE /api/asset/{id}/mitigations/{mitigationId}`) that removes an `applied_to` edge between a Mitigation and an Asset. Both ID parameters SHALL be validated (REQ-025, REQ-038). The underlying nGQL statement:

```nGQL
DELETE EDGE applied_to "{mitigationId}" -> "{assetId}" @0;
```
Response: HTTP 200 on success with `{ "status": "ok" }`. HTTP 500 on database error with `{ "error": "..." }`.

>**Caution:** If no `@rank` is specified, NebulaGraph only deletes rank `0`. Since all applied_to edges use rank `0` (per REQ-035 note), this is correct for the current design. Candidate for the future reconsideration.

**REQ-038:** Mitigation ID parameters received from API requests SHALL be validated against the format `^M\d{4}$` before being used in database queries. Invalid parameters SHALL result in an HTTP 400 response with a descriptive error message.

**REQ-039:** Maturity values received from API requests SHALL be validated as integers in the set `{25, 50, 80, 100}`. Values outside this set SHALL result in an HTTP 400 response. The schema permits 0–100 (int16), but the UI enforces fixed levels per UI-REQ-254.

>Design note: The backend validates against the fixed set `{25, 50, 80, 100}` rather than the full 0–100 range, because the Mitigations Editor is the only write path for this field. If a future bulk-import or API consumer needs the full range, this validation can be relaxed.



#### 3.1.4 Data Validation

**REQ-025:** All asset ID parameters received from API requests SHALL be validated against their expected format before being used in database queries. Invalid parameters SHALL result in an HTTP 400 response with a descriptive error message.

#### 3.1.5 User Account Management

None so far

### 3.2 External Interface Requirements

#### 3.2.1 User Interface

**REQ-100:** The user interface shall be responsive and functional on devices with minimum screen resolution of 1920x1080 pixels (desktop).

**REQ-101:** The interface shall use contrast colours.


#### 3.2.3 Software Interfaces

**REQ-121:** The APP layer shall connect to the GrDB using Vesoft's Go client libraries.

**REQ-122:** The APP layer shall publish the results intended for visualisation as JSON. All API endpoints defined in REQ-020 - REQ-036 SHALL return JSON responses. Endpoints REQ-035 and REQ-036 additionally accept JSON request bodies.

**REQ-123:** The VIS layer SHALL be implemented as described in UI-Requirements.MD (Version 1.10). Cytoscape.js is the graph rendering library. The implementation may use multiple HTML files or a single-page application architecture as needed for functionality.



#### 3.2.4 Communications Interfaces

**REQ-130:** Client communication from VIS Layer to a user can be HTTP.

**REQ-131:** API requests shall use JSON format for request and response payloads.


### 3.3 Non-Functional Requirements

#### 3.3.1 Performance Requirements

**REQ-200:** The system shall use max 2 concurrent user connections. Concurrency is not important at this stage.

**REQ-201:** Page load time for any screen shall not exceed 3 seconds on standard broadband connection (1 Mbps).

**REQ-202:** Search queries shall return results within 2 seconds for result sets up to 1,000 items.

**REQ-204:** Database queries shall be optimized with appropriate indexing to maintain sub-second response times for 95% of queries.

#### 3.3.2 Security Requirements

None so far.

#### 3.3.3 Reliability and Availability

None so far.

#### 3.3.4 Maintainability

**REQ-231:** All functions and classes shall include inline documentation describing purpose, parameters, and return values.

**REQ-232:** The system architecture shall use modular design with clear separation of concerns (MVC or similar pattern).


#### 3.3.5 Portability

**REQ-240:** All components (the system) shall be deployable on Ubuntu 24 LTS Server or newer.

**REQ-242:** Configuration settings shall be externalized in environment files, enabling deployment across development, staging, and production environments without code changes.

**REQ-243:** For the GrDB queries the APP Layer should preferably use nGQL language, rather than Cypher.

**REQ-244:** For the GrDB queries the use of Cypher (MATCH syntax) is deemed to be used only when absolutely necessary, i.e. when nGQL query becomes overly complex, or is perceived as inefficient and/or slow. Each use of MATCH syntax SHALL include a justification comment in the source code.

---

## 4. Definitions, Acronyms, and Abbreviations

| Term           | Definition                                                                                                                           |
|----------------|--------------------------------------------------------------------------------------------------------------------------------------|
| **API**        | Application Programming Interface - a set of protocols for building software applications                                            |
| **TTA**        | Time To Attack, time interval between initial access and the very beginning of actions on objective                                  |
| **TTB**        | Time To Bypass, time interval to traverse a single host                                                                              |
| **REST**       | Representational State Transfer - architectural style for web services                                                               |
| **SMTP**       | Simple Mail Transfer Protocol - protocol for sending email                                                                           |
| **SQL**        | Structured Query Language - relational database query language                                                                       |
| **nGQL**       | NebulaGraph Query Language (nGQL)                                                                                                    |
| **SRS**        | Software Requirements Specification                                                                                                  |
| **TLS**        | Transport Layer Security - cryptographic protocol for secure communications                                                          |
| **The system** | All components from section **`2.1`**                                                                                                |
| **Path ID**    | Ephemeral sequential identifier (e.g. P00001) assigned to each calculated path within a single Path Inspector session; not persisted |



---

## 5. Preliminary Schedule and Milestones

All dates are given from the beginning, W stands for a calendar week.

| Phase                | Duration  | Deliverables                                                | Target Date  |
|----------------------|-----------|-------------------------------------------------------------|--------------|
| Requirements Review  | 1 week    | Approved SRS, Use Cases                                     | W1           |
| System Design        | 1 week    | Architecture Document, Database Schema                      | W2           |
| Development Sprint 1 | 1-2 weeks | JSON-based queries outcomes of Assets are published via API | W2-W3        |
| Development Sprint 2 | 1-2 weeks | Asset Visualisation                                         | W3-W4        |
| Development Sprint 3 | 1-2 weeks | TTB/TTA is calculated                                       | W4-W5        |
| Development Sprint 4 | 1-2 weeks | Mitigation change interface is implemented                  | W5-W6        |
| Presentation design  | 1 week    | PPT presentation is prepared on what has been done          | W7           |

**Total Project Duration:** 7 weeks (less than 2 months)

---

## 6. Appendices

### Appendix A: Use Case Diagram
*[Placeholder for UML use case diagram showing interactions between Patron, Staff, Administrator, and system]*

### Appendix B: Database Schema Reference
https://github.com/94d44027/ESP-data/blob/main/Data/ESP01_NebulaGraph_Schema.md

### Appendix C: API Endpoint Summary

| Endpoint                            | Method | Implements | Purpose                                               | Response format                               |
|-------------------------------------|--------|------------|-------------------------------------------------------|-----------------------------------------------|
| `/api/graph`                        | GET    | REQ-020    | Graph nodes + edges for Cytoscape.js                  | `{ nodes, edges }`                            |
| `/api/assets`                       | GET    | REQ-021    | Asset list for sidebar entity browser                 | `{ assets, total, filtered }`                 |
| `/api/asset/{id}`                   | GET    | REQ-022    | Single asset detail for inspector panel               | `{ asset_id, ... }`                           |
| `/api/neighbors/{id}`               | GET    | REQ-023    | Immediate neighbors of an asset                       | `[ { neighbor_id, direction } ]`              |
| `/api/asset-types`                  | GET    | REQ-024    | Distinct asset types for filter UI                    | `[ { type_id, type_name } ]`                  |
| `/api/edges/{sourceId}/{targetId}`  | GET    | REQ-026    | All connections between two assets for edge inspector | `{ source, target, connections, total }`      |
| `/api/paths?from=&to=&hops=`        | GET    | ALG-REQ-001 | Path calculation with TTA metric                     | `{ paths, entry_point, target, hops, total }` |
| `/api/entry-points`                 | GET    | ALG-REQ-002 | Entry point assets for Path Inspector dropdown       | `[ { asset_id, asset_name } ]`                |
| `/api/targets`                      | GET    | ALG-REQ-003 | Target assets for Path Inspector dropdown            | `[ { asset_id, asset_name } ]`                |
| `/api/mitigations`                  | GET    | REQ-033    | All MITRE mitigations for dropdown                    | `{ mitigations, total }`                      |
| `/api/asset/{id}/mitigations`       | GET    | REQ-034    | Mitigations applied to an asset                       | `{ asset_id, mitigations, total } `           |
| `/api/asset/{id}/mitigations `      | PUT    | REQ-035    | Add/update applied mitigation (UPSERT)                | `{ status }`                                  |
| `/api/asset/{id}/mitigations/{mid}` | DELETE | REQ-036    | Remove applied mitigation                             | `{ status }`                                  |

### Appendix D: Algorithm Specification
AlgoSpec.md — Path calculation and TTA/TTB algorithm requirements (ALG-REQ-001 through ALG-REQ-033)

### Appendix E: nGQL Specification
https://docs.nebula-graph.io/3.8.0/

### Appendix F: Acceptance Criteria Summary
Each requirement shall be considered complete when:
1. Implementation matches specification exactly
2. Unit tests achieve minimum 80% code coverage
3. Performance benchmarks meet specified thresholds

---

## Document Change History

| Version | Date         | Author   | Changes                                                                                                                                                                                              |
|---------|--------------|----------|------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1.1     | Feb 12, 2026 | KSmirnov | Second release                                                                                                                                                                                       |
| 1.2     | Feb 17, 2026 | KSmirnov | REQ-123 updated                                                                                                                                                                                      |
| 1.3     | Feb 17, 2026 | KSmirnov | REQ-020 revised (asset properties + type); REQ-021–025 added (asset list, detail, neighbors, asset types, input validation); REQ-122/REQ-123 revised; Appendix C added; section 3.1.3 restructured   |
| 1.4     | Feb 20, 2026 | KSmirnov | REQ-026 added (edge detail endpoint); REQ-027 added (edge de-duplication); REQ-122 range updated; Appendix C updated                                                                                 |
| 1.5     | Feb 22, 2026 | KSmirnov | REQ-028 added (edge rank requirement); REQ-020 clarifying note on rank rows added                                                                                                                    |
| 1.6     | Feb 23, 2026 | KSmirnov | REQ-029, REQ-030, REQ-031, REQ-032 added, added Path ID definition, Move /api/paths from Future → REQ-029; add two new endpoints; VIS layer refactored under new version of UI-REQ-401               |
| 1.7     | Feb 25, 2026 | KSmirnov | REQ-033–036 added (mitigations CRUD endpoints); REQ-038–039 added (mitigation validation); §1.4 updated (mitigations moved from Out of Scope to In Scope); REQ-122 range updated; Appendix C updated |
| 1.8     | Feb 25, 2026 | KSmirnov | REQ-033–036, REQ-038–039 implemented (backend Go handlers, nGQL queries, model types); REQ-122 confirmed (all endpoints return/accept JSON); implementation note added to §1.4                       |
| 1.10    | Feb 28, 2026 | KSmirnov | REQ-022 and REQ-023 are updated for the new Asset Inspector look. Version skipped to 1.10 to keep in sync with UI-Requirements.                                                                      |
| 1.11    | Mar 1, 2026  | KSmirnov | REQ-029–032 migrated to AlgoSpec.md (ALG-REQ-001–010). Stubs retained in §3.1.3. Appendix C updated with ALG-REQ refs. Appendix D added (AlgoSpec); old D→E, E→F. §1.2 updated with companion docs. |
---


**End of Document**
