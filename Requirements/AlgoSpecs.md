# Algorithm Requirements Specification
## ESP PoC — TTA/TTB Path Calculation

**Version:** 1.0  
**Date:** March 1, 2026  
**Prepared by:** Konstantin Smirnov with the kind assistance of Perplexity AI  
**Project:** ESP PoC for Nebula Graph  
**Reference:** Derived from Requirements.md (SRS v1.10), UI-Requirements.md (v1.10), ESP01_NebulaGraph_Schema.md

---

## 1. Overview

### 1.1 Purpose

This document specifies the algorithmic requirements for path discovery and TTA (Time To Attack) calculation in the ESP PoC system. It covers how attack paths are found in the graph, how TTA/TTB values are computed, and how mitigations influence the results.

### 1.2 Document Scope

This specification covers:
- Path discovery algorithm and its nGQL/MATCH implementation
- TTA/TTB computation rules
- Supporting data endpoints (entry points, targets)
- Mitigation impact on path scoring (future sections, to be expanded)
- Edge cases and algorithmic constraints

This specification does **not** cover:
- Visual presentation of paths — see UI-Requirements.md (UI-REQ-206 through UI-REQ-209, UI-REQ-332)
- General API contracts and non-functional requirements — see Requirements.md (SRS)
- Data model definitions — see ESP01_NebulaGraph_Schema.md

### 1.3 Relationship to Other Documents

| Document                      | Version | Relationship                                                                                         |
|-------------------------------|---------|------------------------------------------------------------------------------------------------------|
| Requirements.md (SRS)         | v1.10   | Parent document. Stubs REQ-029–032 reference this spec. API summary in Appendix C links here.        |
| UI-Requirements.md            | v1.10   | UI-REQ-207 consumes path calculation results; UI-REQ-208/332 visualise them on the graph canvas.     |
| ESP01_NebulaGraph_Schema.md   | —       | Defines Asset.TTB property (TA001), connects_to edges (ED005), applied_to edges (ED001).             |

### 1.4 Requirement ID Convention

All requirements in this document use the prefix `ALG-REQ-` followed by a three-digit number. Sections use `##` for chapters and `###` for individual requirements (as headers), matching the style of UI-Requirements.md.

---

## 2. Definitions

| Term               | Definition                                                                                                                                  |
|--------------------|---------------------------------------------------------------------------------------------------------------------------------------------|
| **TTA**            | Time To Attack — the cumulative time from initial access to the beginning of actions on objective, computed as the sum of TTB along a path  |
| **TTB**            | Time To Bypass — the time interval to traverse (bypass) a single host; stored as `Asset.TTB` (int32, default 10)                            |
| **Path**           | An ordered sequence of Asset nodes connected by directed `connects_to` edges, from an entry point to a target, with no repeated nodes       |
| **Hop**            | A single `connects_to` edge traversal between two adjacent nodes in a path                                                                  |
| **Entry Point**    | An Asset where `is_entrance == true`; represents the attacker's starting position                                                           |
| **Target**         | An Asset where `is_target == true`; represents the objective the attacker aims to reach                                                     |
| **Path ID**        | Ephemeral sequential identifier (e.g. P00001) assigned to each calculated path within a single session; not persisted in the database        |
| **Mitigation**     | A MITRE ATT&CK mitigation (`tMitreMitigation`) linked to an Asset via `applied_to` edge, potentially modifying the effective TTB            |

---

## 3. Path Discovery

### ALG-REQ-001: Path Calculation Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/paths?from={entryId}&to={targetId}&hops={maxHops}`) that calculates all loop-free directed paths from the entry point asset to the target asset, following `connects_to` edges up to `maxHops` hops (default 6, valid range 2–9). For each path the response SHALL include:

- A server-generated sequential path ID (format `P` + zero-padded 5-digit number, e.g. `"P00001"`)
- The ordered host chain as a string of Asset_IDs separated by `->`
- The TTA value: sum of TTB properties of all Asset nodes in the path excluding the first (entry point) node

The response SHALL be ordered by TTA ascending. Both `from` and `to` parameters SHALL be validated per REQ-025 (SRS). The `hops` parameter SHALL be validated as an integer in range 2–9; if omitted, default to 6.

Justification for MATCH syntax (per REQ-244 in SRS): variable-length path traversal with loop detection (`ALL(n IN nodes(p) WHERE single(m IN nodes(p) WHERE m == n))`) and aggregation along path nodes has no practical nGQL/GO equivalent. The underlying query:

```
MATCH p = (a:Asset)-[e:connects_to*..{maxHops}]->(b:Asset)
WHERE a.Asset.Asset_ID == "{entryId}" AND b.Asset.Asset_ID == "{targetId}"
  AND ALL(n IN nodes(p) WHERE single(m IN nodes(p) WHERE m == n))
WITH nodes(p) as Nodes2, p as p
WITH reduce(s = "", n IN Nodes2 | s + n.Asset.Asset_ID + " -> ") as Result1, p as p
WITH Result1 as Result1, left(Result1, length(Result1)-length(" -> ")) as Result2, p as p
WITH nodes(p) as Nodes2, Result2 as Result2
UNWIND Nodes2 as r
WITH r, Result2
RETURN Result2, SUM(r.Asset.TTB) as TTA
ORDER BY TTA;
```

>Note: The path IDs are generated by the APP layer (Go code) sequentially per response — they are ephemeral and not persisted. A different run with different mitigations may produce a different ordering, so P00001 may refer to a different path. Candidate for the future change.

Response format:

```json
{
  "paths": [
    {
      "path_id": "P00001",
      "hosts": "A00013 -> A00014 -> A00012 -> A00011",
      "tta": 244
    },
    {
      "path_id": "P00002",
      "hosts": "A00013 -> A00014 -> A00007 -> A00011",
      "tta": 246
    }
  ],
  "entry_point": "A00013",
  "target": "A00011",
  "hops": 6,
  "total": 15
}
```

**Migrated from:** REQ-029 (SRS v1.10)

### ALG-REQ-002: Entry Points List Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/entry-points`) that returns all assets where `is_entrance == true`, along with their Asset_ID and Asset_Name. This populates the entry point dropdown in the Path Inspector UI (UI-REQ-207 §1). The underlying query:

```nGQL
LOOKUP ON Asset WHERE Asset.is_entrance == true
YIELD id(vertex) AS vid, Asset.Asset_ID AS asset_id, Asset.Asset_Name AS asset_name;
```

**Migrated from:** REQ-030 (SRS v1.10)

### ALG-REQ-003: Targets List Endpoint

The APP layer SHALL provide an API endpoint (`GET /api/targets`) that returns all assets where `is_target == true`, along with their Asset_ID and Asset_Name. This populates the target dropdown in the Path Inspector UI (UI-REQ-207 §2). The underlying query:

```nGQL
LOOKUP ON Asset WHERE Asset.is_target == true
YIELD id(vertex) AS vid, Asset.Asset_ID AS asset_id, Asset.Asset_Name AS asset_name;
```

>Design note on ALG-REQ-002/003: Alternatively, these could have been derived client-side from the existing `/api/assets` response (REQ-021 in SRS), which already returns `is_entrance` and `is_target` for every asset. However, dedicated endpoints are cleaner for the Path Inspector and avoid coupling to the full asset list load.

**Migrated from:** REQ-031 (SRS v1.10)

---

## 4. TTA Computation

### ALG-REQ-010: TTA Calculation Rule

TTA for a given path SHALL be computed as the sum of TTB property values of all Asset nodes in the path **excluding the first node** (the entry point). The entry point is excluded because it represents the attacker's starting position, not a host that needs to be bypassed.

If any TTB value is `NULL` for a node in the path, the default value of **10** (per schema TA001 default) SHALL be used.

**Formal definition:**

Given a path `[N₀, N₁, N₂, ..., Nₖ]` where `N₀` is the entry point:

    TTA = Σ TTB(Nᵢ) for i = 1 to k

where `TTB(Nᵢ)` = `Asset.TTB` if not NULL, else 10.

**Migrated from:** REQ-032 (SRS v1.10)

### ALG-REQ-011: Path ID Generation

Path IDs SHALL be generated by the APP layer sequentially per API response, starting from `P00001` and incrementing by 1 for each path in the result set. Path IDs are ephemeral — they are not stored in the database and have no meaning outside the current response context.

**Derived from:** ALG-REQ-001 note

---

## 5. Mitigation Impact on TTA

> **Status:** This section is a placeholder for future algorithmic requirements. The requirements below are drafts to be refined as the mitigation-aware path calculation is developed.

### ALG-REQ-020: Mitigation-Aware TTA (Placeholder)

_Reserved for: How applied mitigations (via `applied_to` edges) modify the effective TTB of an asset when computing TTA._

Key questions to address:
- Does a mitigation increase TTB (making the host harder to bypass)?
- How does Maturity (25/50/80/100) scale the mitigation effect?
- How does Active/Disabled status affect the calculation?
- Are multiple mitigations on the same asset additive, multiplicative, or capped?

### ALG-REQ-021: Mitigation Maturity Weighting (Placeholder)

_Reserved for: The formula or lookup table that maps Maturity values (25, 50, 80, 100) to their effect on TTB._

### ALG-REQ-022: Recalculation Trigger (Placeholder)

_Reserved for: When and how TTA is recalculated after mitigations are added, modified, or removed._

---

## 6. Edge Cases and Constraints

### ALG-REQ-030: No Path Exists

When no loop-free directed path exists between the selected entry point and target within the specified hop limit, the API SHALL return an empty `paths` array with `total: 0`. The APP layer SHALL NOT treat this as an error condition.

```json
{
  "paths": [],
  "entry_point": "A00013",
  "target": "A00011",
  "hops": 6,
  "total": 0
}
```

### ALG-REQ-031: Loop Prevention

The path discovery query SHALL enforce loop-free paths by ensuring no node appears more than once in any single path. This is implemented via the `ALL(n IN nodes(p) WHERE single(m IN nodes(p) WHERE m == n))` predicate in the MATCH query (ALG-REQ-001).

### ALG-REQ-032: Performance Bound

Path calculation queries SHALL complete within 5 seconds (per CNST003 in SRS). If the graph topology or hop limit produces excessive combinatorial paths, the APP layer SHOULD log a warning. No timeout-based truncation is required for v1.0.

### ALG-REQ-033: Hop Limit Validation

The `hops` parameter SHALL be validated as an integer in the range 2–9. Values outside this range SHALL result in an HTTP 400 response. If the parameter is omitted, the default value of 6 SHALL be used.

---

## 7. Future Extensions

The following capabilities are anticipated but out of scope for v1.0:

- [ ] Mitigation-aware TTA calculation (ALG-REQ-020 through ALG-REQ-022)
- [ ] Path probability scoring (likelihood-weighted TTA)
- [ ] Risk-weighted paths (incorporating asset priority)
- [ ] Multi-target analysis (single entry point, multiple targets)
- [ ] Mitigation impact simulation ("what-if" recalculation)
- [ ] Path comparison (before/after mitigation changes)
- [ ] TTB recalculation based on vulnerability presence (`has_vulnerability`)

---

## 8. Cross-Reference Matrix

### 8.1 ALG-REQ to SRS (Requirements.md)

| ALG-REQ     | Migrated From | SRS Stub                  | API Endpoint                          |
|-------------|---------------|---------------------------|---------------------------------------|
| ALG-REQ-001 | REQ-029       | REQ-029 → see AlgoSpec.md | `GET /api/paths?from=&to=&hops=`      |
| ALG-REQ-002 | REQ-030       | REQ-030 → see AlgoSpec.md | `GET /api/entry-points`               |
| ALG-REQ-003 | REQ-031       | REQ-031 → see AlgoSpec.md | `GET /api/targets`                    |
| ALG-REQ-010 | REQ-032       | REQ-032 → see AlgoSpec.md | (computation rule, not an endpoint)   |

### 8.2 ALG-REQ to UI-Requirements

| ALG-REQ     | Referenced by UI-REQ | Context                                            |
|-------------|----------------------|----------------------------------------------------|
| ALG-REQ-001 | UI-REQ-207 §4–5     | Run button triggers path calculation; results table |
| ALG-REQ-002 | UI-REQ-207 §1       | Entry point dropdown population                     |
| ALG-REQ-003 | UI-REQ-207 §2       | Target dropdown population                          |
| ALG-REQ-010 | UI-REQ-207 §5       | TTA column value in results table                   |

### 8.3 ALG-REQ to Schema

| ALG-REQ     | Schema Reference              | Context                                          |
|-------------|-------------------------------|--------------------------------------------------|
| ALG-REQ-001 | ED005 (connects_to)           | Path traversal follows connects_to edges         |
| ALG-REQ-010 | TA001 (Asset.TTB)             | TTB property used for TTA summation              |
| ALG-REQ-020 | ED001 (applied_to), TA005     | Mitigation impact via applied_to edge properties |

---

## Change Log

| Version | Date         | Author   | Changes                                                                                                          |
|---------|--------------|----------|------------------------------------------------------------------------------------------------------------------|
| 1.0     | Mar 1, 2026  | KSmirnov | Initial version. Migrated REQ-029–032 from SRS v1.10. Added structure for mitigation impact and edge cases.      |

---

**End of Document**
