# Software Requirements Specification (SRS)
## ESP Prof Of Concept system

**Version:** 1.0  
**Date:** February 11, 2026  
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
- **Server side:** Linux (Ubuntu 22.04 LTS), Apache/Nginx web server
- **Database:** Nebula Graph 3.8.0, newest versions of mySQL (if needed to store table data)
- **Network:** Minimum 1 Mbps internet connection

### 2.4 Design and Implementation Constraints
- **CNST001:** No data protection regulations, data is not confidential
- **CNST002:** 90% uptime, no built-in resilience or data protection 
- **CNST003:** Response time for queries must not exceed 5 seconds
- **CNST004:** No user concurrency is required
- **CNST005:** APP layer shall be built in Go programming language for better performance
- **VNST006:** VIS layer can be built using any framework optimised for graph visualisation

### 2.5 Proposed architecture

The proposed architecture is given at the picture below

![architecture](<ESP-data_architecture_v1.png> "propsed architecture")

#### 2.5.1 GrDB
- host name: nebbie.m82

---

## 3. Specific Requirements

### 3.1 Functional Requirements

#### 3.1.1 User Authentication and Authorization

No requirements.

#### 3.1.2 Visualisation

**REQ-010:** 

#### 3.1.3 Data analysis

**REQ-020:** 



#### 3.1.5 User Account Management

None so far



### 3.2 External Interface Requirements

#### 3.2.1 User Interface

**REQ-100:** The user interface shall be responsive and functional on devices with minimum screen resolution of 1920x1080 pixels (desktop).

**REQ-101:** The interface shall use contrast colors.

**REQ-102:** The interface shall use clear visual indicators for asset types: firewall, desktop, server, router, wifi device, IoT device.



#### 3.2.3 Software Interfaces

**REQ-120:** The APP layer shall connect to the legacy MySQL catalog database (version 5.7+) using read/write access for string temporary results (if using graph database is deemed to be inefficient).

**REQ-121:** The APP layer shall connect to the GrDB using Vesoft's Go client libraries.

**REQ-122:** The APP layer shall publish the results intended for future visualisation as JSON.

**REQ-123:** The system shall expose RESTful API endpoints for VIS layer Integration.

#### 3.2.4 Communications Interfaces

**REQ-130:** Client communication from VIS Layer to a user can be HTTP.

**REQ-131:** API requests shall use JSON format for request and response payloads.


### 3.3 Non-Functional Requirements

#### 3.3.1 Performance Requirements

**REQ-200:** The system shall use max 2 concurrent user connections. Concurrency is not important at this stage.

**REQ-201:** Page load time for any screen shall not exceed 3 seconds on standard broadband connection (10 Mbps).

**REQ-202:** Search queries shall return results within 2 seconds for result sets up to 1,000 items.

**REQ-204:** Database queries shall be optimized with appropriate indexing to maintain sub-second response times for 95% of queries.

#### 3.3.2 Security Requirements

None so far.

#### 3.3.3 Reliability and Availability

None so far.

#### 3.3.4 Maintainability

**REQ-231:** All functions and classes shall include inline documentation describing purpose, parameters, and return values.

**REQ-232:** The system (architecture shall use modular design with clear separation of concerns (MVC or similar pattern).


#### 3.3.5 Portability

**REQ-240:** All components (the system) shall be deployable on Ubuntu 22.04 LTS Server or newer.

**REQ-241:** In case of using mySQL the system shall use standard SQL queries compatible with both MySQL and PostgreSQL.

**REQ-242:** Configuration settings shall be externalized in environment files, enabling deployment across development, staging, and production environments without code changes.

**REQ-243:** For the GrDB queries the APP Layer should preferably use nGQL language, rather than Cypher

**REQ-244:** For the GrDB queries the use of Cypher is deemed to be used only when absolutely necessary, i.e. when nGQL query becomes overly complex, or is perceived as inefficient and/or slow.

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

### Appendix D: nGQL Specification
https://docs.nebula-graph.io/3.8.0/

### Appendix E: Acceptance Criteria Summary
Each requirement shall be considered complete when:
1. Implementation matches specification exactly
2. Unit tests achieve minimum 80% code coverage
3. Performance benchmarks meet specified thresholds

---

## Document Change History

| Version | Date         | Author   | Changes         |
|---------|--------------|----------|-----------------|
| 1.0     | Feb 11, 2026 | KSmirnov | Initial release |

---


**End of Document**