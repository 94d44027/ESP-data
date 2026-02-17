# Software Requirements Specification (SRS)
## ESP Proof Of Concept system

**Version:** 1.3  
**Date:** February 17, 2026  
**Prepared by:** Konstantin Smirnov  
**Project:** ESP PoC for Nebula Graph

---

## 1. Introduction

### 1.1 Purpose
This Software Requirements Specification (SRS) defines the functional and non-functional requirements for the ESP Platform Proof of Concept (ESP PoC), an online tool intended to calculate TTA (Time To Attack) for IT Infrastructure models.

### 1.2 Document Scope
This document describes the complete set of requirements for Version 1.0 of ESP PoC. It serves as the foundation for system design, development, testing, and acceptance criteria.

### 1.3 Intended Audience
- Software developers and architects
- Other staff working with PoC

### 1.4 Product Scope
ESP PoC will provide a web-based platform enabling a user to analyse the fixed (pre-existing) set of data representing a simple IT infrastructure of a small company. The intended purpose of building the PoC is to verify that nGQL queries ran against Nebula Graph database will provide adequate performance to enable future scalability. 

**In Scope:**
- Displaying the IT Infrastructure as a graph (assets and their connections with each other)
- Selecting entry points (from the predefined list)
- Selecting targets (from the predefined list)
- Calculating the paths from entry point to a target (by number of hops) withing IT Infrastructure
- Displaying these paths to the user over the IT infrastructure graph

**Out of Scope (Future Releases):**
- Displaying the network segments
- Performing calculations of TTA/TTB against the existing set of mitigations
- Changing the applied mitigations
- Loading new data in the database
- Administrative functions
- Secure access
- User account management

### 1.5 Business Objectives
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

**REQ-013:** Asset ID must fit into the node (tag)

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

**REQ-022:** The APP layer SHALL provide an API endpoint (`GET /api/asset/{id}`) that returns all properties of a single asset together with its related type and network segment, for the detail inspector panel (UI-REQ-210). The underlying query:

```
MATCH (a:Asset) WHERE a.Asset.Asset_ID == $assetId
OPTIONAL MATCH (a)-[:has_type]->(t:Asset_Type)
OPTIONAL MATCH (a)-[:belongs_to]->(s:Network_Segment)
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
  s.Network_Segment.Segment_Name AS segment_name;
```

Note: The `$assetId` parameter SHALL be validated against the expected format (e.g. `^A\d{4,5}$`) before query execution to prevent injection.

**REQ-023:** The APP layer SHALL provide an API endpoint (`GET /api/neighbors/{id}`) that returns the immediate neighbors of a given asset with edge direction, for the inspector connections summary and neighbor list (UI-REQ-210 §3–4). The underlying nGQL query (pure nGQL per REQ-243):

```
GO FROM $assetId OVER connects_to
YIELD connects_to._dst AS neighbor_id, "outbound" AS direction
UNION
GO FROM $assetId OVER connects_to REVERSELY
YIELD connects_to._dst AS neighbor_id, "inbound" AS direction;
```

**REQ-024:** The APP layer SHALL provide an API endpoint (`GET /api/asset-types`) that returns all distinct asset types, for populating the filter checkboxes in the sidebar (UI-REQ-122). The underlying nGQL query (pure nGQL per REQ-243):

```
LOOKUP ON Asset_Type
YIELD Asset_Type.Type_ID AS type_id,
      Asset_Type.Type_Name AS type_name;
```

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

**REQ-122:** The APP layer shall publish the results intended for future visualisation as JSON. All API endpoints defined in REQ-020 through REQ-024 SHALL return JSON responses.

**REQ-123:** The VIS layer SHALL be implemented as described in UI-Requirements.MD (Version 1.1). Cytoscape.js is the graph rendering library. The implementation may use multiple HTML files or a single-page application architecture as needed for functionality.

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

| Term           | Definition                                                                                          |
|----------------|-----------------------------------------------------------------------------------------------------|
| **API**        | Application Programming Interface - a set of protocols for building software applications           |
| **TTA**        | Time To Attack, time interval between initial access and the very beginning of actions on objective |
| **TTB**        | Time To Bypass, time interval to traverse a single host                                             |
| **REST**       | Representational State Transfer - architectural style for web services                              |
| **SMTP**       | Simple Mail Transfer Protocol - protocol for sending email                                          |
| **SQL**        | Structured Query Language - relational database query language                                      |
| **nGQL**       | NebulaGraph Query Language (nGQL)                                                                   |
| **SRS**        | Software Requirements Specification                                                                 |
| **TLS**        | Transport Layer Security - cryptographic protocol for secure communications                         |
| **The system** | All components from section **`2.1`**                                                               |



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

| Endpoint               | Method | Implements | Purpose                                 | Response format                  |
|------------------------|--------|------------|-----------------------------------------|----------------------------------|
| `/api/graph`           | GET    | REQ-020    | Graph nodes + edges for Cytoscape.js    | `{ nodes, edges }`               |
| `/api/assets`          | GET    | REQ-021    | Asset list for sidebar entity browser   | `{ assets, total, filtered }`    |
| `/api/asset/{id}`      | GET    | REQ-022    | Single asset detail for inspector panel | `{ asset_id, ... }`              |
| `/api/neighbors/{id}`  | GET    | REQ-023    | Immediate neighbors of an asset         | `[ { neighbor_id, direction } ]` |
| `/api/asset-types`     | GET    | REQ-024    | Distinct asset types for filter UI      | `[ { type_id, type_name } ]`     |
| `/api/paths?from=&to=` | GET    | Future     | Attack path calculation                 | `{ paths, path_data }`           |

### Appendix D: nGQL Specification
https://docs.nebula-graph.io/3.8.0/

### Appendix E: Acceptance Criteria Summary
Each requirement shall be considered complete when:
1. Implementation matches specification exactly
2. Unit tests achieve minimum 80% code coverage
3. Performance benchmarks meet specified thresholds

---

## Document Change History

| Version | Date         | Author   | Changes                                                                                                                                                                                            |
|---------|--------------|----------|----------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| 1.1     | Feb 12, 2026 | KSmirnov | Second release                                                                                                                                                                                     |
| 1.2     | Feb 17, 2026 | KSmirnov | REQ-123 updated                                                                                                                                                                                    |
| 1.3     | Feb 17, 2026 | KSmirnov | REQ-020 revised (asset properties + type); REQ-021–025 added (asset list, detail, neighbors, asset types, input validation); REQ-122/REQ-123 revised; Appendix C added; section 3.1.3 restructured |

---


**End of Document**
